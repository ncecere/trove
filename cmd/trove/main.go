// Command trove runs the Trove media library server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"trove/internal/app"
	"trove/internal/media"
	"trove/internal/metadata"
	"trove/internal/scanner"
	"trove/web"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	var cfg app.Config
	flag.StringVar(&cfg.Addr, "addr", env("TROVE_ADDR", ":8080"), "listen address (env TROVE_ADDR)")
	flag.StringVar(&cfg.DataDir, "data", env("TROVE_DATA", "./data"), "data directory (env TROVE_DATA)")
	flag.StringVar(&cfg.AdminUser, "admin-user", os.Getenv("TROVE_ADMIN_USER"), "bootstrap admin username, used only when no users exist (env TROVE_ADMIN_USER)")
	debug := flag.Bool("debug", os.Getenv("TROVE_DEBUG") != "", "debug logging (env TROVE_DEBUG)")
	flag.Parse()
	cfg.AdminPassword = os.Getenv("TROVE_ADMIN_PASSWORD") // env only: keep it out of `ps`

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	cfg.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	deps := app.Deps{
		Scanner: scanner.New(scanner.WithErrorHandler(func(path string, err error) {
			cfg.Logger.Warn("scan: skipped unreadable path", "path", path, "err", err)
		})),
		Providers: func(s app.Settings) []media.MetadataProvider {
			var ps []media.MetadataProvider
			if s.TMDBAPIKey != "" {
				ps = append(ps, metadata.NewTMDB(s.TMDBAPIKey))
			}
			if s.OpenLibraryEnabled {
				ps = append(ps, metadata.NewOpenLibrary())
			}
			return ps
		},
		WebFS: web.FS(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, cfg, deps); err != nil {
		fmt.Fprintln(os.Stderr, "trove:", err)
		os.Exit(1)
	}
}
