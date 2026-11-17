package app

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"trove/internal/media"
)

func TestSetupLoginLogoutMe(t *testing.T) {
	e := newEnv(t)
	c := e.client()

	var st struct{ NeedsSetup bool }
	c.must("GET", "/api/v1/setup/status", nil, 200, &st)
	if !st.NeedsSetup {
		t.Fatal("needsSetup should be true")
	}
	c.must("GET", "/api/v1/auth/me", nil, 401, nil)

	if r := c.req("POST", "/api/v1/setup", map[string]string{"username": "x", "password": "supersecret"}); r.StatusCode != 400 {
		t.Fatalf("short username: %d", r.StatusCode)
	}
	if r := c.req("POST", "/api/v1/setup", map[string]string{"username": "admin", "password": "short"}); r.StatusCode != 400 {
		t.Fatalf("short password: %d", r.StatusCode)
	}

	var u userResp
	r := c.must("POST", "/api/v1/setup", map[string]string{"username": "Admin", "password": "supersecret"}, 200, &u)
	if u.User.Role != "admin" || u.User.LibraryIDs != nil || !u.User.CanDownload {
		t.Fatalf("bad user: %+v", u.User)
	}
	var cookie *http.Cookie
	for _, ck := range r.Cookies() {
		if ck.Name == "trove_session" {
			cookie = ck
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Secure {
		t.Fatalf("bad cookie: %+v", cookie)
	}
	if !strings.Contains(string(r.body), `"libraryIds":null`) || !strings.Contains(string(r.body), `"lastLoginAt":"`) {
		t.Fatalf("bad user json: %s", r.body)
	}

	c.must("GET", "/api/v1/setup/status", nil, 200, &st)
	if st.NeedsSetup {
		t.Fatal("needsSetup should be false")
	}
	if r := e.client().req("POST", "/api/v1/setup", map[string]string{"username": "other", "password": "supersecret"}); r.StatusCode != 409 || r.errCode(t) != "conflict" {
		t.Fatalf("second setup: %d %s", r.StatusCode, r.body)
	}

	c.must("GET", "/api/v1/auth/me", nil, 200, &u)
	if u.User.Username != "Admin" {
		t.Fatal(u.User.Username)
	}
	c.must("POST", "/api/v1/auth/logout", nil, 204, nil)
	c.must("GET", "/api/v1/auth/me", nil, 401, nil)

	// Old cookie is dead server-side too.
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	if res, _ := http.DefaultClient.Do(req); res.StatusCode != 401 {
		t.Fatalf("old session still valid: %d", res.StatusCode)
	}

	if r := c.req("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "wrongwrong"}); r.StatusCode != 401 || r.errCode(t) != "unauthorized" {
		t.Fatalf("bad login: %d %s", r.StatusCode, r.body)
	}
	// Case-insensitive username.
	c.must("POST", "/api/v1/auth/login", map[string]string{"username": "ADMIN", "password": "supersecret"}, 200, nil)
	c.must("GET", "/api/v1/auth/me", nil, 200, nil)

	// Secure cookie behind an HTTPS proxy.
	r = e.client().req("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "supersecret"}, "X-Forwarded-Proto", "https")
	if !strings.Contains(r.Header.Get("Set-Cookie"), "Secure") {
		t.Fatalf("expected Secure cookie: %s", r.Header.Get("Set-Cookie"))
	}

	// Login activity recorded.
	var act struct {
		Items []activityJSON `json:"items"`
		Total int            `json:"total"`
	}
	c.must("GET", "/api/v1/admin/activity", nil, 200, &act)
	if act.Total < 2 || act.Items[0].Action != "login" {
		t.Fatalf("activity: %+v", act)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	e.admin()
	c := e.client()
	for i := 0; i < limitPerUser; i++ {
		if r := c.req("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "nopenopenope"}); r.StatusCode != 401 {
			t.Fatalf("attempt %d: %d", i, r.StatusCode)
		}
	}
	r := c.req("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "supersecret"})
	if r.StatusCode != 429 || r.errCode(t) != "rate_limited" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("expected 429: %d %s", r.StatusCode, r.body)
	}
	// Unknown users are limited too.
	for i := 0; i < limitPerUser; i++ {
		c.req("POST", "/api/v1/auth/login", map[string]string{"username": "ghost", "password": "nopenopenope"})
	}
	if r := c.req("POST", "/api/v1/auth/login", map[string]string{"username": "ghost", "password": "x"}); r.StatusCode != 429 {
		t.Fatalf("unknown user not limited: %d", r.StatusCode)
	}
}

func TestCSRF(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/v1/auth/login"},
		{"POST", "/api/v1/setup"},
		{"POST", "/api/v1/libraries"},
		{"PUT", "/api/v1/settings"},
		{"DELETE", "/api/v1/users/1"},
	} {
		r := admin.req(tc.method, tc.path, map[string]string{}, "X-Requested-With", "")
		if r.StatusCode != 403 || r.errCode(t) != "csrf" {
			t.Fatalf("%s %s without header: %d %s", tc.method, tc.path, r.StatusCode, r.body)
		}
		r = admin.req(tc.method, tc.path, map[string]string{}, "X-Requested-With", "XMLHttpRequest")
		if r.StatusCode != 403 || r.errCode(t) != "csrf" {
			t.Fatalf("%s %s wrong header: %d", tc.method, tc.path, r.StatusCode)
		}
	}
	// GET is fine.
	admin.must("GET", "/api/v1/auth/me", nil, 200, nil)
}

func TestAdminOnly(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	user, uid := e.userClient(admin, "bob", nil)
	dir := t.TempDir()
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/v1/users", nil},
		{"POST", "/api/v1/users", map[string]any{"username": "eve", "password": "password123"}},
		{"PATCH", "/api/v1/users/" + itoa(uid), map[string]any{"role": "admin"}},
		{"DELETE", "/api/v1/users/1", nil},
		{"POST", "/api/v1/libraries", map[string]any{"name": "x", "type": "movies", "path": dir}},
		{"POST", "/api/v1/libraries/scan", nil},
		{"GET", "/api/v1/admin/fs?path=/", nil},
		{"GET", "/api/v1/settings", nil},
		{"PUT", "/api/v1/settings", map[string]any{"scanIntervalMinutes": 5}},
		{"GET", "/api/v1/admin/activity", nil},
		{"POST", "/api/v1/items/1/refresh", nil},
	} {
		r := user.req(tc.method, tc.path, tc.body)
		if r.StatusCode != 403 || r.errCode(t) != "forbidden" {
			t.Fatalf("%s %s as user: %d %s", tc.method, tc.path, r.StatusCode, r.body)
		}
	}
	// Unauthenticated: 401.
	if r := e.client().req("GET", "/api/v1/users", nil); r.StatusCode != 401 {
		t.Fatalf("anon: %d", r.StatusCode)
	}
	admin.must("GET", "/api/v1/users", nil, 200, nil)
	var fsResp struct {
		Path   string         `json:"path"`
		Parent *string        `json:"parent"`
		Dirs   []dirEntryJSON `json:"dirs"`
	}
	os.Mkdir(filepath.Join(dir, "Movies"), 0o755)
	os.Mkdir(filepath.Join(dir, ".hidden"), 0o755)
	writeFile(t, dir, "file.txt", "x")
	admin.must("GET", "/api/v1/admin/fs?path="+dir, nil, 200, &fsResp)
	if len(fsResp.Dirs) != 1 || fsResp.Dirs[0].Name != "Movies" || fsResp.Parent == nil {
		t.Fatalf("fs: %+v", fsResp)
	}
	admin.must("GET", "/api/v1/admin/fs", nil, 200, &fsResp)
	if fsResp.Path != "/" || fsResp.Parent != nil {
		t.Fatalf("fs root: %+v", fsResp)
	}
}

func TestLastAdminProtection(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	var me userResp
	admin.must("GET", "/api/v1/auth/me", nil, 200, &me)
	id := itoa(me.User.ID)

	if r := admin.req("PATCH", "/api/v1/users/"+id, map[string]any{"role": "user"}); r.StatusCode != 409 {
		t.Fatalf("demote last admin: %d", r.StatusCode)
	}
	if r := admin.req("DELETE", "/api/v1/users/"+id, nil); r.StatusCode != 409 {
		t.Fatalf("delete self: %d", r.StatusCode)
	}
	// Second admin.
	var u2 userResp
	admin.must("POST", "/api/v1/users", map[string]any{"username": "admin2", "password": "password123", "role": "admin", "libraryIds": []int64{}}, 201, &u2)
	if u2.User.LibraryIDs != nil {
		t.Fatal("admins must have libraryIds null")
	}
	// Duplicate username (case-insensitive).
	if r := admin.req("POST", "/api/v1/users", map[string]any{"username": "ADMIN2", "password": "password123"}); r.StatusCode != 409 {
		t.Fatalf("dup username: %d", r.StatusCode)
	}
	admin2 := e.client()
	admin2.must("POST", "/api/v1/auth/login", map[string]string{"username": "admin2", "password": "password123"}, 200, nil)
	// Now demoting the first admin works.
	admin.must("PATCH", "/api/v1/users/"+id, map[string]any{"role": "user"}, 200, nil)
	// admin2 is now the last admin: can't be demoted or deleted by anyone.
	if r := admin2.req("PATCH", "/api/v1/users/"+itoa(u2.User.ID), map[string]any{"role": "user"}); r.StatusCode != 409 {
		t.Fatalf("demote last admin2: %d", r.StatusCode)
	}
	if r := admin2.req("DELETE", "/api/v1/users/"+itoa(u2.User.ID), nil); r.StatusCode != 409 {
		t.Fatalf("delete self admin2: %d", r.StatusCode)
	}
	// Former admin is now a user.
	if r := admin.req("GET", "/api/v1/users", nil); r.StatusCode != 403 {
		t.Fatalf("demoted user still admin: %d", r.StatusCode)
	}
	admin2.must("DELETE", "/api/v1/users/"+id, nil, 204, nil)
	admin.must("GET", "/api/v1/auth/me", nil, 401, nil) // sessions gone with the user
}

func TestPasswordChangeInvalidatesOtherSessions(t *testing.T) {
	e := newEnv(t)
	a1 := e.admin()
	a2 := e.client()
	a2.must("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "supersecret"}, 200, nil)
	if r := a1.req("PUT", "/api/v1/auth/password", map[string]string{"currentPassword": "bad", "newPassword": "newsecret1"}); r.StatusCode != 400 {
		t.Fatalf("wrong current: %d", r.StatusCode)
	}
	a1.must("PUT", "/api/v1/auth/password", map[string]string{"currentPassword": "supersecret", "newPassword": "newsecret1"}, 204, nil)
	a1.must("GET", "/api/v1/auth/me", nil, 200, nil)
	a2.must("GET", "/api/v1/auth/me", nil, 401, nil)
	e.client().must("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "newsecret1"}, 200, nil)
}

// buildMovies creates a movies library tree.
func buildMovies(t *testing.T) string {
	root := t.TempDir()
	writeFile(t, root, "The Matrix (1999)/The.Matrix.1999.1080p.mkv", "matrix-video-bytes")
	writeFile(t, root, "The Matrix (1999)/The.Matrix.1999.en.srt", "1\n00:00 --> 00:01\nhi\n")
	writeFile(t, root, "The Matrix (1999)/poster.jpg", "\xff\xd8\xff\xe0fakejpeg")
	writeFile(t, root, "The Matrix (1999)/Extras/Trailer.mp4", "trailer")
	writeFile(t, root, "Amélie (2001)/Amélie (2001).mkv", "amelie-bytes")
	writeFile(t, root, "Unknown Film (2010)/film.mkv", "unknown")
	return root
}

func buildShows(t *testing.T) string {
	root := t.TempDir()
	writeFile(t, root, "Severance/Season 01/Severance.S01E01.mkv", "s1e1")
	writeFile(t, root, "Severance/Season 01/Severance.S01E01.en.srt", "sub1")
	writeFile(t, root, "Severance/Season 01/Severance.S01E02-E03.mkv", "s1e2e3")
	writeFile(t, root, "Severance/Season 02/Severance.S02E01.mkv", "s2e1")
	writeFile(t, root, "Severance/Specials/Severance.S00E01.mkv", "special")
	writeFile(t, root, "Severance/fanart.jpg", "\x89PNG\r\n\x1a\nfake")
	return root
}

func buildBooks(t *testing.T) string {
	root := t.TempDir()
	writeFile(t, root, "Frank Herbert - Dune/Dune.epub", "PK-epub")
	writeFile(t, root, "Frank Herbert - Dune/cover.jpg", "\xff\xd8\xff\xe0cover")
	return root
}

func TestScanMovieShowBook(t *testing.T) {
	e := newEnv(t)
	e.prov.set("The Matrix", &media.Metadata{
		Title: "The Matrix", Overview: "Neo.", Year: 1999, Genres: []string{"Action", "Science Fiction"}, Rating: 8.2,
		Tagline: "Free your mind", IDs: map[string]string{"tmdb": "603"}, BackdropURL: "",
	})
	e.prov.set("Severance", &media.Metadata{
		Title: "Severance", Overview: "Work/life.", Genres: []string{"Drama"}, Status: "Returning Series",
		Episodes: []media.EpisodeMetadata{
			{Season: 1, Episode: 1, Title: "Good News About Hell", AirDate: "2022-02-18"},
			{Season: 2, Episode: 1, Title: "Hello, Ms. Cobel"},
		},
	})
	e.prov.set("Dune", &media.Metadata{Title: "Dune", Authors: []string{"Frank Herbert"}, Genres: []string{"Science Fiction"}, Year: 1965})
	admin := e.admin()
	movies := admin.createLibrary("Movies", "movies", buildMovies(t))
	shows := admin.createLibrary("TV", "tv", buildShows(t))
	books := admin.createLibrary("Books", "books", buildBooks(t))

	ml := admin.library(movies)
	if ml.Scan.State != "idle" || ml.Scan.FinishedAt == nil || ml.ItemCount != 3 || ml.TotalSize == 0 || ml.Path == nil {
		t.Fatalf("movies library: %+v", ml)
	}

	// Movie.
	m := admin.findItem("?library="+itoa(movies), "The Matrix")
	if m.Kind != "movie" || m.Year == nil || *m.Year != 1999 || m.Rating == nil || m.PosterURL == nil || !strings.Contains(*m.PosterURL, "?v=") {
		t.Fatalf("matrix summary: %+v", m)
	}
	if m.SeasonCount != nil || m.Author != nil {
		t.Fatal("movie must not have seasonCount/author")
	}
	d := admin.detail(m.ID)
	if d.Overview == nil || *d.Overview != "Neo." || !slices.Equal(d.Genres, []string{"Action", "Science Fiction"}) || d.ExternalIDs["tmdb"] != "603" {
		t.Fatalf("matrix detail: %+v", d)
	}
	if d.BackdropURL != nil || len(d.Seasons) != 0 {
		t.Fatalf("matrix backdrop/seasons: %+v", d)
	}
	var names []string
	for _, f := range d.Files {
		names = append(names, f.RelPath+":"+f.Role)
	}
	if !slices.Equal(names, []string{"The.Matrix.1999.1080p.mkv:video", "The.Matrix.1999.en.srt:subtitle", "Extras/Trailer.mp4:extra"}) {
		t.Fatalf("matrix files: %v", names)
	}
	if d.Files[0].Resolution == nil || *d.Files[0].Resolution != "1080p" || d.Files[1].Language == nil || *d.Files[1].Language != "en" {
		t.Fatalf("file attrs: %+v", d.Files)
	}
	// Local poster served.
	r := admin.req("GET", *m.PosterURL, nil)
	if r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/jpeg" || !strings.Contains(r.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("poster: %d %v", r.StatusCode, r.Header)
	}
	if r := admin.req("GET", "/api/v1/items/"+itoa(m.ID)+"/backdrop", nil); r.StatusCode != 404 {
		t.Fatalf("backdrop should 404: %d", r.StatusCode)
	}
	// Unmatched movie: scanned values, null optional fields.
	u := admin.findItem("?library="+itoa(movies), "Unknown Film")
	ud := admin.detail(u.ID)
	if ud.Overview != nil || len(ud.Genres) != 0 || ud.PosterURL != nil || u.Rating != nil {
		t.Fatalf("unknown film: %+v", ud)
	}
	raw := admin.req("GET", "/api/v1/items/"+itoa(u.ID), nil)
	for _, s := range []string{`"genres":[]`, `"seasons":[]`, `"externalIds":{}`, `"posterUrl":null`, `"tagline":null`} {
		if !strings.Contains(string(raw.body), s) {
			t.Fatalf("detail json missing %s: %s", s, raw.body)
		}
	}

	// Show.
	s := admin.findItem("?library="+itoa(shows), "Severance")
	if s.Kind != "show" || s.SeasonCount == nil || *s.SeasonCount != 3 || *s.EpisodeCount != 4 {
		t.Fatalf("show summary: %+v", s)
	}
	sd := admin.detail(s.ID)
	if len(sd.Seasons) != 3 || sd.Seasons[0].Season != 0 || sd.Seasons[1].Season != 1 || sd.Seasons[2].Season != 2 {
		t.Fatalf("seasons: %+v", sd.Seasons)
	}
	s1 := sd.Seasons[1]
	if s1.EpisodeCount != 2 || s1.Episodes[0].Episode != 1 || s1.Episodes[1].Episode != 2 {
		t.Fatalf("season 1: %+v", s1)
	}
	ep1 := s1.Episodes[0]
	if ep1.Title == nil || *ep1.Title != "Good News About Hell" || ep1.AirDate == nil || len(ep1.Subtitles) != 1 || ep1.Subtitles[0].Language == nil {
		t.Fatalf("episode 1: %+v", ep1)
	}
	if s1.Episodes[1].EpisodeEnd == nil || *s1.Episodes[1].EpisodeEnd != 3 || s1.Episodes[1].Title != nil {
		t.Fatalf("multi-episode: %+v", s1.Episodes[1])
	}
	if s1.Size != int64(len("s1e1")+len("sub1")+len("s1e2e3")) {
		t.Fatalf("season size: %d", s1.Size)
	}
	if len(sd.Files) != 0 {
		t.Fatalf("show files should be empty (fanart is artwork): %+v", sd.Files)
	}
	if sd.BackdropURL == nil {
		t.Fatal("show backdrop missing")
	}
	if r := admin.req("GET", *sd.BackdropURL, nil); r.StatusCode != 200 {
		t.Fatalf("backdrop: %d", r.StatusCode)
	}

	// Book.
	b := admin.findItem("?library="+itoa(books), "Dune")
	if b.Kind != "book" || b.Author == nil || *b.Author != "Frank Herbert" || b.PosterURL == nil || b.Year == nil || *b.Year != 1965 {
		t.Fatalf("book: %+v", b)
	}
	bd := admin.detail(b.ID)
	if len(bd.Files) != 1 || bd.Files[0].Role != "ebook" || bd.Files[0].Container != "epub" {
		t.Fatalf("book files: %+v", bd.Files)
	}

	// Search / filter / sort / genres.
	if p := admin.items("?q=matr"); p.Total != 1 || p.Items[0].Title != "The Matrix" {
		t.Fatalf("search: %+v", p)
	}
	if p := admin.items("?q=herbert"); p.Total != 1 {
		t.Fatalf("author search: %+v", p)
	}
	if p := admin.items("?genre=Science%20Fiction&sort=title"); p.Total != 2 || p.Items[0].Title != "Dune" {
		t.Fatalf("genre filter: %+v", p)
	}
	if p := admin.items("?kind=show"); p.Total != 1 {
		t.Fatalf("kind filter: %+v", p)
	}
	if p := admin.items("?limit=2&page=2&sort=title&order=asc"); p.Total != 5 || len(p.Items) != 2 || p.Page != 2 || p.Limit != 2 {
		t.Fatalf("paging: %+v", p)
	}
	all := admin.items("?sort=title")
	var titles []string
	for _, it := range all.Items {
		titles = append(titles, it.Title)
	}
	if !slices.Equal(titles, []string{"Amélie", "Dune", "The Matrix", "Severance", "Unknown Film"}) {
		t.Fatalf("title sort (article stripped): %v", titles)
	}
	if r := admin.req("GET", "/api/v1/items?sort=bogus", nil); r.StatusCode != 400 {
		t.Fatalf("bad sort: %d", r.StatusCode)
	}
	if p := admin.items("?sort=rating"); p.Items[0].Title != "The Matrix" {
		t.Fatalf("rating sort: %v", p.Items[0].Title)
	}
	var g struct{ Genres []string }
	admin.must("GET", "/api/v1/genres", nil, 200, &g)
	if !slices.Equal(g.Genres, []string{"Action", "Drama", "Science Fiction"}) {
		t.Fatalf("genres: %v", g.Genres)
	}
	admin.must("GET", "/api/v1/genres?library="+itoa(shows), nil, 200, &g)
	if !slices.Equal(g.Genres, []string{"Drama"}) {
		t.Fatalf("genres by lib: %v", g.Genres)
	}

	// Home.
	var home struct {
		RecentlyAdded []summary `json:"recentlyAdded"`
		Libraries     []struct {
			Library libraryJSON `json:"library"`
			Recent  []summary   `json:"recent"`
		} `json:"libraries"`
	}
	admin.must("GET", "/api/v1/home", nil, 200, &home)
	if len(home.RecentlyAdded) != 5 || len(home.Libraries) != 3 || home.Libraries[0].Library.Name != "Books" {
		t.Fatalf("home: %+v", home)
	}
}

func TestRescanUpsertStability(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	root := buildShows(t)
	writeFile(t, root, "Other Show/Other.S01E01.mkv", "x")
	e.prov.set("Severance", &media.Metadata{Title: "Severance", Overview: "Remote overview", Genres: []string{"Drama"}})
	lib := admin.createLibrary("TV", "tv", root)

	before := admin.findItem("", "Severance")
	other := admin.findItem("", "Other Show")
	bd := admin.detail(before.ID)
	fileIDs := map[string]int64{}
	for _, s := range bd.Seasons {
		for _, ep := range s.Episodes {
			fileIDs[ep.File.RelPath] = ep.File.ID
		}
	}
	if e.prov.callCount("Other Show") != 1 || e.prov.callCount("Severance") != 1 {
		t.Fatal("expected one lookup each")
	}

	// Change disk: remove an episode, change a file, remove a show, add a show.
	os.Remove(filepath.Join(root, "Severance/Season 02/Severance.S02E01.mkv"))
	writeFile(t, root, "Severance/Season 01/Severance.S01E01.mkv", "s1e1-but-longer")
	os.RemoveAll(filepath.Join(root, "Other Show"))
	writeFile(t, root, "New Show/New.S01E01.mkv", "new")
	time.Sleep(1100 * time.Millisecond) // addedAt has second resolution
	admin.rescan(lib)

	after := admin.findItem("", "Severance")
	if after.ID != before.ID || after.AddedAt != before.AddedAt {
		t.Fatalf("item identity changed: %+v vs %+v", before, after)
	}
	ad := admin.detail(after.ID)
	if ad.Overview == nil || *ad.Overview != "Remote overview" {
		t.Fatal("remote metadata lost on rescan")
	}
	if len(ad.Seasons) != 2 { // S00 and S01 remain
		t.Fatalf("seasons after rescan: %+v", ad.Seasons)
	}
	for _, s := range ad.Seasons {
		for _, ep := range s.Episodes {
			if id, ok := fileIDs[ep.File.RelPath]; !ok || id != ep.File.ID {
				t.Fatalf("file id changed for %s", ep.File.RelPath)
			}
			if ep.File.RelPath == "Season 01/Severance.S01E01.mkv" && ep.File.Size != int64(len("s1e1-but-longer")) {
				t.Fatal("changed file not updated")
			}
		}
	}
	if r := admin.req("GET", "/api/v1/items/"+itoa(other.ID), nil); r.StatusCode != 404 {
		t.Fatalf("removed item still present: %d", r.StatusCode)
	}
	newShow := admin.findItem("", "New Show")
	if newShow.AddedAt <= before.AddedAt {
		t.Fatalf("new item addedAt should be newer: %s vs %s", newShow.AddedAt, before.AddedAt)
	}
	// Not re-queried: matched stays matched; not-found remembered.
	admin.rescan(lib)
	if e.prov.callCount("Severance") != 1 || e.prov.callCount("New Show") != 1 {
		t.Fatalf("provider re-queried: sev=%d new=%d", e.prov.callCount("Severance"), e.prov.callCount("New Show"))
	}
	// Explicit refresh retries.
	e.prov.set("New Show", &media.Metadata{Title: "New Show (Remote)", Rating: 7})
	admin.must("POST", "/api/v1/items/"+itoa(newShow.ID)+"/refresh", nil, 202, nil)
	deadline := time.Now().Add(5 * time.Second)
	for {
		d := admin.detail(newShow.ID)
		if d.Title == "New Show (Remote)" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not apply")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Empty folder (unmounted share) must not wipe the index.
	entries, _ := os.ReadDir(root)
	for _, en := range entries {
		os.RemoveAll(filepath.Join(root, en.Name()))
	}
	l := admin.rescan(lib)
	if l.Scan.State != "error" || l.Scan.Error == nil || l.ItemCount != 2 {
		t.Fatalf("empty folder: %+v", l.Scan)
	}
}

func TestLibraryAccessFiltering(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	movies := admin.createLibrary("Movies", "movies", buildMovies(t))
	books := admin.createLibrary("Books", "books", buildBooks(t))
	user, uid := e.userClient(admin, "kid", map[string]any{"libraryIds": []int64{books}})

	var me userResp
	user.must("GET", "/api/v1/auth/me", nil, 200, &me)
	if !slices.Equal(me.User.LibraryIDs, []int64{books}) {
		t.Fatalf("libraryIds: %v", me.User.LibraryIDs)
	}
	var ls struct {
		Libraries []libraryJSON `json:"libraries"`
	}
	user.must("GET", "/api/v1/libraries", nil, 200, &ls)
	if len(ls.Libraries) != 1 || ls.Libraries[0].ID != books || ls.Libraries[0].Path != nil {
		t.Fatalf("user libraries: %+v", ls.Libraries)
	}
	if p := user.items(""); p.Total != 1 {
		t.Fatalf("user items: %+v", p)
	}
	matrix := admin.findItem("", "The Matrix")
	ad := admin.detail(matrix.ID)
	for _, p := range []string{
		"/api/v1/items/" + itoa(matrix.ID),
		"/api/v1/items/" + itoa(matrix.ID) + "/poster",
		"/api/v1/items/" + itoa(matrix.ID) + "/download",
		"/api/v1/files/" + itoa(ad.Files[0].ID) + "/download",
		"/api/v1/files/" + itoa(ad.Files[0].ID) + "/stream",
		"/api/v1/items?library=" + itoa(movies),
		"/api/v1/genres?library=" + itoa(movies),
	} {
		if r := user.req("GET", p, nil); r.StatusCode != 404 || r.errCode(t) != "not_found" {
			t.Fatalf("GET %s as restricted user: %d", p, r.StatusCode)
		}
	}
	var home struct {
		RecentlyAdded []summary        `json:"recentlyAdded"`
		Libraries     []map[string]any `json:"libraries"`
	}
	user.must("GET", "/api/v1/home", nil, 200, &home)
	if len(home.RecentlyAdded) != 1 || len(home.Libraries) != 1 {
		t.Fatalf("home: %+v", home)
	}
	// Empty list = nothing.
	admin.must("PATCH", "/api/v1/users/"+itoa(uid), map[string]any{"libraryIds": []int64{}}, 200, nil)
	if p := user.items(""); p.Total != 0 {
		t.Fatalf("empty access: %+v", p)
	}
	// null = all.
	var u userResp
	admin.must("PATCH", "/api/v1/users/"+itoa(uid), map[string]any{"libraryIds": nil}, 200, &u)
	if u.User.LibraryIDs != nil {
		t.Fatal("libraryIds should be null")
	}
	if p := user.items(""); p.Total != 4 {
		t.Fatalf("all access: %+v", p)
	}
	// Unknown library id rejected.
	if r := admin.req("PATCH", "/api/v1/users/"+itoa(uid), map[string]any{"libraryIds": []int64{999}}); r.StatusCode != 400 {
		t.Fatalf("unknown lib: %d", r.StatusCode)
	}
	// Deleting a library removes it from users' lists and never touches disk.
	admin.must("PATCH", "/api/v1/users/"+itoa(uid), map[string]any{"libraryIds": []int64{movies, books}}, 200, nil)
	lib := admin.library(movies)
	admin.must("DELETE", "/api/v1/libraries/"+itoa(movies), nil, 204, nil)
	if _, err := os.Stat(filepath.Join(*lib.Path, "The Matrix (1999)/The.Matrix.1999.1080p.mkv")); err != nil {
		t.Fatal("library delete touched files on disk")
	}
	user.must("GET", "/api/v1/auth/me", nil, 200, &me)
	if !slices.Equal(me.User.LibraryIDs, []int64{books}) {
		t.Fatalf("libraryIds after delete: %v", me.User.LibraryIDs)
	}
}

func TestFileDownloadRange(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	admin.createLibrary("Movies", "movies", buildMovies(t))
	m := admin.findItem("", "Amélie")
	d := admin.detail(m.ID)
	f := d.Files[0]

	r := admin.req("GET", "/api/v1/files/"+itoa(f.ID)+"/download", nil)
	if r.StatusCode != 200 || string(r.body) != "amelie-bytes" {
		t.Fatalf("download: %d %q", r.StatusCode, r.body)
	}
	cd := r.Header.Get("Content-Disposition")
	if !strings.HasPrefix(cd, `attachment; filename="Am_lie (2001).mkv"`) || !strings.Contains(cd, `filename*=UTF-8''Am%C3%A9lie%20%282001%29.mkv`) {
		t.Fatalf("content-disposition: %s", cd)
	}
	if r.Header.Get("Content-Type") != "video/x-matroska" || r.Header.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("headers: %v", r.Header)
	}

	r = admin.req("GET", "/api/v1/files/"+itoa(f.ID)+"/download", nil, "Range", "bytes=2-5")
	if r.StatusCode != 206 || string(r.body) != "elie" || r.Header.Get("Content-Range") != "bytes 2-5/12" {
		t.Fatalf("range: %d %q %s", r.StatusCode, r.body, r.Header.Get("Content-Range"))
	}
	r = admin.req("GET", "/api/v1/files/"+itoa(f.ID)+"/stream", nil, "Range", "bytes=-3")
	if r.StatusCode != 206 || string(r.body) != "tes" || !strings.HasPrefix(r.Header.Get("Content-Disposition"), "inline") {
		t.Fatalf("stream: %d %q %v", r.StatusCode, r.body, r.Header)
	}

	// Activity: one download_file entry despite the range request; none for stream.
	var act struct {
		Items []activityJSON `json:"items"`
	}
	admin.must("GET", "/api/v1/admin/activity", nil, 200, &act)
	n := 0
	for _, a := range act.Items {
		if a.Action == "download_file" {
			n++
			if a.ItemID == nil || *a.ItemID != m.ID || a.Size == nil || *a.Size != 12 || a.Detail == nil || *a.Detail != "Amélie (2001).mkv" {
				t.Fatalf("activity row: %+v", a)
			}
		}
	}
	if n != 1 {
		t.Fatalf("download_file activity rows: %d", n)
	}

	// canDownload=false.
	viewer, _ := e.userClient(admin, "viewer", map[string]any{"canDownload": false})
	viewer.must("GET", "/api/v1/items/"+itoa(m.ID), nil, 200, nil)
	for _, p := range []string{"/files/" + itoa(f.ID) + "/download", "/files/" + itoa(f.ID) + "/stream", "/items/" + itoa(m.ID) + "/download"} {
		if r := viewer.req("GET", "/api/v1"+p, nil); r.StatusCode != 403 || r.errCode(t) != "forbidden" {
			t.Fatalf("%s for viewer: %d", p, r.StatusCode)
		}
	}
	if r := admin.req("GET", "/api/v1/files/999999/download", nil); r.StatusCode != 404 {
		t.Fatalf("missing file: %d", r.StatusCode)
	}
}

func TestZipDownload(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	admin.createLibrary("TV", "tv", buildShows(t))
	admin.createLibrary("Movies", "movies", buildMovies(t))

	readZip := func(path string) map[string]string {
		t.Helper()
		r := admin.req("GET", path, nil)
		if r.StatusCode != 200 || r.Header.Get("Content-Type") != "application/zip" {
			t.Fatalf("zip %s: %d %s", path, r.StatusCode, r.body)
		}
		zr, err := zip.NewReader(bytes.NewReader(r.body), int64(len(r.body)))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, f := range zr.File {
			if f.Method != zip.Store {
				t.Fatalf("%s not stored", f.Name)
			}
			rc, _ := f.Open()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read %s: %v", f.Name, err)
			}
			out[f.Name] = string(b)
		}
		return out
	}
	keys := func(m map[string]string) []string {
		var k []string
		for x := range m {
			k = append(k, x)
		}
		sort.Strings(k)
		return k
	}

	m := admin.findItem("?kind=movie", "The Matrix")
	got := readZip("/api/v1/items/" + itoa(m.ID) + "/download")
	want := []string{"The Matrix (1999)/Extras/Trailer.mp4", "The Matrix (1999)/The.Matrix.1999.1080p.mkv", "The Matrix (1999)/The.Matrix.1999.en.srt"}
	if !slices.Equal(keys(got), want) || got["The Matrix (1999)/The.Matrix.1999.1080p.mkv"] != "matrix-video-bytes" {
		t.Fatalf("matrix zip: %v", keys(got))
	}

	s := admin.findItem("?kind=show", "Severance")
	got = readZip("/api/v1/items/" + itoa(s.ID) + "/download?season=1")
	want = []string{"Severance/Season 01/Severance.S01E01.en.srt", "Severance/Season 01/Severance.S01E01.mkv", "Severance/Season 01/Severance.S01E02-E03.mkv"}
	if !slices.Equal(keys(got), want) {
		t.Fatalf("season zip: %v", keys(got))
	}
	r := admin.req("GET", "/api/v1/items/"+itoa(s.ID)+"/download?season=1", nil)
	if !strings.Contains(r.Header.Get("Content-Disposition"), `Severance - Season 1.zip`) {
		t.Fatalf("zip name: %s", r.Header.Get("Content-Disposition"))
	}
	got = readZip("/api/v1/items/" + itoa(s.ID) + "/download")
	if len(got) != 5 {
		t.Fatalf("full show zip: %v", keys(got))
	}
	if r := admin.req("GET", "/api/v1/items/"+itoa(s.ID)+"/download?season=7", nil); r.StatusCode != 404 {
		t.Fatalf("empty season: %d", r.StatusCode)
	}
	if r := admin.req("GET", "/api/v1/items/"+itoa(s.ID)+"/download?season=x", nil); r.StatusCode != 400 {
		t.Fatalf("bad season: %d", r.StatusCode)
	}
}

func TestPathEscapeSafety(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	outside := t.TempDir()
	secret := writeFile(t, outside, "secret.mkv", "TOP-SECRET")
	writeFile(t, outside, "evil.jpg", "\xff\xd8\xff\xe0evil")
	root := t.TempDir()
	writeFile(t, root, "Legit (2000)/legit.mkv", "ok")
	if err := os.Symlink(secret, filepath.Join(root, "Legit (2000)", "escape.mkv")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	os.Symlink(filepath.Join(outside, "evil.jpg"), filepath.Join(root, "Legit (2000)", "poster.jpg"))
	os.Symlink(outside, filepath.Join(root, "Linked Dir (2001)"))
	// A symlink that stays inside the library is fine.
	os.Symlink("legit.mkv", filepath.Join(root, "Legit (2000)", "inside.mkv"))
	admin.createLibrary("Movies", "movies", root)

	it := admin.findItem("", "Legit")
	d := admin.detail(it.ID)
	if it.PosterURL != nil {
		t.Fatalf("poster pointing outside must not be exposed: %v", *it.PosterURL)
	}
	if r := admin.req("GET", "/api/v1/items/"+itoa(it.ID)+"/poster", nil); r.StatusCode != 404 {
		t.Fatalf("escaping poster served: %d", r.StatusCode)
	}
	sawEscape := false
	for _, f := range d.Files {
		r := admin.req("GET", "/api/v1/files/"+itoa(f.ID)+"/download", nil)
		switch f.Name {
		case "escape.mkv":
			sawEscape = true
			if r.StatusCode != 404 || strings.Contains(string(r.body), "TOP-SECRET") {
				t.Fatalf("symlink escape served: %d %q", r.StatusCode, r.body)
			}
		case "inside.mkv", "legit.mkv":
			if r.StatusCode != 200 || string(r.body) != "ok" {
				t.Fatalf("%s: %d", f.Name, r.StatusCode)
			}
		}
	}
	if !sawEscape {
		t.Fatal("fake scanner should have indexed escape.mkv")
	}
	zr := admin.req("GET", "/api/v1/items/"+itoa(it.ID)+"/download", nil)
	if strings.Contains(string(zr.body), "TOP-SECRET") {
		t.Fatal("zip leaked file outside the library")
	}
	// The symlinked directory item: its files resolve outside the root.
	if p := admin.items("?q=linked"); p.Total == 1 {
		ld := admin.detail(p.Items[0].ID)
		for _, f := range ld.Files {
			if r := admin.req("GET", "/api/v1/files/"+itoa(f.ID)+"/download", nil); r.StatusCode != 404 {
				t.Fatalf("file under symlinked dir served: %d", r.StatusCode)
			}
		}
	}
	// Direct unit check of the resolver.
	for _, rel := range []string{"../x", "/etc/passwd", "a/../../x", "", "Legit (2000)/escape.mkv"} {
		if _, err := resolveInRoot(root, rel); err == nil {
			t.Fatalf("resolveInRoot(%q) should fail", rel)
		}
	}
}

func TestNilScannerGracefulError(t *testing.T) {
	e := newEnv(t, withoutScanner())
	admin := e.admin()
	dir := t.TempDir()
	if r := admin.req("POST", "/api/v1/libraries", map[string]string{"name": "X", "type": "movies", "path": filepath.Join(dir, "missing")}); r.StatusCode != 400 {
		t.Fatalf("missing dir: %d", r.StatusCode)
	}
	if r := admin.req("POST", "/api/v1/libraries", map[string]string{"name": "X", "type": "films", "path": dir}); r.StatusCode != 400 {
		t.Fatalf("bad type: %d", r.StatusCode)
	}
	id := admin.createLibrary("Movies", "movies", dir)
	l := admin.library(id)
	if l.Scan.State != "error" || l.Scan.Error == nil || !strings.Contains(*l.Scan.Error, "scanner") || l.Scan.Phase != nil {
		t.Fatalf("scan status: %+v", l.Scan)
	}
	// PATCH and rescans also fail gracefully.
	var lr libResp
	admin.must("PATCH", "/api/v1/libraries/"+itoa(id), map[string]any{"name": "Films", "type": "tv"}, 200, &lr)
	if lr.Library.Name != "Films" || lr.Library.Type != "tv" {
		t.Fatalf("patch: %+v", lr.Library)
	}
	if l := admin.rescan(id); l.Scan.State != "error" {
		t.Fatalf("rescan: %+v", l.Scan)
	}
}

func TestSettings(t *testing.T) {
	e := newEnv(t)
	admin := e.admin()
	var s settingsJSON
	admin.must("GET", "/api/v1/settings", nil, 200, &s)
	if s.TMDBAPIKeySet || !s.OpenLibraryEnabled || s.ScanIntervalMinutes != 60 {
		t.Fatalf("defaults: %+v", s)
	}
	var seen []Settings
	e.app.deps.Providers = func(st Settings) []media.MetadataProvider {
		seen = append(seen, st)
		return nil
	}
	r := admin.must("PUT", "/api/v1/settings", map[string]any{"tmdbApiKey": "sekrit-key", "scanIntervalMinutes": 0}, 200, &s)
	if !s.TMDBAPIKeySet || s.ScanIntervalMinutes != 0 || strings.Contains(string(r.body), "sekrit") {
		t.Fatalf("after put: %s", r.body)
	}
	if r := admin.req("GET", "/api/v1/settings", nil); strings.Contains(string(r.body), "sekrit") {
		t.Fatal("key leaked")
	}
	e.app.metadataProviders()
	if len(seen) != 1 || seen[0].TMDBAPIKey != "sekrit-key" {
		t.Fatalf("provider factory got %+v", seen)
	}
	e.app.metadataProviders()
	if len(seen) != 1 {
		t.Fatal("providers rebuilt without settings change")
	}
	admin.must("PUT", "/api/v1/settings", map[string]any{"tmdbApiKey": ""}, 200, &s)
	if s.TMDBAPIKeySet {
		t.Fatal("key not cleared")
	}
	if r := admin.req("PUT", "/api/v1/settings", map[string]any{"scanIntervalMinutes": -1}); r.StatusCode != 400 {
		t.Fatalf("negative interval: %d", r.StatusCode)
	}
	// Persisted across restarts of the settings cache.
	e.app.loadSettings(t.Context())
	if e.app.currentSettings().ScanIntervalMinutes != 0 {
		t.Fatal("not persisted")
	}
}

func TestRemoteArtworkCached(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	hits := 0
	img := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/svg" {
			w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
			return
		}
		w.Write(png)
	}))
	defer img.Close()
	e := newEnv(t)
	e.prov.set("Unknown Film", &media.Metadata{Title: "Unknown Film", PosterURL: img.URL + "/p.png", BackdropURL: img.URL + "/svg"})
	admin := e.admin()
	admin.createLibrary("Movies", "movies", buildMovies(t))
	it := admin.findItem("", "Unknown Film")
	if it.PosterURL == nil {
		t.Fatal("no posterUrl")
	}
	for i := 0; i < 3; i++ {
		r := admin.req("GET", *it.PosterURL, nil)
		if r.StatusCode != 200 || !bytes.Equal(r.body, png) || r.Header.Get("Content-Type") != "image/png" {
			t.Fatalf("poster: %d %v", r.StatusCode, r.Header)
		}
	}
	if hits != 1 {
		t.Fatalf("remote fetched %d times", hits)
	}
	d := admin.detail(it.ID)
	if r := admin.req("GET", *d.BackdropURL, nil); r.StatusCode != 404 {
		t.Fatalf("svg backdrop must be refused: %d", r.StatusCode)
	}
}

func TestStaticSPA(t *testing.T) {
	e := newEnv(t, func(d *Deps) {
		d.WebFS = fstest.MapFS{
			"index.html":        {Data: []byte("<html>spa</html>")},
			"assets/app-abc.js": {Data: []byte("console.log(1)")},
			"favicon.svg":       {Data: []byte("<svg/>")},
		}
	})
	c := e.client()
	r := c.req("GET", "/library/5", nil)
	if r.StatusCode != 200 || string(r.body) != "<html>spa</html>" || r.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("spa fallback: %d %q %v", r.StatusCode, r.body, r.Header)
	}
	if !strings.Contains(r.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") || r.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers missing")
	}
	r = c.req("GET", "/assets/app-abc.js", nil)
	if r.StatusCode != 200 || !strings.Contains(r.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %d %v", r.StatusCode, r.Header)
	}
	if r := c.req("GET", "/assets/missing.js", nil); r.StatusCode != 404 {
		t.Fatalf("missing asset: %d", r.StatusCode)
	}
	if r := c.req("GET", "/api/v1/nope", nil); r.StatusCode != 404 || r.errCode(t) != "not_found" {
		t.Fatalf("api 404: %d", r.StatusCode)
	}
	// Nil WebFS: placeholder.
	e2 := newEnv(t)
	if r := e2.client().req("GET", "/", nil); r.StatusCode != 200 || !strings.Contains(string(r.body), "Trove") {
		t.Fatalf("placeholder: %d", r.StatusCode)
	}
}

func TestBootstrapAdmin(t *testing.T) {
	dir := t.TempDir()
	a, err := New(Config{DataDir: dir, AdminUser: "root", AdminPassword: "rootpassword", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()
	defer a.Close()
	e := &testEnv{t: t, app: a, srv: srv}
	c := e.client()
	var u userResp
	c.must("POST", "/api/v1/auth/login", map[string]string{"username": "root", "password": "rootpassword"}, 200, &u)
	if u.User.Role != "admin" {
		t.Fatal("bootstrap user is not admin")
	}
}
