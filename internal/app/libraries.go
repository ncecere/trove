package app

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"trove/internal/media"
	"trove/internal/store"
)

type libraryJSON struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Path      *string    `json:"path"`
	ItemCount int64      `json:"itemCount"`
	TotalSize int64      `json:"totalSize"`
	Scan      ScanStatus `json:"scan"`
	CreatedAt string     `json:"createdAt"`
}

func (a *App) toLibraryJSON(l *store.Library, u *store.User) libraryJSON {
	j := libraryJSON{
		ID: l.ID, Name: l.Name, Type: string(l.Type), ItemCount: l.ItemCount, TotalSize: l.TotalSize,
		Scan: a.scans.status(l), CreatedAt: fmtTime(l.CreatedAt),
	}
	if u.IsAdmin() {
		p := l.Path
		j.Path = &p
	}
	return j
}

// accessibleLibraries lists the libraries the user can see, sorted by name.
func (a *App) accessibleLibraries(r *http.Request) ([]*store.Library, error) {
	u := userFrom(r)
	libs, err := a.store.ListLibraries(r.Context())
	if err != nil {
		return nil, err
	}
	out := libs[:0]
	for _, l := range libs {
		if u.CanAccess(l.ID) {
			out = append(out, l)
		}
	}
	return out, nil
}

// accessFilter returns the library restriction for queries (nil = none).
func accessFilter(u *store.User) []int64 {
	if u.IsAdmin() || u.LibraryIDs == nil {
		return nil
	}
	return u.LibraryIDs
}

func (a *App) handleListLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := a.accessibleLibraries(r)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	u := userFrom(r)
	out := make([]libraryJSON, 0, len(libs))
	for _, l := range libs {
		out = append(out, a.toLibraryJSON(l, u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": out})
}

func validateLibraryPath(p string) (string, string) {
	p = strings.TrimSpace(p)
	if p == "" || !filepath.IsAbs(p) {
		return "", "path must be an absolute directory path"
	}
	p = filepath.Clean(p)
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		return "", "path is not an existing directory"
	}
	return p, ""
}

func validateLibraryName(n string) (string, string) {
	n = strings.TrimSpace(n)
	if n == "" || len([]rune(n)) > 100 {
		return "", "name must be 1-100 characters"
	}
	return n, ""
}

func (a *App) handleCreateLibrary(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name, msg := validateLibraryName(body.Name)
	if msg != "" {
		badRequest(w, msg)
		return
	}
	typ := media.LibraryType(body.Type)
	if !media.ValidLibraryType(typ) {
		badRequest(w, "type must be one of movies, tv, anime, books, audiobooks")
		return
	}
	path, msg := validateLibraryPath(body.Path)
	if msg != "" {
		badRequest(w, msg)
		return
	}
	lib, err := a.store.CreateLibrary(r.Context(), name, typ, path)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.scans.trigger(lib.ID, false)
	writeJSON(w, http.StatusCreated, map[string]any{"library": a.toLibraryJSON(lib, userFrom(r))})
}

func (a *App) handleUpdateLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	var body struct {
		Name *string `json:"name"`
		Type *string `json:"type"`
		Path *string `json:"path"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	cur, err := a.store.GetLibrary(r.Context(), id)
	if err != nil {
		a.storeErr(w, r, err)
		return
	}
	var name, path *string
	var typ *media.LibraryType
	if body.Name != nil {
		n, msg := validateLibraryName(*body.Name)
		if msg != "" {
			badRequest(w, msg)
			return
		}
		name = &n
	}
	if body.Type != nil {
		t := media.LibraryType(*body.Type)
		if !media.ValidLibraryType(t) {
			badRequest(w, "type must be one of movies, tv, anime, books, audiobooks")
			return
		}
		typ = &t
	}
	if body.Path != nil {
		p, msg := validateLibraryPath(*body.Path)
		if msg != "" {
			badRequest(w, msg)
			return
		}
		path = &p
	}
	lib, err := a.store.UpdateLibrary(r.Context(), id, name, typ, path)
	if err != nil {
		a.storeErr(w, r, err)
		return
	}
	if lib.Type != cur.Type || lib.Path != cur.Path {
		a.scans.trigger(lib.ID, true)
	}
	writeJSON(w, http.StatusOK, map[string]any{"library": a.toLibraryJSON(lib, userFrom(r))})
}

func (a *App) handleDeleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	if _, err := a.store.GetLibrary(r.Context(), id); err != nil {
		a.storeErr(w, r, err)
		return
	}
	a.scans.remove(id)
	if err := a.store.DeleteLibrary(r.Context(), id); err != nil {
		a.storeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleScanLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	lib, err := a.store.GetLibrary(r.Context(), id)
	if err != nil {
		a.storeErr(w, r, err)
		return
	}
	a.scans.trigger(id, false)
	writeJSON(w, http.StatusAccepted, map[string]any{"library": a.toLibraryJSON(lib, userFrom(r))})
}

func (a *App) handleScanAll(w http.ResponseWriter, r *http.Request) {
	libs, err := a.store.ListLibraries(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	u := userFrom(r)
	out := make([]libraryJSON, 0, len(libs))
	for _, l := range libs {
		a.scans.trigger(l.ID, false)
		out = append(out, a.toLibraryJSON(l, u))
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"libraries": out})
}

type dirEntryJSON struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (a *App) handleAdminFS(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimSpace(r.URL.Query().Get("path"))
	if p == "" {
		p = string(filepath.Separator)
	}
	if !filepath.IsAbs(p) {
		badRequest(w, "path must be absolute")
		return
	}
	p = filepath.Clean(p)
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		badRequest(w, "path is not an existing directory")
		return
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		badRequest(w, "cannot read directory: "+err.Error())
		return
	}
	dirs := []dirEntryJSON{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(p, name)
		isDir := e.IsDir()
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(full); err == nil && st.IsDir() {
				isDir = true
			}
		}
		if isDir {
			dirs = append(dirs, dirEntryJSON{Name: name, Path: full})
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		a, b := strings.ToLower(dirs[i].Name), strings.ToLower(dirs[j].Name)
		if a != b {
			return a < b
		}
		return dirs[i].Name < dirs[j].Name
	})
	var parent *string
	if d := filepath.Dir(p); d != p {
		parent = &d
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": p, "parent": parent, "dirs": dirs})
}
