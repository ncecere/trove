package metadata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"trove/internal/media"
)

// fakeAPI serves canned JSON per "path?key=value" route and records requests.
type fakeAPI struct {
	t      *testing.T
	mu     sync.Mutex
	reqs   []*http.Request
	routes map[string]func(w http.ResponseWriter, r *http.Request)
}

func newFake(t *testing.T) (*fakeAPI, *httptest.Server) {
	f := &fakeAPI{t: t, routes: map[string]func(http.ResponseWriter, *http.Request){}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.reqs = append(f.reqs, r.Clone(context.Background()))
		f.mu.Unlock()
		// Most specific route first: path plus any matching query constraints.
		best, bestLen := (func(http.ResponseWriter, *http.Request))(nil), -1
		for k, h := range f.routes {
			path, cond, _ := strings.Cut(k, "?")
			if path != r.URL.Path {
				continue
			}
			ok := true
			if cond != "" {
				for _, c := range strings.Split(cond, "&") {
					kk, vv, _ := strings.Cut(c, "=")
					if vv == "!" {
						ok = ok && !r.URL.Query().Has(kk)
					} else {
						ok = ok && r.URL.Query().Get(kk) == vv
					}
				}
			}
			if ok && len(cond) > bestLen {
				best, bestLen = h, len(cond)
			}
		}
		if best == nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"status_code":34,"status_message":"The resource you requested could not be found."}`))
			return
		}
		best(w, r)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeAPI) json(route, body string) {
	f.routes[route] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}
}

func (f *fakeAPI) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		out = append(out, r.URL.Path)
	}
	return out
}

func (f *fakeAPI) last() *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

const v4Token = "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiJ0ZXN0IiwibmJmIjoxNzAwMDAwMDAwfQ.c2lnbmF0dXJlLXNpZ25hdHVyZS1zaWduYXR1cmU"

func testTMDB(srv *httptest.Server, key string, extra ...Option) *TMDB {
	opts := append([]Option{WithBaseURL(srv.URL), WithRateLimit(0), WithHTTPClient(srv.Client())}, extra...)
	return NewTMDB(key, opts...)
}

const matrixDetails = `{
 "id": 603, "imdb_id": "tt0133093", "title": "The Matrix", "original_title": "The Matrix",
 "tagline": "Welcome to the Real World.", "overview": "Set in the 22nd century...",
 "release_date": "1999-03-30", "runtime": 136, "vote_average": 8.2, "status": "Released",
 "poster_path": "/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg", "backdrop_path": "/fNG7i7RqMErkcqhohV2a6cV1Ehy.jpg",
 "genres": [{"id": 28, "name": "Action"}, {"id": 878, "name": "Science Fiction"}],
 "production_companies": [{"id": 79, "name": "Village Roadshow Pictures"}, {"id": 372, "name": "Groucho II Film Partnership"}],
 "external_ids": {"imdb_id": "tt0133093", "facebook_id": "TheMatrixMovie"}
}`

var matrixWant = &media.Metadata{
	Provider: "tmdb", Title: "The Matrix", OriginalTitle: "The Matrix", Tagline: "Welcome to the Real World.",
	Overview: "Set in the 22nd century...", Year: 1999, ReleaseDate: "1999-03-30",
	Genres: []string{"Action", "Science Fiction"}, Rating: 8.2, RuntimeMinutes: 136, Status: "Released",
	Studio:      "Village Roadshow Pictures",
	PosterURL:   "https://image.tmdb.org/t/p/w500/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg",
	BackdropURL: "https://image.tmdb.org/t/p/w1280/fNG7i7RqMErkcqhohV2a6cV1Ehy.jpg",
	IDs:         map[string]string{"tmdb": "603", "imdb": "tt0133093"},
}

func TestTMDBMovieSearchRanking(t *testing.T) {
	f, srv := newFake(t)
	// results[0] is a popular near-miss; the right one is further down.
	f.json("/search/movie?year=1999", `{"page":1,"results":[
	  {"id": 9999, "title": "The Matrix Revisited", "release_date": "2001-11-19", "popularity": 300},
	  {"id": 1111, "title": "The Matrix", "release_date": "1993-01-01", "popularity": 2.1},
	  {"id": 603, "title": "The Matrix", "original_title": "The Matrix", "release_date": "1999-03-30", "popularity": 80.5}
	],"total_results":3}`)
	f.json("/movie/603", matrixDetails)
	md, err := testTMDB(srv, "v3key").Lookup(context.Background(), media.MetadataQuery{Kind: media.KindMovie, Title: "Matrix", Year: 1999})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(md, matrixWant) {
		t.Errorf("got  %+v\nwant %+v", md, matrixWant)
	}
	r := f.last()
	if r.URL.Query().Get("api_key") != "v3key" || r.Header.Get("Authorization") != "" {
		t.Errorf("v3 auth not used: %v %v", r.URL.RawQuery, r.Header)
	}
	if r.URL.Query().Get("append_to_response") != "external_ids" || r.Header.Get("User-Agent") != UserAgent {
		t.Errorf("details request = %v %v", r.URL, r.Header)
	}
}

func TestTMDBRetryWithoutYearAndTolerance(t *testing.T) {
	f, srv := newFake(t)
	f.json("/search/movie?year=2018", `{"results":[]}`)
	f.json("/search/movie?year=!", `{"results":[
	  {"id": 5, "title": "Movie Title", "release_date": "2005-06-01", "popularity": 50},
	  {"id": 6, "title": "Movie Title", "release_date": "2019-01-04", "popularity": 3}
	]}`)
	f.json("/movie/6", `{"id": 6, "title": "Movie Title", "release_date": "2019-01-04"}`)
	md, err := testTMDB(srv, "k").Lookup(context.Background(), media.MetadataQuery{Kind: media.KindMovie, Title: "Movie Title", Year: 2018})
	if err != nil {
		t.Fatal(err)
	}
	if md.IDs["tmdb"] != "6" || md.Year != 2019 {
		t.Errorf("got %+v", md)
	}
	if got := f.paths(); !reflect.DeepEqual(got, []string{"/search/movie", "/search/movie", "/movie/6"}) {
		t.Errorf("requests = %v", got)
	}
}

func TestTMDBWrongYearRejected(t *testing.T) {
	f, srv := newFake(t)
	f.json("/search/movie", `{"results":[{"id": 5, "title": "Movie Title", "release_date": "1950-06-01", "popularity": 9}]}`)
	_, err := testTMDB(srv, "k").Lookup(context.Background(), media.MetadataQuery{Kind: media.KindMovie, Title: "Movie Title", Year: 2019})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestTMDBIDLookups(t *testing.T) {
	f, srv := newFake(t)
	f.json("/movie/603", matrixDetails)
	f.json("/find/tt0133093?external_source=imdb_id", `{"movie_results":[{"id":603,"title":"The Matrix"}],"tv_results":[]}`)
	f.json("/find/81189?external_source=tvdb_id", `{"movie_results":[],"tv_results":[{"id":1396,"name":"Breaking Bad"}]}`)
	f.json("/tv/1396", `{"id":1396,"name":"Breaking Bad","first_air_date":"2008-01-20","external_ids":{"imdb_id":"tt0903747","tvdb_id":81189}}`)
	p := testTMDB(srv, v4Token)
	ctx := context.Background()

	md, err := p.Lookup(ctx, media.MetadataQuery{Kind: media.KindMovie, Title: "ignored", IDs: map[string]string{"tmdb": "603"}})
	if err != nil || md.Title != "The Matrix" {
		t.Fatalf("tmdb id: %v %v", md, err)
	}
	if got := f.paths(); !reflect.DeepEqual(got, []string{"/movie/603"}) {
		t.Errorf("tmdb id requests = %v", got)
	}
	r := f.last()
	if r.Header.Get("Authorization") != "Bearer "+v4Token || r.URL.Query().Has("api_key") {
		t.Errorf("v4 auth not used: %v %v", r.Header, r.URL.RawQuery)
	}

	md, err = p.Lookup(ctx, media.MetadataQuery{Kind: media.KindMovie, IDs: map[string]string{"imdb": "tt0133093"}})
	if err != nil || md.IDs["tmdb"] != "603" {
		t.Fatalf("imdb: %v %v", md, err)
	}

	md, err = p.Lookup(ctx, media.MetadataQuery{Kind: media.KindShow, IDs: map[string]string{"tvdb": "81189"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(md.IDs, map[string]string{"tmdb": "1396", "imdb": "tt0903747", "tvdb": "81189"}) || md.Year != 2008 {
		t.Errorf("tvdb lookup = %+v", md)
	}

	// A stale tmdb id falls back to search.
	f.json("/search/movie", `{"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-30"}]}`)
	md, err = p.Lookup(ctx, media.MetadataQuery{Kind: media.KindMovie, Title: "The Matrix", IDs: map[string]string{"tmdb": "1"}})
	if err != nil || md.IDs["tmdb"] != "603" {
		t.Errorf("stale id fallback: %v %v", md, err)
	}
}

func TestTMDBShowWithEpisodes(t *testing.T) {
	f, srv := newFake(t)
	f.json("/search/tv?first_air_date_year=2008", `{"results":[{"id":1396,"name":"Breaking Bad","original_name":"Breaking Bad","first_air_date":"2008-01-20","popularity":400}]}`)
	f.json("/tv/1396", `{
	  "id":1396,"name":"Breaking Bad","original_name":"Breaking Bad","tagline":"Remember my name","overview":"A teacher...",
	  "first_air_date":"2008-01-20","episode_run_time":[45,47],"vote_average":8.9,"status":"Ended",
	  "networks":[{"id":174,"name":"AMC"}],"genres":[{"id":18,"name":"Drama"}],
	  "poster_path":"/p.jpg","backdrop_path":"/b.jpg",
	  "external_ids":{"imdb_id":"tt0903747","tvdb_id":81189}}`)
	f.json("/tv/1396/season/1", `{"season_number":1,"episodes":[
	  {"season_number":1,"episode_number":1,"name":"Pilot","overview":"Walter...","air_date":"2008-01-20"},
	  {"season_number":1,"episode_number":2,"name":"Cat's in the Bag...","overview":"","air_date":"2008-01-27"}]}`)
	f.json("/tv/1396/season/0", `{"episodes":[{"season_number":0,"episode_number":1,"name":"Good Cop Bad Cop","air_date":"2009-02-17"}]}`)
	md, err := testTMDB(srv, "k").Lookup(context.Background(), media.MetadataQuery{
		Kind: media.KindShow, Title: "Breaking Bad", Year: 2008, Seasons: []int{1, 0, 1, 2024},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := &media.Metadata{
		Provider: "tmdb", Title: "Breaking Bad", OriginalTitle: "Breaking Bad", Tagline: "Remember my name", Overview: "A teacher...",
		Year: 2008, ReleaseDate: "2008-01-20", Genres: []string{"Drama"}, Rating: 8.9, RuntimeMinutes: 45, Status: "Ended", Studio: "AMC",
		PosterURL: "https://image.tmdb.org/t/p/w500/p.jpg", BackdropURL: "https://image.tmdb.org/t/p/w1280/b.jpg",
		IDs: map[string]string{"tmdb": "1396", "imdb": "tt0903747", "tvdb": "81189"},
		Episodes: []media.EpisodeMetadata{
			{Season: 0, Episode: 1, Title: "Good Cop Bad Cop", AirDate: "2009-02-17"},
			{Season: 1, Episode: 1, Title: "Pilot", Overview: "Walter...", AirDate: "2008-01-20"},
			{Season: 1, Episode: 2, Title: "Cat's in the Bag...", AirDate: "2008-01-27"},
		},
	}
	if !reflect.DeepEqual(md, want) {
		t.Errorf("got  %+v\nwant %+v", md, want)
	}
}

func TestTMDBAnimePrefersAnimation(t *testing.T) {
	f, srv := newFake(t)
	f.json("/search/tv", `{"results":[
	  {"id":1,"name":"Monster","first_air_date":"2004-04-07","popularity":60,"genre_ids":[18,80],"original_language":"en"},
	  {"id":2,"name":"Monster","first_air_date":"2004-04-07","popularity":25,"genre_ids":[16,9648],"original_language":"ja"}
	]}`)
	f.json("/tv/1", `{"id":1,"name":"Monster (live action)"}`)
	f.json("/tv/2", `{"id":2,"name":"Monster"}`)
	p := testTMDB(srv, "k")
	md, err := p.Lookup(context.Background(), media.MetadataQuery{Kind: media.KindShow, LibraryType: media.LibraryAnime, Title: "Monster"})
	if err != nil || md.IDs["tmdb"] != "2" {
		t.Errorf("anime: %v %v", md, err)
	}
	md, err = p.Lookup(context.Background(), media.MetadataQuery{Kind: media.KindShow, LibraryType: media.LibraryTV, Title: "Monster"})
	if err != nil || md.IDs["tmdb"] != "1" {
		t.Errorf("tv: %v %v", md, err)
	}
}

func TestTMDBErrors(t *testing.T) {
	ctx := context.Background()
	q := media.MetadataQuery{Kind: media.KindMovie, Title: "Nothing"}

	f, srv := newFake(t)
	f.json("/search/movie", `{"results":[]}`)
	if _, err := testTMDB(srv, "k").Lookup(ctx, q); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("empty search err = %v", err)
	}

	f.routes["/search/movie"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"status_code":7,"status_message":"Invalid API key: You must be granted a valid key.","success":false}`))
	}
	_, err := testTMDB(srv, "secret-key").Lookup(ctx, q)
	if !errors.Is(err, ErrAuth) || !strings.Contains(err.Error(), "Invalid API key") || strings.Contains(err.Error(), "secret-key") {
		t.Errorf("401 err = %v", err)
	}
	var he *HTTPError
	if !errors.As(err, &he) || he.StatusCode != 401 {
		t.Errorf("not an HTTPError: %#v", err)
	}

	f.routes["/search/movie"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if _, err := testTMDB(srv, "k").Lookup(ctx, q); !errors.Is(err, ErrServer) {
		t.Errorf("500 err = %v", err)
	}

	if _, err := testTMDB(srv, "").Lookup(ctx, q); !errors.Is(err, ErrAuth) {
		t.Errorf("no key err = %v", err)
	}
	if _, err := testTMDB(srv, "k").Lookup(ctx, media.MetadataQuery{Kind: media.KindBook, Title: "x"}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("unsupported err = %v", err)
	}
}

func TestTMDB429Retry(t *testing.T) {
	f, srv := newFake(t)
	calls := 0
	f.routes["/search/movie"] = func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"status_code":25,"status_message":"Your request count is over the allowed limit."}`))
			return
		}
		w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-30"}]}`))
	}
	f.json("/movie/603", matrixDetails)
	md, err := testTMDB(srv, "k").Lookup(context.Background(), media.MetadataQuery{Kind: media.KindMovie, Title: "The Matrix"})
	if err != nil || md.IDs["tmdb"] != "603" || calls != 2 {
		t.Fatalf("retry: %v %v calls=%d", md, err, calls)
	}

	// Persistent 429: bounded retries, then ErrRateLimited.
	calls = 0
	f.routes["/search/movie"] = func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}
	_, err = testTMDB(srv, "k", WithMaxRetries(2)).Lookup(context.Background(), media.MetadataQuery{Kind: media.KindMovie, Title: "The Matrix"})
	if !errors.Is(err, ErrRateLimited) || calls != 3 {
		t.Errorf("persistent 429: %v calls=%d", err, calls)
	}

	// Retry-After beyond the cap: fail fast instead of blocking.
	calls = 0
	f.routes["/search/movie"] = func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}
	start := time.Now()
	_, err = testTMDB(srv, "k").Lookup(context.Background(), media.MetadataQuery{Kind: media.KindMovie, Title: "The Matrix"})
	if !errors.Is(err, ErrRateLimited) || calls != 1 || time.Since(start) > 2*time.Second {
		t.Errorf("long retry-after: %v calls=%d", err, calls)
	}
}

func TestTMDBContextAndRateLimit(t *testing.T) {
	f, srv := newFake(t)
	f.json("/search/movie", `{"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-30"}]}`)
	f.json("/movie/603", matrixDetails)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := testTMDB(srv, "k").Lookup(ctx, media.MetadataQuery{Kind: media.KindMovie, Title: "The Matrix"}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled err = %v", err)
	}
	p := testTMDB(srv, "k", WithRateLimit(40*time.Millisecond))
	start := time.Now()
	for i := 0; i < 2; i++ { // 2 lookups x 2 requests
		if _, err := p.Lookup(context.Background(), media.MetadataQuery{Kind: media.KindMovie, Title: "The Matrix"}); err != nil {
			t.Fatal(err)
		}
	}
	if el := time.Since(start); el < 120*time.Millisecond {
		t.Errorf("rate limiter not applied: 4 requests in %v", el)
	}
}

func TestTMDBAuthDetection(t *testing.T) {
	if !isV4Token(v4Token) || isV4Token("0123456789abcdef0123456789abcdef") || isV4Token("eyJshort") {
		t.Error("isV4Token")
	}
}

func TestSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		min  float64
		max  float64
	}{
		{"The Matrix", "Matrix", 1, 1},
		{"Amélie", "Amelie", 1, 1},
		{"Mission Impossible Dead Reckoning Part One", "Mission: Impossible - Dead Reckoning Part One", 1, 1},
		{"Fast & Furious", "Fast and Furious", 1, 1},
		{"The Matrix", "The Matrix Reloaded", 0.6, 0.9},
		{"Alien", "Aliens", 0.8, 0.95},
		{"Alien", "Predator", 0, 0.3},
	}
	for _, c := range cases {
		if s := similarity(c.a, c.b); s < c.min || s > c.max {
			t.Errorf("similarity(%q,%q) = %v", c.a, c.b, s)
		}
	}
	if personSimilarity("Tolkien, J.R.R.", "J. R. R. Tolkien") < 0.8 || personSimilarity("Andy Weir", "Stephen King") > 0.2 {
		t.Error("personSimilarity")
	}
}
