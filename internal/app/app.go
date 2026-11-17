// Package app is the Trove HTTP server: auth, libraries, browsing, artwork,
// downloads and the background scan manager.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"trove/internal/media"
	"trove/internal/store"
)

// Settings are the runtime settings editable by admins. They are handed to
// Deps.Providers so metadata providers can be (re)built when they change.
type Settings struct {
	TMDBAPIKey          string
	OpenLibraryEnabled  bool
	ScanIntervalMinutes int
}

// Deps are the pluggable parts of the application.
type Deps struct {
	// Scanner walks library folders. nil = scans fail with a clear error.
	Scanner media.Scanner
	// Providers builds the metadata providers for the current settings, in
	// priority order (the first provider whose Supports(kind) is true is
	// used). It is called again whenever the settings change; the result is
	// cached otherwise. nil = no remote metadata.
	Providers func(s Settings) []media.MetadataProvider
	// WebFS is the built single-page app (index.html at its root, hashed
	// assets under assets/). nil = a small placeholder page is served.
	WebFS fs.FS
	// HTTPClient is used to download remote artwork. nil = a default client
	// with a 20s timeout.
	HTTPClient *http.Client
}

// Config is the static configuration.
type Config struct {
	Addr    string // listen address, e.g. ":8080"
	DataDir string // holds trove.db and images/
	// Optional bootstrap admin, created on startup when no user exists.
	AdminUser     string
	AdminPassword string
	// Logger; nil = slog.Default().
	Logger *slog.Logger
}

// App is a configured Trove instance.
type App struct {
	cfg     Config
	deps    Deps
	log     *slog.Logger
	store   *store.Store
	scans   *scanManager
	limiter *loginLimiter
	images  *imageCache
	dedupe  *activityDedupe
	handler http.Handler

	settingsMu sync.Mutex
	settings   Settings
	provKey    *Settings
	providers  []media.MetadataProvider

	dummyHash []byte
}

var bcryptCost = bcrypt.DefaultCost

const (
	settingTMDBKey      = "tmdb_api_key"
	settingOpenLibrary  = "openlibrary_enabled"
	settingScanInterval = "scan_interval_minutes"
)

// New opens the data dir and builds the application. Call Start to launch
// background work and Close when done.
func New(cfg Config, deps Deps) (*App, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	dataDir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	cfg.DataDir = dataDir
	if err := os.MkdirAll(filepath.Join(dataDir, "images"), 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	st, err := store.Open(filepath.Join(dataDir, "trove.db"))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	a := &App{
		cfg:     cfg,
		deps:    deps,
		log:     log,
		store:   st,
		limiter: newLoginLimiter(),
		dedupe:  newActivityDedupe(30 * time.Minute),
	}
	client := deps.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	a.images = newImageCache(filepath.Join(dataDir, "images"), client, log)
	a.dummyHash, _ = bcrypt.GenerateFromPassword([]byte("trove-timing-equalizer"), bcryptCost)

	if err := a.loadSettings(context.Background()); err != nil {
		st.Close()
		return nil, err
	}
	if err := a.bootstrapAdmin(context.Background()); err != nil {
		st.Close()
		return nil, err
	}
	a.scans = newScanManager(a)
	a.handler = a.routes()
	return a, nil
}

// Handler returns the root HTTP handler.
func (a *App) Handler() http.Handler { return a.handler }

// Start launches background work (periodic scans, cleanup) until ctx ends.
func (a *App) Start(ctx context.Context) {
	a.scans.start(ctx)
	go a.housekeeping(ctx)
}

// Close stops scans and closes the database.
func (a *App) Close() error {
	a.scans.stop()
	return a.store.Close()
}

// Run starts the server and blocks until ctx is cancelled, then shuts down
// gracefully.
func Run(ctx context.Context, cfg Config, deps Deps) error {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	a, err := New(cfg, deps)
	if err != nil {
		return err
	}
	defer a.Close()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(a.log.Handler(), slog.LevelWarn),
		// No WriteTimeout: downloads can legitimately take hours.
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.Start(runCtx)

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	a.log.Info("trove listening", "addr", ln.Addr().String(), "data", a.cfg.DataDir)

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	a.log.Info("shutting down")
	cancel()
	shCtx, shCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shCancel()
	if err := srv.Shutdown(shCtx); err != nil {
		a.log.Warn("graceful shutdown timed out; closing connections", "err", err)
		srv.Close()
	}
	return nil
}

func (a *App) bootstrapAdmin(ctx context.Context) error {
	if a.cfg.AdminUser == "" || a.cfg.AdminPassword == "" {
		return nil
	}
	n, err := a.store.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	if msg := validateUsername(a.cfg.AdminUser); msg != "" {
		return fmt.Errorf("bootstrap admin: %s", msg)
	}
	if msg := validatePassword(a.cfg.AdminPassword); msg != "" {
		return fmt.Errorf("bootstrap admin: %s", msg)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(a.cfg.AdminPassword), bcryptCost)
	if err != nil {
		return err
	}
	_, err = a.store.CreateUser(ctx, store.NewUser{Username: a.cfg.AdminUser, PasswordHash: string(hash), Role: "admin", CanDownload: true}, true)
	if errors.Is(err, store.ErrConflict) {
		return nil
	}
	if err == nil {
		a.log.Info("created bootstrap admin", "username", a.cfg.AdminUser)
	}
	return err
}

func (a *App) loadSettings(ctx context.Context) error {
	kv, err := a.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	s := Settings{OpenLibraryEnabled: true, ScanIntervalMinutes: 60}
	if v, ok := kv[settingTMDBKey]; ok {
		s.TMDBAPIKey = v
	}
	if v, ok := kv[settingOpenLibrary]; ok {
		s.OpenLibraryEnabled = v == "1"
	}
	if v, ok := kv[settingScanInterval]; ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			s.ScanIntervalMinutes = n
		}
	}
	a.settingsMu.Lock()
	a.settings = s
	a.settingsMu.Unlock()
	return nil
}

func (a *App) currentSettings() Settings {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.settings
}

// metadataProviders returns the providers for the current settings,
// rebuilding them only when the settings changed.
func (a *App) metadataProviders() []media.MetadataProvider {
	if a.deps.Providers == nil {
		return nil
	}
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	key := a.settings
	key.ScanIntervalMinutes = 0 // irrelevant for providers
	if a.provKey == nil || *a.provKey != key {
		a.providers = a.deps.Providers(a.settings)
		a.provKey = &key
	}
	return a.providers
}

func (a *App) housekeeping(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		now := time.Now()
		if err := a.store.DeleteExpiredSessions(ctx, now); err != nil && ctx.Err() == nil {
			a.log.Warn("purge sessions", "err", err)
		}
		if err := a.store.PruneActivity(ctx, now.AddDate(0, 0, -365)); err != nil && ctx.Err() == nil {
			a.log.Warn("prune activity", "err", err)
		}
		a.limiter.gc(now)
		a.dedupe.gc(now)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func validateUsername(u string) string {
	if len(u) < 3 || len(u) > 32 {
		return "Username must be 3-32 characters"
	}
	for _, r := range u {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return "Username may only contain letters, digits, '.', '_' and '-'"
		}
	}
	return ""
}

func validatePassword(p string) string {
	if len([]rune(p)) < 8 {
		return "Password must be at least 8 characters"
	}
	if len(p) > 72 {
		return "Password must be at most 72 bytes"
	}
	return ""
}

func trimmed(s string) string { return strings.TrimSpace(s) }
