package scanner

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// file is one regular file found while walking the library.
type file struct {
	name string // base name
	rel  string // slash-separated, relative to the library root
	stem string // name without extension
	ext  string // lowercase extension without dot
	size int64
	mod  time.Time
	abs  string
}

// dir is one directory of the in-memory library tree.
type dir struct {
	name  string
	rel   string // "" for the root
	abs   string
	files []*file
	dirs  []*dir
}

// ignoredNames are NAS / OS housekeeping entries that are never media.
var ignoredNames = map[string]bool{
	"@eadir": true, "#recycle": true, "#snapshot": true, "$recycle.bin": true,
	"lost+found": true, "thumbs.db": true, "desktop.ini": true,
	"system volume information": true, "@recycle": true, "@recently-snapshot": true,
	"@__thumb": true, "@tmp": true, ".ds_store": true, "$recycle": true,
	"recycler": true, ".trash": true, ".trashes": true,
}

var partialExts = map[string]bool{"part": true, "!qb": true, "crdownload": true, "tmp": true, "partial": true, "aria2": true}

func ignored(name string) bool {
	return strings.HasPrefix(name, ".") || ignoredNames[strings.ToLower(name)]
}

func splitExt(name string) (stem, ext string) {
	e := path.Ext(name)
	if e == "" || len(e) > 6 || e == name {
		return name, ""
	}
	return name[:len(name)-len(e)], strings.ToLower(e[1:])
}

type walker struct {
	ctx      context.Context
	visited  map[string]bool
	maxDepth int
	onError  func(path string, err error)
}

// walk reads the whole library tree. It never writes to disk. Unreadable
// subdirectories are reported to onError and skipped; symlinked directories
// are followed at most once (by resolved path) so loops terminate.
func (s *Scanner) walk(ctx context.Context, root string) (*dir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, &fs.PathError{Op: "scan", Path: root, Err: fs.ErrInvalid}
	}
	w := &walker{ctx: ctx, visited: map[string]bool{}, maxDepth: s.maxDepth, onError: s.onError}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		real = abs
	}
	w.visited[real] = true
	d := &dir{abs: abs}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	if err := w.fill(d, entries, 0); err != nil {
		return nil, err
	}
	return d, nil
}

func (w *walker) fill(d *dir, entries []os.DirEntry, depth int) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if ignored(name) {
			continue
		}
		full := filepath.Join(d.abs, name)
		rel := name
		if d.rel != "" {
			rel = d.rel + "/" + name
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			info, err = os.Stat(full) // follow; broken links are skipped
			if err != nil {
				continue
			}
		}
		switch {
		case info.IsDir():
			if depth+1 > w.maxDepth {
				continue
			}
			real, err := filepath.EvalSymlinks(full)
			if err != nil {
				continue
			}
			if w.visited[real] {
				continue
			}
			w.visited[real] = true
			sub, err := os.ReadDir(full)
			if err != nil {
				if w.onError != nil {
					w.onError(full, err)
				}
				continue
			}
			sd := &dir{name: name, rel: rel, abs: full}
			if err := w.fill(sd, sub, depth+1); err != nil {
				return err
			}
			d.dirs = append(d.dirs, sd)
		case info.Mode().IsRegular():
			stem, ext := splitExt(name)
			if partialExts[ext] {
				continue
			}
			d.files = append(d.files, &file{name: name, rel: rel, stem: stem, ext: ext, size: info.Size(), mod: info.ModTime(), abs: full})
		}
	}
	return nil
}

// hasFile reports whether any file under d (recursively) satisfies pred.
func (d *dir) hasFile(pred func(*file) bool) bool {
	for _, f := range d.files {
		if pred(f) {
			return true
		}
	}
	for _, sd := range d.dirs {
		if sd.hasFile(pred) {
			return true
		}
	}
	return false
}

// fileByName finds a direct child file case-insensitively.
func (d *dir) fileByName(name string) *file {
	for _, f := range d.files {
		if strings.EqualFold(f.name, name) {
			return f
		}
	}
	return nil
}
