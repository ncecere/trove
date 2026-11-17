package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"trove/internal/store"
)

const maxImageBytes = 15 << 20

func (a *App) handleArtwork(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r)
		if !ok {
			notFound(w)
			return
		}
		item, lib, ok := a.loadAccessibleItem(w, r, id)
		if !ok {
			return
		}
		localRel, remoteURL := item.Scanned.PosterRelPath, ""
		if kind == "backdrop" {
			localRel = item.Scanned.BackdropRelPath
		}
		if item.Remote != nil {
			remoteURL = item.Remote.PosterURL
			if kind == "backdrop" {
				remoteURL = item.Remote.BackdropURL
			}
		}
		cache := "private, max-age=86400"
		if r.URL.Query().Get("v") != "" {
			cache = "private, max-age=31536000, immutable"
		}

		if localRel != "" && imageExts[strings.ToLower(path.Ext(localRel))] {
			if fh, fi, err := openInRoot(lib.Path, localRel); err == nil {
				defer fh.Close()
				if fi.Size() <= 4*maxImageBytes {
					w.Header().Set("Cache-Control", cache)
					w.Header().Set("Content-Type", contentTypeFor(localRel))
					http.ServeContent(w, r, "", fi.ModTime(), fh)
					return
				}
			}
		}
		if remoteURL != "" {
			p, err := a.images.get(r.Context(), item.ID, kind, remoteURL)
			if err == nil {
				fh, err := os.Open(p)
				if err == nil {
					defer fh.Close()
					fi, _ := fh.Stat()
					w.Header().Set("Cache-Control", cache)
					var head [512]byte
					n, _ := io.ReadFull(fh, head[:])
					w.Header().Set("Content-Type", http.DetectContentType(head[:n]))
					if _, err := fh.Seek(0, io.SeekStart); err == nil {
						http.ServeContent(w, r, "", fi.ModTime(), fh)
						return
					}
				}
			}
		}
		writeError(w, http.StatusNotFound, "not_found", "No artwork")
	}
}

// imageCache downloads remote artwork once into <DataDir>/images.
type imageCache struct {
	dir    string
	client *http.Client
	log    *slog.Logger
	sem    chan struct{}

	mu       sync.Mutex
	inflight map[string]*imageCall
	failed   map[string]time.Time
}

type imageCall struct {
	done chan struct{}
	err  error
}

func newImageCache(dir string, client *http.Client, log *slog.Logger) *imageCache {
	return &imageCache{
		dir: dir, client: client, log: log, sem: make(chan struct{}, 4),
		inflight: map[string]*imageCall{}, failed: map[string]time.Time{},
	}
}

func (c *imageCache) fileName(itemID int64, kind, u string) string {
	sum := sha256.Sum256([]byte(u))
	return fmt.Sprintf("%d-%s-%s", itemID, kind, hex.EncodeToString(sum[:12]))
}

var errNoImage = errors.New("image unavailable")

// get returns the path of the cached image, downloading it if needed.
func (c *imageCache) get(ctx context.Context, itemID int64, kind, u string) (string, error) {
	name := c.fileName(itemID, kind, u)
	p := filepath.Join(c.dir, name)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	c.mu.Lock()
	if t, ok := c.failed[name]; ok && time.Since(t) < 15*time.Minute {
		c.mu.Unlock()
		return "", errNoImage
	}
	call, ok := c.inflight[name]
	if !ok {
		call = &imageCall{done: make(chan struct{})}
		c.inflight[name] = call
		go func() {
			call.err = c.download(u, p)
			c.mu.Lock()
			delete(c.inflight, name)
			if call.err != nil {
				c.failed[name] = time.Now()
				if len(c.failed) > 10000 {
					c.failed = map[string]time.Time{}
				}
			}
			c.mu.Unlock()
			if call.err == nil {
				c.removeOthers(itemID, kind, name)
			} else {
				c.log.Warn("artwork download failed", "item", itemID, "url", u, "err", call.err)
			}
			close(call.done)
		}()
	}
	c.mu.Unlock()
	select {
	case <-call.done:
		if call.err != nil {
			return "", errNoImage
		}
		return p, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (c *imageCache) download(u, dst string) error {
	pu, err := url.Parse(u)
	if err != nil || (pu.Scheme != "http" && pu.Scheme != "https") || pu.Host == "" {
		return fmt.Errorf("unsupported artwork URL")
	}
	c.sem <- struct{}{}
	defer func() { <-c.sem }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pu.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Trove/1.0")
	req.Header.Set("Accept", "image/*")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxImageBytes {
		return fmt.Errorf("image too large")
	}
	tmp, err := os.CreateTemp(c.dir, ".dl-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		tmp.Close()
		return err
	}
	if n > maxImageBytes {
		tmp.Close()
		return fmt.Errorf("image too large")
	}
	var head [512]byte
	m, _ := tmp.ReadAt(head[:], 0)
	ct := http.DetectContentType(head[:m])
	if !strings.HasPrefix(ct, "image/") || strings.Contains(ct, "svg") {
		tmp.Close()
		return fmt.Errorf("not an image (%s)", ct)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

func (c *imageCache) removeOthers(itemID int64, kind, keep string) {
	matches, _ := filepath.Glob(filepath.Join(c.dir, fmt.Sprintf("%d-%s-*", itemID, kind)))
	for _, m := range matches {
		if filepath.Base(m) != keep {
			os.Remove(m)
		}
	}
}

// cleanup removes cached images of items that no longer exist.
func (c *imageCache) cleanup(ctx context.Context, st *store.Store) {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	ids, err := st.ItemIDs(ctx)
	if err != nil {
		return
	}
	for _, e := range entries {
		idStr, _, ok := strings.Cut(e.Name(), "-")
		if !ok {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		if !ids[id] {
			os.Remove(filepath.Join(c.dir, e.Name()))
		}
	}
}
