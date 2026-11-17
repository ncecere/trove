package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"trove/internal/store"
)

const apiPrefix = "/api/v1"

const maxJSONBody = 1 << 20

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	h := func(pattern string, fn http.HandlerFunc) {
		method, path, _ := strings.Cut(pattern, " ")
		mux.Handle(method+" "+apiPrefix+path, fn)
	}

	// Setup & auth (public).
	h("GET /setup/status", a.handleSetupStatus)
	h("POST /setup", a.handleSetup)
	h("POST /auth/login", a.handleLogin)
	h("POST /auth/logout", a.handleLogout)
	h("GET /auth/me", a.authed(a.handleMe))
	h("PUT /auth/password", a.authed(a.handleChangePassword))

	// Users.
	h("GET /users", a.admin(a.handleListUsers))
	h("POST /users", a.admin(a.handleCreateUser))
	h("PATCH /users/{id}", a.admin(a.handleUpdateUser))
	h("DELETE /users/{id}", a.admin(a.handleDeleteUser))

	// Libraries.
	h("GET /libraries", a.authed(a.handleListLibraries))
	h("POST /libraries", a.admin(a.handleCreateLibrary))
	h("PATCH /libraries/{id}", a.admin(a.handleUpdateLibrary))
	h("DELETE /libraries/{id}", a.admin(a.handleDeleteLibrary))
	h("POST /libraries/{id}/scan", a.admin(a.handleScanLibrary))
	h("POST /libraries/scan", a.admin(a.handleScanAll))
	h("GET /admin/fs", a.admin(a.handleAdminFS))

	// Browsing.
	h("GET /home", a.authed(a.handleHome))
	h("GET /items", a.authed(a.handleItems))
	h("GET /items/{id}", a.authed(a.handleItem))
	h("GET /genres", a.authed(a.handleGenres))
	h("GET /items/{id}/poster", a.authed(a.handleArtwork("poster")))
	h("GET /items/{id}/backdrop", a.authed(a.handleArtwork("backdrop")))

	// Downloads.
	h("GET /files/{id}/download", a.authed(a.handleFileDownload))
	h("GET /files/{id}/stream", a.authed(a.handleFileStream))
	h("GET /items/{id}/download", a.authed(a.handleItemDownload))

	// Admin.
	h("POST /items/{id}/refresh", a.admin(a.handleRefreshItem))
	h("GET /settings", a.admin(a.handleGetSettings))
	h("PUT /settings", a.admin(a.handlePutSettings))
	h("GET /admin/activity", a.admin(a.handleActivity))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Unknown API endpoint")
	})
	mux.Handle("/", a.staticHandler())

	return a.logRequests(securityHeaders(csrfCheck(mux)))
}

// ---- responses ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	var b errorBody
	b.Error.Code, b.Error.Message = code, msg
	writeJSON(w, status, b)
}

func badRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, "invalid_request", msg)
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not_found", "Not found")
}

func forbidden(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusForbidden, "forbidden", msg)
}

func conflict(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusConflict, "conflict", msg)
}

func (a *App) internalError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
		return
	}
	a.log.Error("internal error", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "Internal server error")
}

// storeErr maps common store errors to responses.
func (a *App) storeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		notFound(w)
	case errors.Is(err, store.ErrConflict):
		conflict(w, "Already exists")
	default:
		a.internalError(w, r, err)
	}
}

// decodeJSON reads a size-limited JSON body into v.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(30 * time.Second))
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		var mbe *http.MaxBytesError
		switch {
		case errors.As(err, &mbe):
			writeError(w, http.StatusRequestEntityTooLarge, "invalid_request", "Request body too large")
		case errors.Is(err, io.EOF):
			badRequest(w, "Request body is required")
		default:
			badRequest(w, "Invalid JSON body: "+err.Error())
		}
		return false
	}
	return true
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func fmtTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := fmtTime(*t)
	return &s
}

// ---- middleware ----

func csrfCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("X-Requested-With") != "trove" {
				writeError(w, http.StatusForbidden, "csrf", "Missing or invalid X-Requested-With header")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; " +
	"media-src 'self' blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; " +
	"form-action 'self'; frame-ancestors 'none'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Referrer-Policy", "same-origin")
		hd.Set("Content-Security-Policy", csp)
		hd.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusWriter) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

// ReadFrom keeps sendfile(2) working for downloads.
func (s *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	var n int64
	var err error
	if rf, ok := s.ResponseWriter.(io.ReaderFrom); ok {
		n, err = rf.ReadFrom(r)
	} else {
		n, err = io.Copy(struct{ io.Writer }{s.ResponseWriter}, r)
	}
	s.bytes += n
	return n, err
}

func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(s.ResponseWriter).Hijack()
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (a *App) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				a.log.Error("panic in handler", "path", r.URL.Path, "panic", p)
				if sw.status == 0 {
					writeError(sw, http.StatusInternalServerError, "internal", "Internal server error")
				}
			}
			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			lvl := slog.LevelInfo
			if !strings.HasPrefix(r.URL.Path, "/api/") || strings.HasSuffix(r.URL.Path, "/poster") || strings.HasSuffix(r.URL.Path, "/backdrop") {
				lvl = slog.LevelDebug
			}
			if status >= 500 {
				lvl = slog.LevelError
			}
			a.log.Log(r.Context(), lvl, "http",
				"method", r.Method, "path", r.URL.Path, "status", status, "bytes", sw.bytes,
				"dur", time.Since(start).Round(time.Microsecond).String(), "ip", displayIP(r))
		}()
		next.ServeHTTP(sw, r)
	})
}

// ---- client info ----

// remoteIP is the TCP peer address (used for rate limiting; not spoofable).
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// displayIP honours X-Forwarded-For / X-Real-IP; only for display.
func displayIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if ip := net.ParseIP(strings.TrimSpace(first)); ip != nil {
			return ip.String()
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		if ip := net.ParseIP(xr); ip != nil {
			return ip.String()
		}
	}
	return remoteIP(r)
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	p, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(p), "https")
}
