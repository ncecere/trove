package app

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const placeholderPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Trove</title>
<style>body{font-family:system-ui,sans-serif;max-width:40rem;margin:4rem auto;padding:0 1rem;color:#222}</style>
</head><body><h1>Trove</h1><p>The Trove API is running at <code>/api/v1</code>, but this build does not include the web interface.</p></body></html>
`

// staticHandler serves the SPA from WebFS with index.html fallback.
func (a *App) staticHandler() http.Handler {
	webFS := a.deps.WebFS
	var index []byte
	if webFS != nil {
		if b, err := fs.ReadFile(webFS, "index.html"); err == nil {
			index = b
		}
	}
	started := time.Now()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if index == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			io.WriteString(w, placeholderPage)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" && fs.ValidPath(name) {
			if f, err := webFS.Open(name); err == nil {
				fi, err := f.Stat()
				if err == nil && fi.Mode().IsRegular() {
					if rs, ok := f.(io.ReadSeeker); ok {
						defer f.Close()
						if strings.HasPrefix(name, "assets/") {
							w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
						} else {
							w.Header().Set("Cache-Control", "no-cache")
						}
						http.ServeContent(w, r, name, fi.ModTime(), rs)
						return
					}
				}
				f.Close()
			}
			// Missing hashed asset: a real 404, not the SPA shell.
			if strings.HasPrefix(name, "assets/") {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", started, bytes.NewReader(index))
	})
}
