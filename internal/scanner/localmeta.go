package scanner

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"html"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// localMeta is metadata read from .nfo, metadata.opf or metadata.json.
type localMeta struct {
	Title       string
	Year        int
	Overview    string
	Genres      []string
	IDs         map[string]string
	Authors     []string
	Narrators   []string
	Series      string
	SeriesIndex float64
}

func (m *localMeta) setID(k, v string) {
	v = strings.TrimSpace(v)
	if v == "" || v == "0" {
		return
	}
	if m.IDs == nil {
		m.IDs = map[string]string{}
	}
	if _, ok := m.IDs[k]; !ok {
		m.IDs[k] = v
	}
}

const maxMetaFile = 4 << 20

func readSmall(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxMetaFile))
	if err != nil {
		return nil
	}
	return b
}

func newXMLDecoder(data []byte) *xml.Decoder {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	return d
}

type nfoXML struct {
	XMLName       xml.Name
	Title         string   `xml:"title"`
	OriginalTitle string   `xml:"originaltitle"`
	Year          string   `xml:"year"`
	Premiered     string   `xml:"premiered"`
	Aired         string   `xml:"aired"`
	ReleaseDate   string   `xml:"releasedate"`
	Plot          string   `xml:"plot"`
	Outline       string   `xml:"outline"`
	Genres        []string `xml:"genre"`
	UniqueIDs     []struct {
		Type  string `xml:"type,attr"`
		Value string `xml:",chardata"`
	} `xml:"uniqueid"`
	IMDB  string `xml:"imdbid"`
	IMDB2 string `xml:"imdb_id"`
	TMDB  string `xml:"tmdbid"`
	TVDB  string `xml:"tvdbid"`
	ID    string `xml:"id"`
}

var (
	reNfoIMDB = regexp.MustCompile(`\b(tt\d{7,9})\b`)
	reNfoTMDB = regexp.MustCompile(`(?i)themoviedb\.org/(?:movie|tv)/(\d+)`)
	reNfoTVDB = regexp.MustCompile(`(?i)thetvdb\.com/\S*?(?:[?&](?:series)?id=|/series/)(\d+)`)
	reYear4   = regexp.MustCompile(`\b(1[89]\d\d|20\d\d)\b`)
)

// parseNFO reads a Kodi/Jellyfin .nfo. It also accepts "URL-only" NFOs and
// scene NFOs, from which only ids are extracted.
func parseNFO(data []byte) localMeta {
	var m localMeta
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var x nfoXML
	if bytes.Contains(data, []byte("<")) && newXMLDecoder(data).Decode(&x) == nil {
		switch strings.ToLower(x.XMLName.Local) {
		case "movie", "tvshow", "episodedetails", "musicvideo":
			m.Title = strings.TrimSpace(x.Title)
			if y := atoi(strings.TrimSpace(x.Year)); bookYearOK(y) {
				m.Year = y
			}
			for _, d := range []string{x.Premiered, x.Aired, x.ReleaseDate} {
				if m.Year != 0 {
					break
				}
				if ym := reYear4.FindString(d); ym != "" {
					m.Year = atoi(ym)
				}
			}
			m.Overview = strings.TrimSpace(x.Plot)
			if m.Overview == "" {
				m.Overview = strings.TrimSpace(x.Outline)
			}
			m.Genres = splitGenres(x.Genres)
			for _, u := range x.UniqueIDs {
				t := strings.ToLower(strings.TrimSpace(u.Type))
				switch t {
				case "imdb", "tmdb", "tvdb", "anidb":
					m.setID(t, u.Value)
				}
			}
			m.setID("imdb", x.IMDB)
			m.setID("imdb", x.IMDB2)
			m.setID("tmdb", x.TMDB)
			m.setID("tvdb", x.TVDB)
			if id := strings.TrimSpace(x.ID); id != "" {
				switch {
				case strings.HasPrefix(id, "tt"):
					m.setID("imdb", id)
				case strings.EqualFold(x.XMLName.Local, "tvshow"):
					m.setID("tvdb", id)
				default:
					m.setID("tmdb", id)
				}
			}
		}
	}
	if s := reNfoIMDB.FindSubmatch(data); s != nil {
		m.setID("imdb", string(s[1]))
	}
	if s := reNfoTMDB.FindSubmatch(data); s != nil {
		m.setID("tmdb", string(s[1]))
	}
	if s := reNfoTVDB.FindSubmatch(data); s != nil {
		m.setID("tvdb", string(s[1]))
	}
	if v, ok := m.IDs["imdb"]; ok && !strings.HasPrefix(v, "tt") {
		delete(m.IDs, "imdb")
	}
	return m
}

func splitGenres(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, g := range in {
		for _, p := range strings.FieldsFunc(g, func(r rune) bool { return r == '/' || r == '|' || r == ';' }) {
			p = strings.TrimSpace(p)
			if p == "" || seen[strings.ToLower(p)] {
				continue
			}
			seen[strings.ToLower(p)] = true
			out = append(out, p)
		}
	}
	return out
}

type opfXML struct {
	Metadata struct {
		Titles   []string `xml:"title"`
		Creators []struct {
			Role  string `xml:"role,attr"`
			ID    string `xml:"id,attr"`
			Value string `xml:",chardata"`
		} `xml:"creator"`
		Descriptions []string `xml:"description"`
		Subjects     []string `xml:"subject"`
		Identifiers  []struct {
			Scheme string `xml:"scheme,attr"`
			Value  string `xml:",chardata"`
		} `xml:"identifier"`
		Dates []string `xml:"date"`
		Metas []struct {
			Name     string `xml:"name,attr"`
			Content  string `xml:"content,attr"`
			Property string `xml:"property,attr"`
			Refines  string `xml:"refines,attr"`
			ID       string `xml:"id,attr"`
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
}

// parseOPF reads a Calibre / EPUB metadata.opf.
func parseOPF(data []byte) (localMeta, bool) {
	var m localMeta
	var x opfXML
	if err := newXMLDecoder(data).Decode(&x); err != nil {
		return m, false
	}
	md := x.Metadata
	if len(md.Titles) > 0 {
		m.Title = strings.TrimSpace(md.Titles[0])
	}
	roles := map[string]string{} // EPUB3 refines: creator id -> role
	for _, me := range md.Metas {
		if me.Property == "role" && strings.HasPrefix(me.Refines, "#") {
			roles[me.Refines[1:]] = strings.TrimSpace(me.Value)
		}
	}
	for _, c := range md.Creators {
		name := strings.TrimSpace(c.Value)
		if name == "" {
			continue
		}
		role := strings.ToLower(c.Role)
		if role == "" {
			role = strings.ToLower(roles[c.ID])
		}
		switch role {
		case "", "aut":
			m.Authors = append(m.Authors, name)
		case "nrt":
			m.Narrators = append(m.Narrators, name)
		}
	}
	for _, d := range md.Descriptions {
		if t := stripHTML(d); t != "" {
			m.Overview = t
			break
		}
	}
	m.Genres = splitGenres(md.Subjects)
	for _, id := range md.Identifiers {
		v := strings.TrimSpace(id.Value)
		scheme := strings.ToLower(id.Scheme)
		lv := strings.ToLower(v)
		switch {
		case scheme == "isbn" || strings.HasPrefix(lv, "urn:isbn:") || strings.HasPrefix(lv, "isbn:"):
			if isbn := normISBN(v); isbn != "" {
				m.setID("isbn", isbn)
			}
		case scheme == "amazon" || scheme == "mobi-asin" || scheme == "asin" || strings.HasPrefix(lv, "urn:asin:"):
			m.setID("asin", strings.TrimPrefix(strings.TrimPrefix(v, "urn:asin:"), "URN:ASIN:"))
		case scheme == "goodreads":
			m.setID("goodreads", v)
		case scheme == "google":
			m.setID("google", v)
		}
	}
	for _, d := range md.Dates {
		if y := reYear4.FindString(d); y != "" {
			m.Year = atoi(y)
			break
		}
	}
	collections := map[string]string{}
	for _, me := range md.Metas {
		switch {
		case me.Name == "calibre:series":
			m.Series = strings.TrimSpace(me.Content)
		case me.Name == "calibre:series_index":
			m.SeriesIndex, _ = strconv.ParseFloat(strings.TrimSpace(me.Content), 64)
		case me.Property == "belongs-to-collection":
			collections[me.ID] = strings.TrimSpace(me.Value)
			if m.Series == "" {
				m.Series = strings.TrimSpace(me.Value)
			}
		}
	}
	for _, me := range md.Metas {
		if me.Property == "group-position" && strings.HasPrefix(me.Refines, "#") && m.SeriesIndex == 0 {
			if collections[me.Refines[1:]] == m.Series {
				m.SeriesIndex, _ = strconv.ParseFloat(strings.TrimSpace(me.Value), 64)
			}
		}
	}
	return m, true
}

var (
	reBlockTags = regexp.MustCompile(`(?i)<\s*(?:br\s*/?|/p|/div|/li|/h\d)\s*>`)
	reAnyTag    = regexp.MustCompile(`(?s)<[^>]*>`)
)

func stripHTML(s string) string {
	s = reBlockTags.ReplaceAllString(s, "\n")
	s = reAnyTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if l = collapseSpaces(l); l != "" {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

// parseABSMetadata reads an Audiobookshelf metadata.json.
func parseABSMetadata(data []byte) (localMeta, bool) {
	var raw struct {
		Title         string          `json:"title"`
		Subtitle      *string         `json:"subtitle"`
		Authors       json.RawMessage `json:"authors"`
		Narrators     json.RawMessage `json:"narrators"`
		Series        json.RawMessage `json:"series"`
		Genres        []string        `json:"genres"`
		PublishedYear json.RawMessage `json:"publishedYear"`
		Description   *string         `json:"description"`
		ISBN          *string         `json:"isbn"`
		ASIN          *string         `json:"asin"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return localMeta{}, false
	}
	m := localMeta{Title: strings.TrimSpace(raw.Title)}
	m.Authors = jsonNames(raw.Authors)
	m.Narrators = jsonNames(raw.Narrators)
	m.Genres = splitGenres(raw.Genres)
	if raw.Description != nil {
		m.Overview = stripHTML(*raw.Description)
	}
	if raw.ISBN != nil {
		if isbn := normISBN(*raw.ISBN); isbn != "" {
			m.setID("isbn", isbn)
		}
	}
	if raw.ASIN != nil {
		m.setID("asin", *raw.ASIN)
	}
	if len(raw.PublishedYear) > 0 {
		var ys string
		if json.Unmarshal(raw.PublishedYear, &ys) != nil {
			var yn int
			if json.Unmarshal(raw.PublishedYear, &yn) == nil {
				ys = strconv.Itoa(yn)
			}
		}
		if y := reYear4.FindString(ys); y != "" {
			m.Year = atoi(y)
		}
	}
	// series: ["Name #1"] or [{"name":..,"sequence":..}]
	if len(raw.Series) > 0 {
		var ss []string
		var so []struct {
			Name     string          `json:"name"`
			Sequence json.RawMessage `json:"sequence"`
		}
		if json.Unmarshal(raw.Series, &ss) == nil && len(ss) > 0 {
			s := strings.TrimSpace(ss[0])
			if mm := reSeriesHash.FindStringSubmatch(s); mm != nil {
				m.Series = strings.TrimSpace(mm[1])
				m.SeriesIndex, _ = strconv.ParseFloat(mm[2], 64)
			} else {
				m.Series = s
			}
		} else if json.Unmarshal(raw.Series, &so) == nil && len(so) > 0 {
			m.Series = strings.TrimSpace(so[0].Name)
			seq := strings.Trim(string(so[0].Sequence), `"`)
			m.SeriesIndex, _ = strconv.ParseFloat(seq, 64)
		}
	}
	return m, true
}

func jsonNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		var out []string
		for _, s := range ss {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	var objs []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &objs) == nil {
		var out []string
		for _, o := range objs {
			if s := strings.TrimSpace(o.Name); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	var one string
	if json.Unmarshal(raw, &one) == nil && strings.TrimSpace(one) != "" {
		return []string{strings.TrimSpace(one)}
	}
	return nil
}
