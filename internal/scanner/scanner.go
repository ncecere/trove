// Package scanner walks a library folder (read-only) and turns the files in
// it into media.ScannedItems, understanding the common Plex / Jellyfin /
// Sonarr / Radarr / Readarr / Calibre / Audiobookshelf layouts as well as raw
// scene-release names.
//
// Choices for ambiguous cases:
//
//   - Date-based episodes ("Show.2024.03.15.mkv") get Season = year and
//     Episode = month*100+day (2024-03-15 -> S2024E0315, Episode 315).
//   - Anime OVA / OAD / SP files with a number ("Show - OVA 02", "Show OVA - 2",
//     "[..] Show - 05 [OVA]") become season 0 episodes; unnumbered ones, creditless
//     NCOP/NCED, PV/CM/menus and recap half-episodes ("- 12.5") are RoleExtra.
//   - Absolute anime numbering without a season marker goes to season 1 unless
//     a season folder (or "2nd Season" / "S2" in the name) says otherwise.
//   - Video files with no episode number in tv/anime are kept as RoleExtra.
//   - Multi-part movies (cd1/cd2, part1, "CD1/" folders) set ScannedFile.Track
//     to the part number when two or more parts exist.
//   - Loose episode files directly in a tv/anime root are grouped by parsed
//     show title; the item's RelPath is the first (natural order) file.
//   - Audiobook Track is a continuous 1-based index across all audio files of
//     the book (disc folders included), ordered by (disc, track, natural name).
package scanner

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"trove/internal/media"
)

// Scanner implements media.Scanner.
type Scanner struct {
	maxDepth int
	onError  func(path string, err error)
}

// Option configures a Scanner.
type Option func(*Scanner)

// WithMaxDepth limits how deep the walker descends (default 16).
func WithMaxDepth(n int) Option { return func(s *Scanner) { s.maxDepth = n } }

// WithErrorHandler receives non-fatal errors (unreadable directories).
func WithErrorHandler(fn func(path string, err error)) Option {
	return func(s *Scanner) { s.onError = fn }
}

// New returns a Scanner.
func New(opts ...Option) *Scanner {
	s := &Scanner{maxDepth: 16}
	for _, o := range opts {
		o(s)
	}
	return s
}

var _ media.Scanner = (*Scanner)(nil)

// Scan walks root and returns the items found, sorted by SortTitle then
// RelPath. It never modifies the library.
func (s *Scanner) Scan(ctx context.Context, root string, libType media.LibraryType) ([]media.ScannedItem, error) {
	if !media.ValidLibraryType(libType) {
		return nil, fmt.Errorf("scanner: unknown library type %q", libType)
	}
	tree, err := s.walk(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("scanner: %w", err)
	}
	b := &builder{ctx: ctx}
	switch libType {
	case media.LibraryMovies:
		b.scanMovies(tree)
	case media.LibraryTV:
		b.scanShows(tree, false)
	case media.LibraryAnime:
		b.scanShows(tree, true)
	case media.LibraryBooks:
		b.scanBooks(tree)
	case media.LibraryAudiobooks:
		b.scanAudiobooks(tree)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items := b.items
	for i := range items {
		finalize(&items[i])
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortTitle != items[j].SortTitle {
			return items[i].SortTitle < items[j].SortTitle
		}
		return items[i].RelPath < items[j].RelPath
	})
	return items, nil
}

type builder struct {
	ctx   context.Context
	items []media.ScannedItem
}

func (b *builder) add(it media.ScannedItem) {
	if len(it.Files) == 0 {
		return
	}
	b.items = append(b.items, it)
}

func finalize(it *media.ScannedItem) {
	it.Title = strings.TrimSpace(it.Title)
	if it.Title == "" {
		it.Title = it.RelPath[strings.LastIndexByte(it.RelPath, '/')+1:]
	}
	it.SortTitle = SortTitle(it.Title)
	if len(it.IDs) == 0 {
		it.IDs = nil
	}
	if len(it.Genres) == 0 {
		it.Genres = nil
	}
	sort.SliceStable(it.Files, func(i, j int) bool { return NaturalLess(it.Files[i].RelPath, it.Files[j].RelPath) })
}

// ---- extensions ----

func setOf(xs ...string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

var (
	videoExts = setOf("mkv", "mp4", "m4v", "avi", "mov", "wmv", "mpg", "mpeg", "ts", "m2ts", "mts", "webm", "flv", "vob", "iso", "ogv", "3gp", "divx", "rmvb", "asf", "strm")
	audioExts = setOf("mp3", "m4b", "m4a", "flac", "opus", "ogg", "oga", "aac", "wma", "wav", "mka")
	ebookExts = setOf("epub", "mobi", "azw", "azw3", "pdf", "fb2", "djvu", "cbz", "cbr", "cb7", "txt", "kfx")
	subExts   = setOf("srt", "ass", "ssa", "vtt", "sub", "idx", "sup", "smi")
	imageExts = []string{"jpg", "jpeg", "png", "webp"}
)

func isVideo(f *file) bool    { return videoExts[f.ext] }
func isAudio(f *file) bool    { return audioExts[f.ext] }
func isEbook(f *file) bool    { return ebookExts[f.ext] }
func isSubtitle(f *file) bool { return subExts[f.ext] }

func newFile(f *file, role media.FileRole) media.ScannedFile {
	return media.ScannedFile{RelPath: f.rel, Size: f.size, ModTime: f.mod, Role: role, Container: f.ext}
}

func applyRelease(sf *media.ScannedFile, r Release) {
	sf.Resolution, sf.Source, sf.VideoCodec, sf.AudioCodec, sf.HDR = r.Resolution, r.Source, r.VideoCodec, r.AudioCodec, r.HDR
	sf.Edition = r.Edition
}

// ---- artwork / nfo ----

// findArt looks for local artwork in d. folder enables the generic names
// (poster.jpg, folder.jpg ...); bases are file stems for "<base>-poster.jpg"
// and "<base>.jpg".
func findArt(d *dir, folder bool, bases ...string) (poster, backdrop string) {
	var posterStems, backStems []string
	if folder {
		posterStems = append(posterStems, "poster", "folder", "cover", "movie", "show")
		backStems = append(backStems, "fanart", "backdrop", "background")
	}
	for _, b := range bases {
		if b == "" {
			continue
		}
		posterStems = append(posterStems, b+"-poster", b+"-cover", b)
		backStems = append(backStems, b+"-fanart", b+"-backdrop", b+"-background")
	}
	lookup := func(stems []string) string {
		for _, st := range stems {
			for _, e := range imageExts {
				if f := d.fileByName(st + "." + e); f != nil {
					return f.rel
				}
			}
		}
		return ""
	}
	return lookup(posterStems), lookup(backStems)
}

// findNFO parses the first .nfo found among names, then (if any) the only
// remaining .nfo in d.
func findNFO(d *dir, anyNFO bool, names ...string) (localMeta, bool) {
	for _, n := range names {
		if n == "" {
			continue
		}
		if f := d.fileByName(n); f != nil {
			if data := readSmall(f.abs); data != nil {
				return parseNFO(data), true
			}
		}
	}
	if anyNFO {
		for _, f := range d.files {
			if f.ext == "nfo" {
				if data := readSmall(f.abs); data != nil {
					return parseNFO(data), true
				}
			}
		}
	}
	return localMeta{}, false
}

func mergeIDs(dst map[string]string, srcs ...map[string]string) map[string]string {
	if dst == nil {
		dst = map[string]string{}
	}
	for _, src := range srcs {
		for k, v := range src {
			if v == "" {
				continue
			}
			if _, ok := dst[k]; !ok {
				dst[k] = v
			}
		}
	}
	return dst
}

// applyNFO overlays .nfo data onto an item.
func applyNFO(it *media.ScannedItem, m localMeta) {
	if m.Title != "" {
		it.Title = m.Title
	}
	if m.Year != 0 {
		it.Year = m.Year
	}
	if m.Overview != "" {
		it.Overview = m.Overview
	}
	if len(m.Genres) > 0 {
		it.Genres = m.Genres
	}
	// NFO ids are authoritative: they override ids guessed from names.
	if len(m.IDs) > 0 {
		it.IDs = mergeIDs(m.IDs, it.IDs)
	}
}

// subtitleFor attaches a subtitle's language using the longest video stem
// that prefixes it.
func matchVideoStem(sub *file, stems []string) string {
	best := ""
	for _, s := range stems {
		if len(s) > len(best) && len(sub.stem) >= len(s) && strings.EqualFold(sub.stem[:len(s)], s) {
			best = s
		}
	}
	return best
}
