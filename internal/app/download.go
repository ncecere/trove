package app

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"trove/internal/store"
)

var errOutsideRoot = errors.New("path escapes the library root")

// resolveInRoot maps a stored slash-separated relative path to an absolute
// path, verifying (after cleaning and resolving symlinks) that it is still
// inside the library root.
func resolveInRoot(root, rel string) (string, error) {
	rel, ok := cleanRel(rel)
	if !ok {
		return "", errOutsideRoot
	}
	realRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(filepath.Join(realRoot, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	r, err := filepath.Rel(realRoot, real)
	if err != nil || !filepath.IsLocal(r) {
		return "", errOutsideRoot
	}
	return real, nil
}

// openInRoot opens a regular file inside root. It double-checks with
// os.Root, which also refuses symlink escapes at open time.
func openInRoot(root, rel string) (*os.File, os.FileInfo, error) {
	real, err := resolveInRoot(root, rel)
	if err != nil {
		return nil, nil, err
	}
	realRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return nil, nil, err
	}
	rt, err := os.OpenRoot(realRoot)
	if err != nil {
		return nil, nil, err
	}
	defer rt.Close()
	r, _ := filepath.Rel(realRoot, real)
	f, err := rt.Open(r)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, nil, fmt.Errorf("%s is not a regular file", rel)
	}
	return f, fi, nil
}

// ---- content types ----

var contentTypes = map[string]string{
	"mkv": "video/x-matroska", "mk3d": "video/x-matroska", "mp4": "video/mp4", "m4v": "video/mp4",
	"mov": "video/quicktime", "avi": "video/x-msvideo", "wmv": "video/x-ms-wmv", "webm": "video/webm",
	"ts": "video/mp2t", "m2ts": "video/mp2t", "mts": "video/mp2t", "mpg": "video/mpeg", "mpeg": "video/mpeg",
	"flv": "video/x-flv", "ogv": "video/ogg", "3gp": "video/3gpp", "vob": "video/dvd", "iso": "application/x-iso9660-image",
	"mp3": "audio/mpeg", "m4a": "audio/mp4", "m4b": "audio/mp4", "aac": "audio/aac", "flac": "audio/flac",
	"ogg": "audio/ogg", "oga": "audio/ogg", "opus": "audio/ogg", "wav": "audio/wav", "wma": "audio/x-ms-wma",
	"alac": "audio/mp4", "aiff": "audio/aiff", "aif": "audio/aiff", "mka": "audio/x-matroska",
	"epub": "application/epub+zip", "mobi": "application/x-mobipocket-ebook", "azw": "application/vnd.amazon.ebook",
	"azw3": "application/vnd.amazon.mobi8-ebook", "kfx": "application/vnd.amazon.ebook", "pdf": "application/pdf",
	"cbz": "application/vnd.comicbook+zip", "cbr": "application/vnd.comicbook-rar", "cb7": "application/x-cb7",
	"djvu": "image/vnd.djvu", "fb2": "application/x-fictionbook+xml", "lit": "application/x-ms-reader",
	"txt": "text/plain; charset=utf-8", "rtf": "application/rtf",
	"srt": "application/x-subrip", "vtt": "text/vtt; charset=utf-8", "ass": "text/x-ssa", "ssa": "text/x-ssa",
	"sub": "text/plain; charset=utf-8", "idx": "text/plain; charset=utf-8", "sup": "application/octet-stream",
	"nfo": "text/plain; charset=utf-8", "cue": "text/plain; charset=utf-8",
	"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp", "gif": "image/gif", "avif": "image/avif",
	"zip": "application/zip", "rar": "application/vnd.rar", "7z": "application/x-7z-compressed",
}

func contentTypeFor(name string) string {
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension("." + ext); ct != "" && ext != "" {
		return ct
	}
	return "application/octet-stream"
}

// inlineSafe reports whether a type may be rendered inline on our origin
// (never HTML/SVG/XML that could run script).
func inlineSafe(ct string) bool {
	base, _, _ := strings.Cut(ct, ";")
	base = strings.TrimSpace(base)
	switch {
	case strings.HasPrefix(base, "video/"), strings.HasPrefix(base, "audio/"):
		return true
	case strings.HasPrefix(base, "image/") && !strings.Contains(base, "svg"):
		return true
	case base == "application/pdf", base == "text/plain", base == "text/vtt":
		return true
	}
	return false
}

// contentDisposition builds an RFC 6266 header with an ASCII fallback and an
// RFC 5987 filename* parameter for non-ASCII names.
func contentDisposition(disp, name string) string {
	var b strings.Builder
	ascii := true
	for _, r := range name {
		if r >= 0x80 || r < 0x20 || r == 0x7f {
			ascii = false
		}
	}
	fallback := strings.Map(func(r rune) rune {
		switch {
		case r >= 0x80 || r < 0x20 || r == 0x7f:
			return '_'
		case r == '"' || r == '\\':
			return '_'
		}
		return r
	}, name)
	b.WriteString(disp)
	b.WriteString(`; filename="`)
	b.WriteString(fallback)
	b.WriteString(`"`)
	if !ascii {
		b.WriteString(`; filename*=UTF-8''`)
		b.WriteString(rfc5987Escape(name))
	}
	return b.String()
}

func rfc5987Escape(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c < 0x80 && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("!#$&+-.^_`|~", c) >= 0) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// fileHeaders isolates served media from the app origin.
func fileHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "private, no-cache")
}

// ---- activity de-duplication ----

type activityDedupe struct {
	mu     sync.Mutex
	window time.Duration
	seen   map[string]time.Time
}

func newActivityDedupe(window time.Duration) *activityDedupe {
	return &activityDedupe{window: window, seen: map[string]time.Time{}}
}

// first reports whether key was not seen within the window (and marks it).
func (d *activityDedupe) first(key string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.seen[key]; ok && now.Sub(t) < d.window {
		return false
	}
	d.seen[key] = now
	return true
}

func (d *activityDedupe) gc(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, t := range d.seen {
		if now.Sub(t) >= d.window {
			delete(d.seen, k)
		}
	}
}

func (a *App) recordDownload(r *http.Request, u *store.User, key, action string, item *store.Item, detail string, size int64) {
	if r.Method != http.MethodGet || !a.dedupe.first(fmt.Sprintf("%d|%s", u.ID, key), time.Now()) {
		return
	}
	uid, iid := u.ID, item.ID
	title := item.Title
	if err := a.store.AddActivity(r.Context(), store.Activity{
		UserID: &uid, Username: u.Username, Action: action, ItemID: &iid, ItemTitle: &title, Detail: &detail, Size: &size,
	}); err != nil {
		a.log.Warn("record download activity", "err", err)
	}
}

// ---- handlers ----

// loadAccessibleItem loads an item if the user may see it (else 404).
func (a *App) loadAccessibleItem(w http.ResponseWriter, r *http.Request, id int64) (*store.Item, *store.Library, bool) {
	it, err := a.store.GetItem(r.Context(), id)
	if err != nil {
		a.storeErr(w, r, err)
		return nil, nil, false
	}
	if !userFrom(r).CanAccess(it.LibraryID) {
		notFound(w)
		return nil, nil, false
	}
	lib, err := a.store.GetLibrary(r.Context(), it.LibraryID)
	if err != nil {
		a.storeErr(w, r, err)
		return nil, nil, false
	}
	return it, lib, true
}

func (a *App) handleFileDownload(w http.ResponseWriter, r *http.Request) { a.serveFile(w, r, false) }

func (a *App) handleFileStream(w http.ResponseWriter, r *http.Request) { a.serveFile(w, r, true) }

func (a *App) serveFile(w http.ResponseWriter, r *http.Request, inline bool) {
	u := userFrom(r)
	if !u.CanDownload {
		forbidden(w, "Downloads are disabled for this account")
		return
	}
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	f, err := a.store.GetFile(r.Context(), id)
	if err != nil {
		a.storeErr(w, r, err)
		return
	}
	item, lib, ok := a.loadAccessibleItem(w, r, f.ItemID)
	if !ok {
		return
	}
	fh, fi, err := openInRoot(lib.Path, f.RelPath)
	if err != nil {
		if errors.Is(err, errOutsideRoot) || !errors.Is(err, os.ErrNotExist) {
			a.log.Warn("refusing to serve file", "file", f.ID, "relPath", f.RelPath, "err", err)
		}
		writeError(w, http.StatusNotFound, "not_found", "File is not available on disk")
		return
	}
	defer fh.Close()

	name := path.Base(f.RelPath)
	ct := contentTypeFor(name)
	disp := "attachment"
	if inline {
		if inlineSafe(ct) {
			disp = "inline"
		} else {
			ct = "application/octet-stream"
		}
	}
	fileHeaders(w)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", contentDisposition(disp, name))
	if !inline {
		a.recordDownload(r, u, "file|"+strconv.FormatInt(f.ID, 10), "download_file", item, name, fi.Size())
	}
	http.ServeContent(w, r, name, fi.ModTime(), fh)
}

// itemFolderName is the top-level folder of an item zip.
func itemFolderName(it *store.Item) string {
	name := it.Title
	if it.Year > 0 && (it.Kind == "movie" || it.Kind == "show") {
		name = fmt.Sprintf("%s (%d)", name, it.Year)
	}
	name = sanitizeFileName(name)
	if name == "" {
		name = fmt.Sprintf("item-%d", it.ID)
	}
	return name
}

func sanitizeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || r == 0x7f:
			return -1
		case strings.ContainsRune(`/\:*?"<>|`, r):
			return '_'
		}
		return r
	}, s)
	s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '.' })
	for len(s) > 150 {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s
}

// relToItem returns a file path relative to the item folder.
func relToItem(itemRel, fileRel string) string {
	if itemRel == "" || itemRel == "." {
		return fileRel
	}
	if rest, ok := strings.CutPrefix(fileRel, itemRel+"/"); ok && rest != "" {
		return rest
	}
	return path.Base(fileRel)
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

func (a *App) handleItemDownload(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if !u.CanDownload {
		forbidden(w, "Downloads are disabled for this account")
		return
	}
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	item, lib, ok := a.loadAccessibleItem(w, r, id)
	if !ok {
		return
	}
	season := -1
	if s := r.URL.Query().Get("season"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			badRequest(w, "season must be a non-negative integer")
			return
		}
		season = n
	}
	files, err := a.store.ItemFiles(r.Context(), item.ID)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	type entry struct {
		name string
		f    store.File
	}
	folder := itemFolderName(item)
	var entries []entry
	var total int64
	for _, f := range files {
		if season >= 0 && !(f.Season == season && (f.Episode > 0 || season > 0)) {
			continue
		}
		rel, ok := cleanRel(relToItem(item.RelPath, f.RelPath))
		if !ok {
			continue
		}
		entries = append(entries, entry{folder + "/" + rel, f})
		total += f.Size
	}
	if len(entries) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "No files to download")
		return
	}

	zipName := folder
	detail := "All files"
	if season >= 0 {
		label := fmt.Sprintf("Season %d", season)
		if season == 0 {
			label = "Specials"
		}
		zipName += " - " + label
		detail = label
	}
	zipName += ".zip"

	fileHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", zipName))
	a.recordDownload(r, u, fmt.Sprintf("item|%d|%d", item.ID, season), "download_item", item, detail, total)
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)

	ctx := r.Context()
	bw := bufio.NewWriterSize(w, 256<<10)
	zw := zip.NewWriter(bw)
	buf := make([]byte, 1<<20)
	for _, e := range entries {
		if ctx.Err() != nil {
			return
		}
		fh, fi, err := openInRoot(lib.Path, e.f.RelPath)
		if err != nil {
			a.log.Warn("zip: skipping unavailable file", "item", item.ID, "relPath", e.f.RelPath, "err", err)
			continue
		}
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Store, Modified: fi.ModTime()}
		hdr.SetMode(0o644)
		ew, err := zw.CreateHeader(hdr)
		if err != nil {
			fh.Close()
			return
		}
		_, err = io.CopyBuffer(ew, ctxReader{ctx, fh}, buf)
		fh.Close()
		if err != nil {
			if ctx.Err() == nil {
				a.log.Info("zip download aborted", "item", item.ID, "err", err)
			}
			return
		}
	}
	if err := zw.Close(); err != nil {
		return
	}
	_ = bw.Flush()
}

// apiURL builds an API-relative URL with a cache-busting version.
func apiURL(p, v string) *string {
	if v == "" {
		return nil
	}
	s := apiPrefix + p + "?v=" + url.QueryEscape(v)
	return &s
}
