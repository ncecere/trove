package app

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"trove/internal/media"
)

func TestMain(m *testing.M) {
	bcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}

type testEnv struct {
	t       *testing.T
	app     *App
	srv     *httptest.Server
	scanner *fakeScanner
	prov    *fakeProvider
}

type envOpt func(*Deps)

func withoutScanner() envOpt { return func(d *Deps) { d.Scanner = nil } }

func newEnv(t *testing.T, opts ...envOpt) *testEnv {
	t.Helper()
	e := &testEnv{t: t, scanner: &fakeScanner{}, prov: newFakeProvider("fake", media.KindMovie, media.KindShow, media.KindBook)}
	deps := Deps{
		Scanner:   e.scanner,
		Providers: func(Settings) []media.MetadataProvider { return []media.MetadataProvider{e.prov} },
	}
	for _, o := range opts {
		o(&deps)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if testing.Verbose() {
		log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	a, err := New(Config{DataDir: t.TempDir(), Logger: log}, deps)
	if err != nil {
		t.Fatal(err)
	}
	e.app = a
	e.srv = httptest.NewServer(a.Handler())
	t.Cleanup(func() {
		e.srv.Close()
		a.Close()
	})
	return e
}

type client struct {
	e  *testEnv
	hc *http.Client
}

func (e *testEnv) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, hc: &http.Client{Jar: jar}}
}

type resp struct {
	*http.Response
	body []byte
}

func (r *resp) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
}

func (r *resp) errCode(t *testing.T) string {
	t.Helper()
	var b errorBody
	r.decode(t, &b)
	return b.Error.Code
}

func (c *client) req(method, path string, body any, hdr ...string) *resp {
	c.e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, rd)
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("X-Requested-With", "trove")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		if hdr[i+1] == "" {
			req.Header.Del(hdr[i])
		} else {
			req.Header.Set(hdr[i], hdr[i+1])
		}
	}
	r, err := c.hc.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return &resp{r, b}
}

// must performs a request and checks the status, decoding into out.
func (c *client) must(method, path string, body any, want int, out any) *resp {
	c.e.t.Helper()
	r := c.req(method, path, body)
	if r.StatusCode != want {
		c.e.t.Fatalf("%s %s: status %d, want %d: %s", method, path, r.StatusCode, want, r.body)
	}
	if out != nil {
		r.decode(c.e.t, out)
	}
	return r
}

type userResp struct {
	User struct {
		ID          int64   `json:"id"`
		Username    string  `json:"username"`
		Role        string  `json:"role"`
		CanDownload bool    `json:"canDownload"`
		LibraryIDs  []int64 `json:"libraryIds"`
		LastLoginAt *string `json:"lastLoginAt"`
	} `json:"user"`
}

type libResp struct {
	Library libraryJSON `json:"library"`
}

// admin runs setup and returns a logged-in admin client.
func (e *testEnv) admin() *client {
	c := e.client()
	c.must("POST", "/api/v1/setup", map[string]string{"username": "admin", "password": "supersecret"}, 200, nil)
	return c
}

func (e *testEnv) userClient(admin *client, name string, extra map[string]any) (*client, int64) {
	body := map[string]any{"username": name, "password": "password123", "role": "user", "canDownload": true, "libraryIds": nil}
	for k, v := range extra {
		body[k] = v
	}
	var u userResp
	admin.must("POST", "/api/v1/users", body, 201, &u)
	c := e.client()
	c.must("POST", "/api/v1/auth/login", map[string]string{"username": name, "password": "password123"}, 200, nil)
	return c, u.User.ID
}

func (c *client) createLibrary(name, typ, path string) int64 {
	c.e.t.Helper()
	var l libResp
	c.must("POST", "/api/v1/libraries", map[string]string{"name": name, "type": typ, "path": path}, 201, &l)
	c.waitScan(l.Library.ID)
	return l.Library.ID
}

func (c *client) library(id int64) libraryJSON {
	c.e.t.Helper()
	var ls struct {
		Libraries []libraryJSON `json:"libraries"`
	}
	c.must("GET", "/api/v1/libraries", nil, 200, &ls)
	for _, l := range ls.Libraries {
		if l.ID == id {
			return l
		}
	}
	c.e.t.Fatalf("library %d not listed", id)
	return libraryJSON{}
}

func (c *client) waitScan(id int64) libraryJSON {
	c.e.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		l := c.library(id)
		if l.Scan.State != "scanning" && !c.e.app.scans.busy(id) {
			return c.library(id)
		}
		time.Sleep(10 * time.Millisecond)
	}
	c.e.t.Fatalf("scan of library %d did not finish", id)
	return libraryJSON{}
}

func (c *client) rescan(id int64) libraryJSON {
	c.e.t.Helper()
	c.must("POST", "/api/v1/libraries/"+itoa(id)+"/scan", nil, 202, nil)
	return c.waitScan(id)
}

type summary struct {
	ID           int64   `json:"id"`
	LibraryID    int64   `json:"libraryId"`
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	Year         *int    `json:"year"`
	Author       *string `json:"author"`
	PosterURL    *string `json:"posterUrl"`
	Rating       *float64
	AddedAt      string `json:"addedAt"`
	Size         int64  `json:"size"`
	FileCount    int    `json:"fileCount"`
	SeasonCount  *int   `json:"seasonCount"`
	EpisodeCount *int   `json:"episodeCount"`
}

type page struct {
	Items []summary `json:"items"`
	Total int       `json:"total"`
	Page  int       `json:"page"`
	Limit int       `json:"limit"`
}

func (c *client) items(query string) page {
	c.e.t.Helper()
	var p page
	c.must("GET", "/api/v1/items"+query, nil, 200, &p)
	return p
}

func (c *client) findItem(query, title string) summary {
	c.e.t.Helper()
	for _, it := range c.items(query).Items {
		if it.Title == title {
			return it
		}
	}
	c.e.t.Fatalf("item %q not found in %s", title, query)
	return summary{}
}

type detail struct {
	summary
	Overview    *string           `json:"overview"`
	Genres      []string          `json:"genres"`
	BackdropURL *string           `json:"backdropUrl"`
	Narrator    *string           `json:"narrator"`
	ExternalIDs map[string]string `json:"externalIds"`
	Files       []mediaFileJSON   `json:"files"`
	Seasons     []seasonJSON      `json:"seasons"`
}

func (c *client) detail(id int64) detail {
	c.e.t.Helper()
	var d detail
	c.must("GET", "/api/v1/items/"+itoa(id), nil, 200, &d)
	return d
}

func itoa(n int64) string { return jsonNum(n) }

func jsonNum(n int64) string { b, _ := json.Marshal(n); return string(b) }

// writeFile creates a file (and parents) with the given content.
func writeFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
