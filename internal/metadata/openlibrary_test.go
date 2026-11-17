package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"trove/internal/media"
)

func testOL(srv *httptest.Server) *OpenLibrary {
	return NewOpenLibrary(WithBaseURL(srv.URL), WithRateLimit(0), WithHTTPClient(srv.Client()))
}

func TestOpenLibraryISBN(t *testing.T) {
	f, srv := newFake(t)
	// /isbn/ redirects to the edition, as the real API does.
	f.routes["/isbn/9780765326355.json"] = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/books/OL24275148M.json", http.StatusFound)
	}
	f.json("/books/OL24275148M.json", `{
	  "key": "/books/OL24275148M", "title": "The Way of Kings", "publishers": ["Tor"],
	  "publish_date": "August 31, 2010", "covers": [6979861], "works": [{"key": "/works/OL15358691W"}],
	  "authors": [{"key": "/authors/OL1394865A"}], "isbn_13": ["9780765326355"]}`)
	f.json("/works/OL15358691W.json", `{
	  "key": "/works/OL15358691W", "title": "The Way of Kings",
	  "description": "Roshar is a world of stone and storms.",
	  "subjects": ["Fiction", "Fantasy", "Fiction, fantasy, epic", "Accessible book", "Protected DAISY", "nyt:hardcover-fiction=2010-09-19", "Knights", "Magic"],
	  "first_publish_date": "2010", "covers": [6979861]}`)
	f.json("/authors/OL1394865A.json", `{"key": "/authors/OL1394865A", "name": "Brandon Sanderson"}`)

	md, err := testOL(srv).Lookup(context.Background(), media.MetadataQuery{Kind: media.KindBook, IDs: map[string]string{"isbn": "978-0-7653-2635-5"}})
	if err != nil {
		t.Fatal(err)
	}
	want := &media.Metadata{
		Provider: "openlibrary", Title: "The Way of Kings", Overview: "Roshar is a world of stone and storms.",
		Year: 2010, Genres: []string{"Fantasy", "Knights", "Magic", "Fiction"}, Studio: "Tor",
		Authors:   []string{"Brandon Sanderson"},
		PosterURL: "https://covers.openlibrary.org/b/id/6979861-L.jpg",
		IDs:       map[string]string{"isbn": "9780765326355", "olid": "OL24275148M", "openlibrary": "OL15358691W"},
	}
	if !reflect.DeepEqual(md, want) {
		t.Errorf("got  %+v\nwant %+v", md, want)
	}
	if ua := f.last().Header.Get("User-Agent"); ua != "Trove/0.1 (self-hosted media library)" {
		t.Errorf("User-Agent = %q", ua)
	}
}

func TestOpenLibrarySearchRanking(t *testing.T) {
	f, srv := newFake(t)
	f.json("/search.json", `{"numFound": 3, "docs": [
	  {"key": "/works/OL1W", "title": "Dune Messiah", "author_name": ["Frank Herbert"], "first_publish_year": 1969, "cover_i": 1},
	  {"key": "/works/OL2W", "title": "Dune", "author_name": ["Brian Herbert", "Kevin J. Anderson"], "first_publish_year": 2001, "cover_i": 2},
	  {"key": "/works/OL893415W", "title": "Dune", "author_name": ["Frank Herbert"], "first_publish_year": 1965, "cover_i": 11481354,
	   "isbn": ["0441172717", "9780441172719"], "subject": ["Science fiction", "Fiction, science fiction, general", "Dune (Imaginary place)", "Large type books"],
	   "ratings_average": 4.25}
	]}`)
	f.json("/works/OL893415W.json", `{"key": "/works/OL893415W", "title": "Dune",
	  "description": {"type": "/type/text", "value": "Set on the desert planet Arrakis."}}`)
	md, err := testOL(srv).Lookup(context.Background(), media.MetadataQuery{Kind: media.KindAudiobook, Title: "Dune", Author: "Herbert, Frank"})
	if err != nil {
		t.Fatal(err)
	}
	want := &media.Metadata{
		Provider: "openlibrary", Title: "Dune", Overview: "Set on the desert planet Arrakis.", Year: 1965,
		Genres: []string{"Science Fiction"}, Rating: 8.5, Authors: []string{"Frank Herbert"},
		PosterURL: "https://covers.openlibrary.org/b/id/11481354-L.jpg",
		IDs:       map[string]string{"openlibrary": "OL893415W", "isbn": "9780441172719"},
	}
	if !reflect.DeepEqual(md, want) {
		t.Errorf("got  %+v\nwant %+v", md, want)
	}
	q := f.reqs[0].URL.Query()
	if q.Get("title") != "Dune" || q.Get("author") != "Herbert, Frank" || q.Get("limit") != "5" || q.Get("fields") != olFields {
		t.Errorf("search query = %v", q)
	}
}

func TestOpenLibraryRetryWithoutAuthorAndNotFound(t *testing.T) {
	f, srv := newFake(t)
	f.json("/search.json?author=J.R.R. Tolkien", `{"docs": []}`)
	f.json("/search.json?author=!", `{"docs": [{"key": "/works/OL27482W", "title": "The Hobbit", "author_name": ["J. R. R. Tolkien"], "first_publish_year": 1937}]}`)
	f.json("/works/OL27482W.json", `{"key": "/works/OL27482W", "title": "The Hobbit"}`)
	md, err := testOL(srv).Lookup(context.Background(), media.MetadataQuery{Kind: media.KindBook, Title: "The Hobbit", Author: "J.R.R. Tolkien"})
	if err != nil || md.IDs["openlibrary"] != "OL27482W" || md.Overview != "" {
		t.Fatalf("got %+v %v", md, err)
	}

	// Unknown ISBN falls back to search; a poor match is not accepted.
	f.json("/search.json?title=Some Obscure Book", `{"docs": [{"key": "/works/OL9W", "title": "A Completely Different Thing", "author_name": ["X"]}]}`)
	_, err = testOL(srv).Lookup(context.Background(), media.MetadataQuery{Kind: media.KindBook, Title: "Some Obscure Book", IDs: map[string]string{"isbn": "9780000000002"}})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if _, err := testOL(srv).Lookup(context.Background(), media.MetadataQuery{Kind: media.KindMovie, Title: "x"}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("unsupported: %v", err)
	}
}

func TestOpenLibraryRateLimited(t *testing.T) {
	f, srv := newFake(t)
	calls := 0
	f.routes["/search.json"] = func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}
	_, err := testOL(srv).Lookup(context.Background(), media.MetadataQuery{Kind: media.KindBook, Title: "Dune"})
	if !errors.Is(err, ErrRateLimited) || calls != 3 {
		t.Errorf("err = %v calls = %d", err, calls)
	}
}

func TestOLTextShapes(t *testing.T) {
	var w olWork
	for _, js := range []string{`{"description":"plain"}`, `{"description":{"type":"/type/text","value":"plain"}}`} {
		w = olWork{}
		if err := jsonUnmarshal(js, &w); err != nil || w.Description != "plain" {
			t.Errorf("%s -> %q %v", js, w.Description, err)
		}
	}
}

func TestTrimSubjects(t *testing.T) {
	got := trimSubjects([]string{
		"Fiction", "Fiction, science fiction, general", "Science fiction", "Space warfare", "Accessible book",
		"Protected DAISY", "In library", "Mystery and detective stories", "Interplanetary voyages",
		"Long subject that goes on and on and on forever", "Dune (Imaginary place)", "Large type books", "Humor", "Romance", "Horror",
	}, 5)
	want := []string{"Science Fiction", "Mystery", "Humor", "Romance", "Horror"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("trimSubjects = %v", got)
	}
}

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
