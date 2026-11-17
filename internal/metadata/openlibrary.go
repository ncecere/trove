package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"trove/internal/media"
)

const (
	olDefaultBase = "https://openlibrary.org"
	// OpenLibraryCoversBase is the prefix of cover URLs.
	OpenLibraryCoversBase = "https://covers.openlibrary.org/b/id/"
)

// OpenLibrary is a media.MetadataProvider for books and audiobooks. It needs
// no API key.
type OpenLibrary struct {
	c *httpClient
}

var _ media.MetadataProvider = (*OpenLibrary)(nil)

// NewOpenLibrary returns an Open Library provider (rate limited to 1 req/s by
// default, as Open Library asks).
func NewOpenLibrary(opts ...Option) *OpenLibrary {
	return &OpenLibrary{c: newHTTPClient("openlibrary", newConfig(olDefaultBase, time.Second, opts))}
}

// Name implements media.MetadataProvider.
func (o *OpenLibrary) Name() string { return "openlibrary" }

// Supports implements media.MetadataProvider.
func (o *OpenLibrary) Supports(k media.Kind) bool {
	return k == media.KindBook || k == media.KindAudiobook
}

// Lookup implements media.MetadataProvider.
func (o *OpenLibrary) Lookup(ctx context.Context, q media.MetadataQuery) (*media.Metadata, error) {
	if !o.Supports(q.Kind) {
		return nil, fmt.Errorf("openlibrary: %w: %q", ErrUnsupported, q.Kind)
	}
	if isbn := cleanISBN(q.IDs["isbn"]); isbn != "" {
		md, err := o.byISBN(ctx, isbn)
		if !errors.Is(err, media.ErrNotFound) {
			return md, err
		}
	}
	if strings.TrimSpace(q.Title) == "" {
		return nil, notFound("openlibrary", "no title or isbn to look up")
	}
	return o.bySearch(ctx, q)
}

func cleanISBN(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= '0' && r <= '9') || r == 'X' {
			b.WriteRune(r)
		}
	}
	if n := b.Len(); n == 10 || n == 13 {
		return b.String()
	}
	return ""
}

// olText is Open Library's "string or {type, value}" text field.
type olText string

func (t *olText) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*t = olText(s)
		return nil
	}
	var o struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return nil // tolerate unexpected shapes
	}
	*t = olText(o.Value)
	return nil
}

type olKey struct {
	Key string `json:"key"`
}

type olWork struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	Description      olText   `json:"description"`
	Subjects         []string `json:"subjects"`
	Covers           []int    `json:"covers"`
	FirstPublishDate string   `json:"first_publish_date"`
	Authors          []struct {
		Author olKey `json:"author"`
	} `json:"authors"`
}

func (o *OpenLibrary) work(ctx context.Context, key string) (*olWork, error) {
	key = "/works/" + strings.TrimPrefix(strings.TrimPrefix(key, "/works/"), "/")
	var w olWork
	if err := o.c.getJSON(ctx, key+".json", nil, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

func olID(key string) string {
	return key[strings.LastIndexByte(key, '/')+1:]
}

func coverURL(id int) string {
	if id <= 0 {
		return ""
	}
	return OpenLibraryCoversBase + strconv.Itoa(id) + "-L.jpg"
}

func (o *OpenLibrary) byISBN(ctx context.Context, isbn string) (*media.Metadata, error) {
	var ed struct {
		Key         string   `json:"key"`
		Title       string   `json:"title"`
		Subtitle    string   `json:"subtitle"`
		Publishers  []string `json:"publishers"`
		PublishDate string   `json:"publish_date"`
		Covers      []int    `json:"covers"`
		Works       []olKey  `json:"works"`
		Authors     []olKey  `json:"authors"`
		Subjects    []string `json:"subjects"`
		Description olText   `json:"description"`
	}
	if err := o.c.getJSON(ctx, "/isbn/"+isbn+".json", nil, &ed); err != nil {
		return nil, err
	}
	md := &media.Metadata{Provider: "openlibrary", Title: ed.Title, IDs: map[string]string{"isbn": isbn}}
	if ed.Key != "" {
		md.IDs["olid"] = olID(ed.Key)
	}
	if len(ed.Publishers) > 0 {
		md.Studio = ed.Publishers[0]
	}
	md.Year = yearIn(ed.PublishDate)
	md.Overview = string(ed.Description)
	subjects := ed.Subjects
	if len(ed.Covers) > 0 {
		md.PosterURL = coverURL(ed.Covers[0])
	}
	authorKeys := ed.Authors
	if len(ed.Works) > 0 {
		w, err := o.work(ctx, ed.Works[0].Key)
		if err != nil && !errors.Is(err, media.ErrNotFound) {
			return nil, err
		}
		if w != nil {
			md.IDs["openlibrary"] = olID(w.Key)
			if md.Overview == "" {
				md.Overview = string(w.Description)
			}
			if len(subjects) == 0 {
				subjects = w.Subjects
			}
			if md.PosterURL == "" && len(w.Covers) > 0 {
				md.PosterURL = coverURL(w.Covers[0])
			}
			if y := yearIn(w.FirstPublishDate); y > 0 && (md.Year == 0 || y < md.Year) {
				md.Year = y
			}
			if len(authorKeys) == 0 {
				for _, a := range w.Authors {
					authorKeys = append(authorKeys, a.Author)
				}
			}
		}
	}
	for i, a := range authorKeys {
		if i == 2 || a.Key == "" {
			break
		}
		var au struct {
			Name string `json:"name"`
		}
		err := o.c.getJSON(ctx, "/authors/"+olID(a.Key)+".json", nil, &au)
		if err != nil && !errors.Is(err, media.ErrNotFound) {
			return nil, err
		}
		if au.Name != "" {
			md.Authors = append(md.Authors, au.Name)
		}
	}
	md.Overview = strings.TrimSpace(md.Overview)
	md.Genres = trimSubjects(subjects, 5)
	return md, nil
}

type olDoc struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	CoverI           int      `json:"cover_i"`
	ISBN             []string `json:"isbn"`
	Subject          []string `json:"subject"`
	RatingsAverage   float64  `json:"ratings_average"`
}

const olFields = "key,title,author_name,first_publish_year,cover_i,isbn,subject,ratings_average"

func (o *OpenLibrary) search(ctx context.Context, title, author string) ([]olDoc, error) {
	v := url.Values{"title": {title}, "limit": {"5"}, "fields": {olFields}}
	if author != "" {
		v.Set("author", author)
	}
	var res struct {
		Docs []olDoc `json:"docs"`
	}
	if err := o.c.getJSON(ctx, "/search.json", v, &res); err != nil {
		return nil, err
	}
	return res.Docs, nil
}

const minOLScore = 60

// scoreOL ranks a search doc: title similarity (0-100), author agreement
// (up to +30 / -25), year proximity (+10 / +5) and a small bonus for a cover.
func scoreOL(d olDoc, q media.MetadataQuery) float64 {
	sim := titleSimilarity(q.Title, d.Title)
	if sim < 0.6 {
		return math.Inf(-1)
	}
	s := sim * 100
	if q.Author != "" {
		best := 0.0
		for _, a := range d.AuthorName {
			best = max(best, authorSimilarity(q.Author, a))
		}
		switch {
		case best >= 0.8:
			s += 30 * best // graded: exact name beats a shared surname
		case len(d.AuthorName) > 0:
			s -= 25
		}
	}
	if q.Year > 0 && d.FirstPublishYear > 0 {
		switch abs(q.Year - d.FirstPublishYear) {
		case 0:
			s += 10
		case 1:
			s += 5
		}
	}
	if d.CoverI > 0 {
		s += 2
	}
	return s
}

// authorSimilarity handles "A & B" query strings.
func authorSimilarity(query, cand string) float64 {
	best := 0.0
	for _, part := range strings.FieldsFunc(query, func(r rune) bool { return r == '&' || r == ';' }) {
		best = max(best, personSimilarity(part, cand))
	}
	return best
}

func (o *OpenLibrary) bySearch(ctx context.Context, q media.MetadataQuery) (*media.Metadata, error) {
	author := q.Author
	if i := strings.IndexAny(author, "&;"); i > 0 {
		author = strings.TrimSpace(author[:i])
	}
	docs, err := o.search(ctx, q.Title, author)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 && author != "" {
		// Author spelled differently than Open Library has it.
		if docs, err = o.search(ctx, q.Title, ""); err != nil {
			return nil, err
		}
	}
	var best *olDoc
	bestScore := math.Inf(-1)
	for i := range docs {
		if s := scoreOL(docs[i], q); s > bestScore {
			best, bestScore = &docs[i], s
		}
	}
	if best == nil || bestScore < minOLScore {
		return nil, notFound("openlibrary", "no book matching %q by %q", q.Title, q.Author)
	}
	d := best
	md := &media.Metadata{
		Provider: "openlibrary",
		Title:    d.Title,
		Authors:  d.AuthorName,
		Year:     d.FirstPublishYear,
		IDs:      map[string]string{},
	}
	if d.RatingsAverage > 0 {
		md.Rating = math.Round(d.RatingsAverage*2*100) / 100
	}
	md.PosterURL = coverURL(d.CoverI)
	if d.Key != "" {
		md.IDs["openlibrary"] = olID(d.Key)
	}
	if isbn := pickISBN(d.ISBN); isbn != "" {
		md.IDs["isbn"] = isbn
	}
	subjects := d.Subject
	if d.Key != "" {
		w, err := o.work(ctx, d.Key)
		if err != nil && !errors.Is(err, media.ErrNotFound) {
			return nil, err
		}
		if w != nil {
			md.Overview = strings.TrimSpace(string(w.Description))
			if len(subjects) == 0 {
				subjects = w.Subjects
			}
			if md.PosterURL == "" && len(w.Covers) > 0 {
				md.PosterURL = coverURL(w.Covers[0])
			}
		}
	}
	md.Genres = trimSubjects(subjects, 5)
	return md, nil
}

func pickISBN(list []string) string {
	first := ""
	for _, s := range list {
		c := cleanISBN(s)
		if c == "" {
			continue
		}
		if len(c) == 13 {
			return c
		}
		if first == "" {
			first = c
		}
	}
	return first
}

func yearIn(s string) int {
	for i := 0; i+4 <= len(s); i++ {
		if (i == 0 || !isDigit(s[i-1])) && (i+4 == len(s) || !isDigit(s[i+4])) {
			if y := yearOf(s[i : i+4]); y >= 1000 {
				return y
			}
		}
	}
	return 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// genreCanon maps Open Library subjects to a short list of display genres.
var genreCanon = map[string]string{
	"fantasy": "Fantasy", "epic fantasy": "Fantasy", "fantasy fiction": "Fantasy", "urban fantasy": "Fantasy",
	"science fiction": "Science Fiction", "sci-fi": "Science Fiction", "space opera": "Science Fiction",
	"mystery": "Mystery", "mystery and detective stories": "Mystery", "detective and mystery stories": "Mystery", "mystery fiction": "Mystery",
	"thriller": "Thriller", "thrillers": "Thriller", "suspense": "Thriller", "suspense fiction": "Thriller",
	"romance": "Romance", "love stories": "Romance", "romance fiction": "Romance",
	"horror": "Horror", "horror fiction": "Horror", "horror stories": "Horror",
	"historical fiction": "Historical Fiction", "history": "History",
	"biography": "Biography", "autobiography": "Biography", "memoir": "Memoir", "memoirs": "Memoir",
	"young adult": "Young Adult", "young adult fiction": "Young Adult",
	"children's fiction": "Children's", "juvenile fiction": "Children's", "children's stories": "Children's",
	"poetry": "Poetry", "drama": "Drama", "plays": "Drama",
	"adventure": "Adventure", "adventure stories": "Adventure", "adventure fiction": "Adventure",
	"humor": "Humor", "humorous stories": "Humor", "humorous fiction": "Humor",
	"philosophy": "Philosophy", "science": "Science", "self-help": "Self-Help", "business": "Business",
	"travel": "Travel", "cooking": "Cooking", "religion": "Religion", "psychology": "Psychology",
	"crime": "Crime", "crime fiction": "Crime", "true crime": "True Crime",
	"classics": "Classics", "classic literature": "Classics",
	"dystopia": "Dystopian", "dystopias": "Dystopian", "dystopian fiction": "Dystopian",
	"graphic novels": "Comics", "comics": "Comics", "comic books, strips, etc": "Comics",
	"nonfiction": "Nonfiction", "non-fiction": "Nonfiction",
	"short stories": "Short Stories", "literary fiction": "Literary Fiction",
	"politics": "Politics", "economics": "Economics", "art": "Art", "music": "Music",
	"war": "War", "war stories": "War", "westerns": "Western", "western stories": "Western", "satire": "Satire",
	"fiction": "Fiction",
}

var subjectJunk = []string{
	"accessible book", "protected daisy", "in library", "lending library", "overdrive",
	"large type", "nyt:", "new york times", "open library", "reading level", "award",
	"translations", "staff picks", "long now", "internet archive", "textbooks", "open_syllabus",
	"=", "readers", "collectionid",
}

// trimSubjects reduces Open Library's long subject list to at most n display
// genres: known genres first (in subject order), then short clean subjects.
func trimSubjects(subjects []string, n int) []string {
	var canon, other []string
	seen := map[string]bool{}
	fiction := false
	for _, raw := range subjects {
		s := strings.ToLower(strings.TrimSpace(raw))
		if s == "" || len(s) > 40 {
			continue
		}
		junk := false
		for _, j := range subjectJunk {
			if strings.Contains(s, j) {
				junk = true
				break
			}
		}
		if junk {
			continue
		}
		key := strings.TrimPrefix(s, "fiction, ")
		if i := strings.Index(key, ", "); i > 0 {
			key = key[:i]
		}
		if g, ok := genreCanon[key]; ok {
			if g == "Fiction" {
				fiction = true
				continue
			}
			if !seen[g] {
				seen[g] = true
				canon = append(canon, g)
			}
			continue
		}
		if strings.ContainsAny(s, ",()0123456789:/") || len(strings.Fields(s)) > 3 {
			continue
		}
		g := titleCase(s)
		if !seen[g] {
			seen[g] = true
			other = append(other, g)
		}
	}
	out := append(canon, other...)
	if fiction && !seen["Fiction"] {
		out = append(out, "Fiction")
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
