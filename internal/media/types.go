// Package media holds the types shared between the scanner, the metadata
// providers and the application/server. It is the contract between those
// packages: it must stay dependency-free (stdlib only) and must not import any
// other package in this module.
package media

import (
	"context"
	"errors"
	"time"
)

// LibraryType is what a mapped library path contains. It decides how the
// scanner interprets the directory tree.
type LibraryType string

const (
	LibraryMovies     LibraryType = "movies"
	LibraryTV         LibraryType = "tv"
	LibraryAnime      LibraryType = "anime"
	LibraryBooks      LibraryType = "books"
	LibraryAudiobooks LibraryType = "audiobooks"
)

// ValidLibraryType reports whether t is one of the known library types.
func ValidLibraryType(t LibraryType) bool {
	switch t {
	case LibraryMovies, LibraryTV, LibraryAnime, LibraryBooks, LibraryAudiobooks:
		return true
	}
	return false
}

// Kind is the kind of a single library item.
type Kind string

const (
	KindMovie     Kind = "movie"
	KindShow      Kind = "show"
	KindBook      Kind = "book"
	KindAudiobook Kind = "audiobook"
)

// FileRole classifies a file that belongs to an item.
type FileRole string

const (
	RoleVideo    FileRole = "video"    // movie file or episode
	RoleAudio    FileRole = "audio"    // audiobook track / m4b
	RoleEbook    FileRole = "ebook"    // epub, pdf, mobi, azw3, cbz ...
	RoleSubtitle FileRole = "subtitle" // srt, ass, vtt ... (sidecar)
	RoleExtra    FileRole = "extra"    // trailers, featurettes, booklets (pdf next to an audiobook) ...
)

// ScannedFile is one downloadable file found on disk.
type ScannedFile struct {
	// RelPath is slash-separated and relative to the library root.
	RelPath string
	Size    int64
	ModTime time.Time
	Role    FileRole
	// Container is the lowercase file extension without the dot ("mkv", "epub").
	Container string

	// Episode numbering (shows only). Season 0 means "Specials".
	// Episode == 0 means the file is not an episode.
	// EpisodeEnd > Episode for multi-episode files (S01E01-E02), else 0.
	Season       int
	Episode      int
	EpisodeEnd   int
	EpisodeTitle string // parsed from the filename when present

	// Track is the ordering index for audiobook parts (1-based, 0 = unknown).
	Track int

	// Release info parsed from the filename; all optional.
	Resolution string // "2160p", "1080p", "720p", "480p"
	Source     string // "BluRay", "WEB-DL", "WEBRip", "HDTV", "DVD", "Remux"
	VideoCodec string // "x265", "x264", "AV1" ...
	AudioCodec string // "DTS-HD MA", "TrueHD", "AAC", "FLAC" ...
	HDR        string // "HDR10", "DV", "HDR10+" ...
	Edition    string // "Director's Cut", "Extended" ...
	Language   string // subtitles: language code from the filename ("en", "es")
}

// ScannedItem is one logical library item (a movie, a whole show, a book, an
// audiobook) together with every file that belongs to it.
type ScannedItem struct {
	Kind Kind
	// RelPath identifies the item inside its library: the item's folder, or the
	// file itself for a loose file in the library root. Slash-separated,
	// relative to the library root, stable across rescans.
	RelPath string

	Title     string
	SortTitle string // lowercase, leading article stripped ("matrix")
	Year      int    // 0 = unknown

	// Books / audiobooks.
	Author      string
	Narrator    string
	Series      string
	SeriesIndex float64

	// IDs parsed from folder/file names or .nfo files.
	// Keys: "tmdb", "imdb", "tvdb", "isbn", "asin".
	IDs map[string]string

	// Optional data from local .nfo / .opf / metadata files.
	Overview string
	Genres   []string

	// Local artwork, slash-separated and relative to the library root
	// (poster.jpg, folder.jpg, cover.jpg, fanart.jpg, backdrop.jpg ...).
	PosterRelPath   string
	BackdropRelPath string

	Files []ScannedFile
}

// Scanner walks a library root and returns the items found in it.
// Implemented by package scanner.
type Scanner interface {
	Scan(ctx context.Context, root string, libType LibraryType) ([]ScannedItem, error)
}

// ErrNotFound is returned by a MetadataProvider when nothing matches.
var ErrNotFound = errors.New("metadata: not found")

// MetadataQuery describes the item to look up.
type MetadataQuery struct {
	Kind        Kind
	LibraryType LibraryType
	Title       string
	Year        int
	Author      string
	IDs         map[string]string
	// Seasons present on disk (shows only); providers should return episode
	// metadata for these seasons.
	Seasons []int
}

// EpisodeMetadata is per-episode data for shows.
type EpisodeMetadata struct {
	Season   int
	Episode  int
	Title    string
	Overview string
	AirDate  string // YYYY-MM-DD
}

// Metadata is what a provider knows about an item. Every field is optional.
type Metadata struct {
	Provider       string // "tmdb", "openlibrary"
	Title          string
	OriginalTitle  string
	Tagline        string
	Overview       string
	Year           int
	ReleaseDate    string // YYYY-MM-DD
	Genres         []string
	Rating         float64 // 0-10
	RuntimeMinutes int
	Status         string   // shows: "Ended", "Returning Series" ...
	Studio         string   // studio / network / publisher
	Authors        []string // books
	// Absolute URLs of remote artwork; the app downloads and caches them.
	PosterURL   string
	BackdropURL string
	IDs         map[string]string
	Episodes    []EpisodeMetadata
}

// MetadataProvider looks up metadata on a remote service.
// Implemented by package metadata. Returns ErrNotFound when nothing matches.
type MetadataProvider interface {
	Name() string
	// Supports reports whether the provider can handle items of this kind.
	Supports(kind Kind) bool
	Lookup(ctx context.Context, q MetadataQuery) (*Metadata, error)
}
