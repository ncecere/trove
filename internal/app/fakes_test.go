package app

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"trove/internal/media"
)

// fakeScanner is a tiny stand-in for package scanner: every top-level entry
// of the root is an item; files below it belong to it.
type fakeScanner struct {
	mu    sync.Mutex
	calls int
	err   error
}

var epRe = regexp.MustCompile(`(?i)S(\d+)E(\d+)(?:-?E(\d+))?`)

func roleFor(ext string) media.FileRole {
	switch ext {
	case "mkv", "mp4", "avi":
		return media.RoleVideo
	case "mp3", "m4b", "flac":
		return media.RoleAudio
	case "epub", "mobi", "azw3", "cbz":
		return media.RoleEbook
	case "srt", "ass", "vtt":
		return media.RoleSubtitle
	}
	return media.RoleExtra
}

func (s *fakeScanner) Scan(ctx context.Context, root string, t media.LibraryType) ([]media.ScannedItem, error) {
	s.mu.Lock()
	s.calls++
	err := s.err
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var items []media.ScannedItem
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		it := media.ScannedItem{RelPath: e.Name(), Title: e.Name(), IDs: map[string]string{}}
		switch t {
		case media.LibraryMovies:
			it.Kind = media.KindMovie
		case media.LibraryTV, media.LibraryAnime:
			it.Kind = media.KindShow
		case media.LibraryBooks:
			it.Kind = media.KindBook
		case media.LibraryAudiobooks:
			it.Kind = media.KindAudiobook
		}
		// "Title (Year)"
		if m := regexp.MustCompile(`^(.*) \((\d{4})\)$`).FindStringSubmatch(it.Title); m != nil {
			it.Title = m[1]
			it.Year, _ = strconv.Atoi(m[2])
		}
		// "Author - Title" for books.
		if it.Kind == media.KindBook || it.Kind == media.KindAudiobook {
			if a, b, ok := strings.Cut(it.Title, " - "); ok {
				it.Author, it.Title = a, b
			}
		}
		add := func(rel string) {
			fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil || fi.IsDir() {
				return
			}
			base := path.Base(rel)
			ext := strings.TrimPrefix(strings.ToLower(path.Ext(base)), ".")
			switch strings.ToLower(base) {
			case "poster.jpg", "cover.jpg":
				it.PosterRelPath = rel
				return
			case "fanart.jpg":
				it.BackdropRelPath = rel
				return
			}
			f := media.ScannedFile{RelPath: rel, Size: fi.Size(), ModTime: fi.ModTime(), Role: roleFor(ext), Container: ext}
			if strings.Contains(rel, "/Extras/") {
				f.Role = media.RoleExtra
			}
			if m := epRe.FindStringSubmatch(base); m != nil && it.Kind == media.KindShow {
				f.Season, _ = strconv.Atoi(m[1])
				f.Episode, _ = strconv.Atoi(m[2])
				if m[3] != "" {
					f.EpisodeEnd, _ = strconv.Atoi(m[3])
				}
			}
			if f.Role == media.RoleSubtitle {
				parts := strings.Split(strings.TrimSuffix(base, "."+ext), ".")
				if len(parts) > 1 {
					f.Language = parts[len(parts)-1]
				}
			}
			if strings.Contains(base, "1080p") {
				f.Resolution = "1080p"
			}
			it.Files = append(it.Files, f)
		}
		if e.IsDir() {
			filepath.WalkDir(filepath.Join(root, e.Name()), func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(root, p)
				add(filepath.ToSlash(rel))
				return nil
			})
		} else {
			it.Title = strings.TrimSuffix(it.Title, path.Ext(it.Title))
			add(e.Name())
		}
		if len(it.Files) > 0 {
			items = append(items, it)
		}
	}
	return items, nil
}

func (s *fakeScanner) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// fakeProvider returns canned metadata by title.
type fakeProvider struct {
	name  string
	kinds []media.Kind
	mu    sync.Mutex
	data  map[string]*media.Metadata
	calls map[string]int
}

func newFakeProvider(name string, kinds ...media.Kind) *fakeProvider {
	return &fakeProvider{name: name, kinds: kinds, data: map[string]*media.Metadata{}, calls: map[string]int{}}
}

func (p *fakeProvider) Name() string { return p.name }

func (p *fakeProvider) Supports(k media.Kind) bool {
	for _, x := range p.kinds {
		if x == k {
			return true
		}
	}
	return false
}

func (p *fakeProvider) Lookup(ctx context.Context, q media.MetadataQuery) (*media.Metadata, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls[q.Title]++
	md, ok := p.data[q.Title]
	if !ok {
		return nil, media.ErrNotFound
	}
	cp := *md
	return &cp, nil
}

func (p *fakeProvider) set(title string, md *media.Metadata) {
	p.mu.Lock()
	p.data[title] = md
	p.mu.Unlock()
}

func (p *fakeProvider) callCount(title string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls[title]
}
