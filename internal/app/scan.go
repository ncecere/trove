package app

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"trove/internal/media"
	"trove/internal/store"
)

// ScanStatus is the API view of a library's scan state.
type ScanStatus struct {
	State      string  `json:"state"` // idle | scanning | error
	Phase      *string `json:"phase"` // walking | indexing | metadata | null
	Done       int     `json:"done"`
	Total      int     `json:"total"`
	Error      *string `json:"error"`
	StartedAt  *string `json:"startedAt"`
	FinishedAt *string `json:"finishedAt"`
}

type libScan struct {
	running   bool
	removed   bool
	pending   bool
	cancel    context.CancelFunc
	done      chan struct{}
	state     string
	phase     string
	n, total  int
	err       string
	startedAt *time.Time
	finished  *time.Time
	attempted time.Time
}

type scanManager struct {
	a      *App
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu         sync.Mutex
	libs       map[int64]*libScan
	refreshing map[int64]bool
	stopped    bool

	scanSem chan struct{} // concurrent library walks
	metaSem chan struct{} // concurrent provider lookups (be polite)
}

const metadataLookupTimeout = 60 * time.Second

func newScanManager(a *App) *scanManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &scanManager{
		a: a, ctx: ctx, cancel: cancel,
		libs:       map[int64]*libScan{},
		refreshing: map[int64]bool{},
		scanSem:    make(chan struct{}, 2),
		metaSem:    make(chan struct{}, 2),
	}
}

func (m *scanManager) start(ctx context.Context) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.loop(ctx)
	}()
}

func (m *scanManager) stop() {
	m.mu.Lock()
	m.stopped = true
	m.mu.Unlock()
	m.cancel()
	m.wg.Wait()
}

func (m *scanManager) get(id int64) *libScan {
	ls := m.libs[id]
	if ls == nil {
		ls = &libScan{state: "idle"}
		m.libs[id] = ls
	}
	return ls
}

// trigger starts a scan of the library unless one is running. With restart,
// a running scan is cancelled and started again (path/type changed).
func (m *scanManager) trigger(id int64, restart bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return
	}
	ls := m.get(id)
	if ls.running {
		if restart {
			ls.pending = true
			ls.cancel()
		}
		return
	}
	m.launchLocked(id, ls)
}

func (m *scanManager) launchLocked(id int64, ls *libScan) {
	ctx, cancel := context.WithCancel(m.ctx)
	now := time.Now().UTC()
	ls.running, ls.pending, ls.cancel, ls.done = true, false, cancel, make(chan struct{})
	ls.state, ls.phase, ls.n, ls.total, ls.err = "scanning", "walking", 0, 0, ""
	ls.startedAt, ls.attempted = &now, now
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer cancel()
		err := m.run(ctx, id, ls)
		m.finish(id, ls, err)
	}()
}

func (m *scanManager) finish(id int64, ls *libScan, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ls.running = false
	close(ls.done)
	ls.phase, ls.n, ls.total = "", 0, 0
	switch {
	case err == nil:
		now := time.Now().UTC()
		ls.state, ls.err, ls.finished = "idle", "", &now
	case errors.Is(err, context.Canceled):
		ls.state, ls.err = "idle", ""
	default:
		ls.state, ls.err = "error", err.Error()
		m.a.log.Warn("library scan failed", "library", id, "err", err)
	}
	if ls.pending && !ls.removed && !m.stopped {
		m.launchLocked(id, ls)
	}
}

// remove cancels any scan of a library that is about to be deleted.
func (m *scanManager) remove(id int64) {
	m.mu.Lock()
	ls := m.libs[id]
	delete(m.libs, id)
	var done chan struct{}
	if ls != nil {
		ls.removed = true
		if ls.running {
			ls.cancel()
			done = ls.done
		}
	}
	m.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
	}
}

func (m *scanManager) status(l *store.Library) ScanStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := ScanStatus{State: "idle", FinishedAt: fmtTimePtr(l.LastScanAt)}
	ls := m.libs[l.ID]
	if ls == nil {
		return st
	}
	st.State = ls.state
	if ls.phase != "" {
		p := ls.phase
		st.Phase = &p
	}
	st.Done, st.Total = ls.n, ls.total
	if ls.state == "error" && ls.err != "" {
		e := ls.err
		st.Error = &e
	}
	st.StartedAt = fmtTimePtr(ls.startedAt)
	if ls.finished != nil {
		st.FinishedAt = fmtTimePtr(ls.finished)
	}
	return st
}

func (m *scanManager) setPhase(ls *libScan, phase string, total int) {
	m.mu.Lock()
	ls.phase, ls.n, ls.total = phase, 0, total
	m.mu.Unlock()
}

func (m *scanManager) setDone(ls *libScan, n int) {
	m.mu.Lock()
	ls.n = n
	m.mu.Unlock()
}

func (m *scanManager) busy(id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	ls := m.libs[id]
	return ls != nil && ls.running
}

// ---- periodic scans ----

func (m *scanManager) loop(ctx context.Context) {
	m.tick(true)
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.ctx.Done():
			return
		case <-t.C:
			m.tick(false)
		}
	}
}

func (m *scanManager) tick(startup bool) {
	libs, err := m.a.store.ListLibraries(m.ctx)
	if err != nil {
		if m.ctx.Err() == nil {
			m.a.log.Warn("periodic scan: list libraries", "err", err)
		}
		return
	}
	interval := time.Duration(m.a.currentSettings().ScanIntervalMinutes) * time.Minute
	for _, l := range libs {
		m.mu.Lock()
		ls := m.libs[l.ID]
		running := ls != nil && ls.running
		var attempted time.Time
		if ls != nil {
			attempted = ls.attempted
		}
		m.mu.Unlock()
		if running {
			continue
		}
		if startup && l.LastScanAt == nil {
			m.trigger(l.ID, false)
			continue
		}
		if interval <= 0 {
			continue
		}
		last := attempted
		if l.LastScanAt != nil && l.LastScanAt.After(last) {
			last = *l.LastScanAt
		}
		if last.IsZero() || time.Since(last) >= interval {
			m.trigger(l.ID, false)
		}
	}
}

// ---- the scan itself ----

var errNoScanner = errors.New("no media scanner is configured in this build of Trove, so libraries cannot be scanned")

func (m *scanManager) run(ctx context.Context, id int64, ls *libScan) error {
	a := m.a
	select {
	case m.scanSem <- struct{}{}:
		defer func() { <-m.scanSem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	lib, err := a.store.GetLibrary(ctx, id)
	if err != nil {
		return err
	}
	if a.deps.Scanner == nil {
		return errNoScanner
	}
	fi, err := os.Stat(lib.Path)
	if err != nil {
		return fmt.Errorf("library folder is not accessible: %v", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("library path %s is not a directory", lib.Path)
	}

	started := time.Now()
	scanned, err := safeScan(ctx, a.deps.Scanner, lib.Path, lib.Type)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("scan failed: %v", err)
	}
	if len(scanned) == 0 && lib.ItemCount > 0 {
		return fmt.Errorf("library folder appears empty; keeping the %d indexed items (is the share mounted?)", lib.ItemCount)
	}

	inputs := m.prepare(lib, scanned)
	m.setPhase(ls, "indexing", len(inputs))
	gen, err := a.store.NextScanGen(ctx, id)
	if err != nil {
		return err
	}
	firstScan := lib.LastScanAt == nil && lib.ItemCount == 0
	if err := a.store.IndexItems(ctx, id, gen, inputs, firstScan, func(n int) { m.setDone(ls, n) }); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("indexing failed: %v", err)
	}
	removed, err := a.store.PruneLibrary(ctx, id, gen)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("indexing failed: %v", err)
	}

	matched, failed := m.metadataPhase(ctx, lib, ls)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := a.store.SetLibraryScanned(ctx, id, time.Now()); err != nil {
		return err
	}
	if removed > 0 {
		a.images.cleanup(ctx, a.store)
	}
	a.log.Info("library scanned", "library", id, "name", lib.Name, "items", len(inputs), "removed", removed,
		"metadataMatched", matched, "metadataErrors", failed, "dur", time.Since(started).Round(time.Millisecond).String())
	return nil
}

func safeScan(ctx context.Context, s media.Scanner, root string, t media.LibraryType) (items []media.ScannedItem, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("scanner panic: %v", p)
		}
	}()
	return s.Scan(ctx, root, t)
}

// cleanRel validates a slash-separated relative path from the scanner.
func cleanRel(rel string) (string, bool) {
	if rel == "" || strings.ContainsRune(rel, 0) {
		return "", false
	}
	rel = strings.TrimPrefix(path.Clean(strings.ReplaceAll(rel, `\`, "/")), "./")
	if rel == "." || !filepath.IsLocal(filepath.FromSlash(rel)) {
		return "", false
	}
	return rel, true
}

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true, ".avif": true}

func defaultKind(t media.LibraryType) media.Kind {
	switch t {
	case media.LibraryMovies:
		return media.KindMovie
	case media.LibraryTV, media.LibraryAnime:
		return media.KindShow
	case media.LibraryBooks:
		return media.KindBook
	case media.LibraryAudiobooks:
		return media.KindAudiobook
	}
	return media.KindMovie
}

// prepare sanitizes scanner output and computes local artwork versions.
func (m *scanManager) prepare(lib *store.Library, items []media.ScannedItem) []store.IndexItem {
	out := make([]store.IndexItem, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		rel, ok := cleanRel(it.RelPath)
		if !ok || seen[rel] {
			m.a.log.Warn("scanner returned an invalid or duplicate item path; skipped", "library", lib.ID, "relPath", it.RelPath)
			continue
		}
		seen[rel] = true
		it.RelPath = rel
		if it.Kind == "" {
			it.Kind = defaultKind(lib.Type)
		}
		if strings.TrimSpace(it.Title) == "" {
			it.Title = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
		}
		it.IDs = maps.Clone(it.IDs)
		files := make([]media.ScannedFile, 0, len(it.Files))
		fseen := map[string]bool{}
		for _, f := range it.Files {
			frel, ok := cleanRel(f.RelPath)
			if !ok || fseen[frel] {
				continue
			}
			fseen[frel] = true
			f.RelPath = frel
			if f.Container == "" {
				f.Container = strings.TrimPrefix(strings.ToLower(path.Ext(frel)), ".")
			}
			files = append(files, f)
		}
		it.Files = files
		in := store.IndexItem{Item: it}
		in.Item.PosterRelPath, in.LocalPosterV = m.localArt(lib.Path, it.PosterRelPath)
		in.Item.BackdropRelPath, in.LocalBackdropV = m.localArt(lib.Path, it.BackdropRelPath)
		out = append(out, in)
	}
	return out
}

func (m *scanManager) localArt(root, rel string) (string, string) {
	if rel == "" {
		return "", ""
	}
	rel, ok := cleanRel(rel)
	if !ok || !imageExts[strings.ToLower(path.Ext(rel))] {
		return "", ""
	}
	p, err := resolveInRoot(root, rel)
	if err != nil {
		return "", ""
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return "", ""
	}
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%d|%d", rel, fi.Size(), fi.ModTime().UnixNano())
	return rel, fmt.Sprintf("l%x", h.Sum64())
}

// ---- metadata ----

func (m *scanManager) metadataPhase(ctx context.Context, lib *store.Library, ls *libScan) (matched, failed int) {
	provs := m.a.metadataProviders()
	if len(provs) == 0 {
		return 0, 0
	}
	cands, err := m.a.store.MetadataCandidates(ctx, lib.ID)
	if err != nil {
		if ctx.Err() == nil {
			m.a.log.Warn("metadata candidates", "library", lib.ID, "err", err)
		}
		return 0, 0
	}
	if len(cands) == 0 {
		return 0, 0
	}
	m.setPhase(ls, "metadata", len(cands))
	for i := range cands {
		if ctx.Err() != nil {
			return
		}
		ok, err := m.lookup(ctx, &cands[i], provs)
		switch {
		case err != nil:
			failed++
		case ok:
			matched++
		}
		m.setDone(ls, i+1)
	}
	return
}

// lookup queries the first provider that supports the item's kind and
// stores the result. Returns whether metadata was stored.
func (m *scanManager) lookup(ctx context.Context, c *store.MetaCandidate, provs []media.MetadataProvider) (bool, error) {
	var p media.MetadataProvider
	for _, cand := range provs {
		if cand != nil && cand.Supports(c.Kind) {
			p = cand
			break
		}
	}
	if p == nil {
		return false, nil
	}
	select {
	case m.metaSem <- struct{}{}:
		defer func() { <-m.metaSem }()
	case <-ctx.Done():
		return false, ctx.Err()
	}
	lctx, cancel := context.WithTimeout(ctx, metadataLookupTimeout)
	defer cancel()
	q := media.MetadataQuery{
		Kind: c.Kind, LibraryType: c.LibraryType, Title: c.Scanned.Title, Year: c.Scanned.Year,
		Author: c.Scanned.Author, IDs: maps.Clone(c.Scanned.IDs), Seasons: c.Seasons,
	}
	if q.IDs == nil {
		q.IDs = map[string]string{}
	}
	md, err := safeLookup(lctx, p, q)
	if errors.Is(err, media.ErrNotFound) || (err == nil && md == nil) {
		return false, m.a.store.SetMetadataNotFound(ctx, c.ID)
	}
	if err != nil {
		if ctx.Err() == nil {
			m.a.log.Warn("metadata lookup failed", "item", c.ID, "title", c.Scanned.Title, "provider", p.Name(), "err", err)
		}
		return false, err
	}
	if md.Provider == "" {
		md.Provider = p.Name()
	}
	if err := m.a.store.SetRemoteMetadata(ctx, c.ID, md); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			m.a.log.Warn("store metadata", "item", c.ID, "err", err)
		}
		return false, err
	}
	return true, nil
}

func safeLookup(ctx context.Context, p media.MetadataProvider, q media.MetadataQuery) (md *media.Metadata, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("metadata provider panic: %v", r)
		}
	}()
	return p.Lookup(ctx, q)
}

// refresh re-fetches remote metadata for one item in the background.
func (m *scanManager) refresh(itemID int64) {
	m.mu.Lock()
	if m.stopped || m.refreshing[itemID] {
		m.mu.Unlock()
		return
	}
	m.refreshing[itemID] = true
	m.mu.Unlock()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			m.mu.Lock()
			delete(m.refreshing, itemID)
			m.mu.Unlock()
		}()
		provs := m.a.metadataProviders()
		if len(provs) == 0 {
			return
		}
		c, err := m.a.store.MetadataCandidate(m.ctx, itemID)
		if err != nil {
			return
		}
		_, _ = m.lookup(m.ctx, c, provs)
	}()
}
