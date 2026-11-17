package metadata

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"trove/internal/media"
)

const (
	tmdbDefaultBase = "https://api.themoviedb.org/3"
	// TMDBImageBase is the prefix of poster / backdrop URLs.
	TMDBImageBase = "https://image.tmdb.org/t/p/"

	tmdbAnimationGenre = 16
)

// TMDB is a media.MetadataProvider for movies and shows.
type TMDB struct {
	c      *httpClient
	key    string
	bearer bool
	lang   string
}

var _ media.MetadataProvider = (*TMDB)(nil)

// NewTMDB returns a TMDB provider. apiKey may be a v3 API key (sent as the
// api_key query parameter) or a v4 read-access token (a JWT starting with
// "eyJ", sent as "Authorization: Bearer").
func NewTMDB(apiKey string, opts ...Option) *TMDB {
	cfg := newConfig(tmdbDefaultBase, 250*time.Millisecond, opts)
	t := &TMDB{key: strings.TrimSpace(apiKey), lang: cfg.language}
	t.bearer = isV4Token(t.key)
	t.c = newHTTPClient("tmdb", cfg)
	t.c.auth = func(req *http.Request, q url.Values) {
		if t.bearer {
			req.Header.Set("Authorization", "Bearer "+t.key)
		} else {
			q.Set("api_key", t.key)
		}
	}
	return t
}

func isV4Token(k string) bool {
	return strings.HasPrefix(k, "eyJ") && strings.Count(k, ".") == 2 && len(k) > 40
}

// Name implements media.MetadataProvider.
func (t *TMDB) Name() string { return "tmdb" }

// Supports implements media.MetadataProvider.
func (t *TMDB) Supports(k media.Kind) bool { return k == media.KindMovie || k == media.KindShow }

type tmdbResult struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	ReleaseDate      string  `json:"release_date"`
	FirstAirDate     string  `json:"first_air_date"`
	Popularity       float64 `json:"popularity"`
	GenreIDs         []int   `json:"genre_ids"`
	OriginalLanguage string  `json:"original_language"`
}

func (r tmdbResult) titles() (string, string) {
	if r.Title != "" || r.OriginalTitle != "" {
		return r.Title, r.OriginalTitle
	}
	return r.Name, r.OriginalName
}

func (r tmdbResult) year() int {
	if r.ReleaseDate != "" {
		return yearOf(r.ReleaseDate)
	}
	return yearOf(r.FirstAirDate)
}

// Lookup implements media.MetadataProvider.
func (t *TMDB) Lookup(ctx context.Context, q media.MetadataQuery) (*media.Metadata, error) {
	if !t.Supports(q.Kind) {
		return nil, fmt.Errorf("tmdb: %w: %q", ErrUnsupported, q.Kind)
	}
	if t.key == "" {
		return nil, fmt.Errorf("tmdb: %w: no API key configured", ErrAuth)
	}
	kind := "movie"
	if q.Kind == media.KindShow {
		kind = "tv"
	}

	// 1. Direct TMDB id.
	if id, err := strconv.Atoi(strings.TrimSpace(q.IDs["tmdb"])); err == nil && id > 0 {
		md, err := t.details(ctx, kind, id, q)
		if !errors.Is(err, media.ErrNotFound) {
			return md, err
		}
	}
	// 2. External ids via /find.
	for _, ext := range []struct{ key, source string }{{"imdb", "imdb_id"}, {"tvdb", "tvdb_id"}} {
		v := strings.TrimSpace(q.IDs[ext.key])
		if v == "" {
			continue
		}
		id, err := t.find(ctx, kind, v, ext.source)
		if err != nil && !errors.Is(err, media.ErrNotFound) {
			return nil, err
		}
		if id > 0 {
			md, err := t.details(ctx, kind, id, q)
			if !errors.Is(err, media.ErrNotFound) {
				return md, err
			}
		}
	}
	// 3. Search.
	if strings.TrimSpace(q.Title) == "" {
		return nil, notFound("tmdb", "no title or id to look up")
	}
	id, err := t.search(ctx, kind, q)
	if err != nil {
		return nil, err
	}
	return t.details(ctx, kind, id, q)
}

func (t *TMDB) find(ctx context.Context, kind, id, source string) (int, error) {
	var res struct {
		MovieResults []tmdbResult `json:"movie_results"`
		TVResults    []tmdbResult `json:"tv_results"`
	}
	if err := t.c.getJSON(ctx, "/find/"+url.PathEscape(id), url.Values{"external_source": {source}}, &res); err != nil {
		return 0, err
	}
	list := res.MovieResults
	if kind == "tv" {
		list = res.TVResults
	}
	if len(list) == 0 {
		return 0, notFound("tmdb", "no %s for %s %s", kind, source, id)
	}
	return list[0].ID, nil
}

func (t *TMDB) searchOnce(ctx context.Context, kind string, q media.MetadataQuery, withYear bool) ([]tmdbResult, error) {
	v := url.Values{"query": {q.Title}, "include_adult": {"false"}, "language": {t.lang}}
	if withYear {
		if kind == "tv" {
			v.Set("first_air_date_year", strconv.Itoa(q.Year))
		} else {
			v.Set("year", strconv.Itoa(q.Year))
		}
	}
	var res struct {
		Results []tmdbResult `json:"results"`
	}
	if err := t.c.getJSON(ctx, "/search/"+kind, v, &res); err != nil {
		return nil, err
	}
	return res.Results, nil
}

// minTMDBScore is the lowest acceptable candidate score (see scoreTMDB).
const minTMDBScore = 65

// search finds the best candidate: with the year first, then without it.
func (t *TMDB) search(ctx context.Context, kind string, q media.MetadataQuery) (int, error) {
	var best *tmdbResult
	bestScore := math.Inf(-1)
	consider := func(rs []tmdbResult) {
		for i := range rs {
			if s := scoreTMDB(rs[i], q); s > bestScore {
				best, bestScore = &rs[i], s
			}
		}
	}
	if q.Year > 0 {
		rs, err := t.searchOnce(ctx, kind, q, true)
		if err != nil {
			return 0, err
		}
		consider(rs)
	}
	if best == nil || bestScore < minTMDBScore+20 {
		rs, err := t.searchOnce(ctx, kind, q, false)
		if err != nil {
			return 0, err
		}
		consider(rs)
	}
	if best == nil || bestScore < minTMDBScore {
		return 0, notFound("tmdb", "no %s matching %q (%d)", kind, q.Title, q.Year)
	}
	return best.ID, nil
}

// scoreTMDB ranks a search result: title similarity (0-100) dominates, then
// year proximity (+30 exact, +20 for ±1, penalties beyond), then popularity
// (≤ +10) and, for anime libraries, Animation genre / Japanese origin.
func scoreTMDB(r tmdbResult, q media.MetadataQuery) float64 {
	title, orig := r.titles()
	sim := max(titleSimilarity(q.Title, title), titleSimilarity(q.Title, orig))
	if sim < 0.6 {
		return math.Inf(-1)
	}
	s := sim * 100
	if y := r.year(); q.Year > 0 {
		switch d := abs(y - q.Year); {
		case y == 0:
			s -= 5
		case d == 0:
			s += 30
		case d == 1:
			s += 20
		case d <= 3:
			s -= 10
		default:
			s -= 50
		}
	}
	s += math.Min(10, math.Log10(1+r.Popularity)*4)
	if q.LibraryType == media.LibraryAnime {
		for _, g := range r.GenreIDs {
			if g == tmdbAnimationGenre {
				s += 15
				break
			}
		}
		if r.OriginalLanguage == "ja" {
			s += 10
		}
	}
	return s
}

type tmdbDetails struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	OriginalTitle    string `json:"original_title"`
	Name             string `json:"name"`
	OriginalName     string `json:"original_name"`
	Tagline          string `json:"tagline"`
	Overview         string `json:"overview"`
	ReleaseDate      string `json:"release_date"`
	FirstAirDate     string `json:"first_air_date"`
	Runtime          int    `json:"runtime"`
	EpisodeRunTime   []int  `json:"episode_run_time"`
	LastEpisodeToAir *struct {
		Runtime int `json:"runtime"`
	} `json:"last_episode_to_air"`
	VoteAverage float64 `json:"vote_average"`
	Status      string  `json:"status"`
	IMDBID      string  `json:"imdb_id"`
	PosterPath  string  `json:"poster_path"`
	Backdrop    string  `json:"backdrop_path"`
	Genres      []struct {
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCompanies []struct {
		Name string `json:"name"`
	} `json:"production_companies"`
	Networks []struct {
		Name string `json:"name"`
	} `json:"networks"`
	ExternalIDs struct {
		IMDBID string `json:"imdb_id"`
		TVDBID *int   `json:"tvdb_id"`
	} `json:"external_ids"`
}

func (t *TMDB) details(ctx context.Context, kind string, id int, q media.MetadataQuery) (*media.Metadata, error) {
	var d tmdbDetails
	v := url.Values{"append_to_response": {"external_ids"}, "language": {t.lang}}
	if err := t.c.getJSON(ctx, "/"+kind+"/"+strconv.Itoa(id), v, &d); err != nil {
		return nil, err
	}
	md := &media.Metadata{
		Provider: "tmdb",
		Tagline:  d.Tagline,
		Overview: d.Overview,
		Rating:   d.VoteAverage,
		Status:   d.Status,
		IDs:      map[string]string{"tmdb": strconv.Itoa(d.ID)},
	}
	if kind == "movie" {
		md.Title, md.OriginalTitle, md.ReleaseDate, md.RuntimeMinutes = d.Title, d.OriginalTitle, d.ReleaseDate, d.Runtime
		if len(d.ProductionCompanies) > 0 {
			md.Studio = d.ProductionCompanies[0].Name
		}
	} else {
		md.Title, md.OriginalTitle, md.ReleaseDate = d.Name, d.OriginalName, d.FirstAirDate
		if len(d.EpisodeRunTime) > 0 {
			md.RuntimeMinutes = d.EpisodeRunTime[0]
		} else if d.LastEpisodeToAir != nil {
			md.RuntimeMinutes = d.LastEpisodeToAir.Runtime
		}
		if len(d.Networks) > 0 {
			md.Studio = d.Networks[0].Name
		}
	}
	md.Year = yearOf(md.ReleaseDate)
	for _, g := range d.Genres {
		md.Genres = append(md.Genres, g.Name)
	}
	if d.PosterPath != "" {
		md.PosterURL = TMDBImageBase + "w500" + d.PosterPath
	}
	if d.Backdrop != "" {
		md.BackdropURL = TMDBImageBase + "w1280" + d.Backdrop
	}
	if imdb := firstNonEmpty(d.IMDBID, d.ExternalIDs.IMDBID); imdb != "" {
		md.IDs["imdb"] = imdb
	}
	if d.ExternalIDs.TVDBID != nil && *d.ExternalIDs.TVDBID > 0 {
		md.IDs["tvdb"] = strconv.Itoa(*d.ExternalIDs.TVDBID)
	}
	if kind == "tv" && len(q.Seasons) > 0 {
		eps, err := t.episodes(ctx, d.ID, q.Seasons)
		if err != nil {
			return nil, err
		}
		md.Episodes = eps
	}
	return md, nil
}

func (t *TMDB) episodes(ctx context.Context, id int, seasons []int) ([]media.EpisodeMetadata, error) {
	seen := map[int]bool{}
	var ss []int
	for _, s := range seasons {
		if s >= 0 && !seen[s] {
			seen[s] = true
			ss = append(ss, s)
		}
	}
	sort.Ints(ss)
	var out []media.EpisodeMetadata
	for _, n := range ss {
		var res struct {
			Episodes []struct {
				SeasonNumber  int    `json:"season_number"`
				EpisodeNumber int    `json:"episode_number"`
				Name          string `json:"name"`
				Overview      string `json:"overview"`
				AirDate       string `json:"air_date"`
			} `json:"episodes"`
		}
		err := t.c.getJSON(ctx, fmt.Sprintf("/tv/%d/season/%d", id, n), url.Values{"language": {t.lang}}, &res)
		if errors.Is(err, media.ErrNotFound) {
			continue // season not on TMDB (e.g. date-based "season 2024")
		}
		if err != nil {
			return nil, err
		}
		for _, e := range res.Episodes {
			sn := e.SeasonNumber
			if sn == 0 && n != 0 {
				sn = n
			}
			out = append(out, media.EpisodeMetadata{Season: sn, Episode: e.EpisodeNumber, Title: e.Name, Overview: e.Overview, AirDate: e.AirDate})
		}
	}
	return out, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
