// Package metadata implements media.MetadataProvider for TMDB (movies, shows)
// and Open Library (books, audiobooks) using only the standard library.
package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"trove/internal/media"
)

// UserAgent is sent with every request. It deliberately carries no contact
// details.
const UserAgent = "Trove/0.1 (self-hosted media library)"

// Error classes. Errors returned by providers wrap one of these (or
// media.ErrNotFound) so callers can use errors.Is.
var (
	ErrAuth        = errors.New("metadata: authentication failed")
	ErrRateLimited = errors.New("metadata: rate limited")
	ErrServer      = errors.New("metadata: server error")
	ErrUnsupported = errors.New("metadata: unsupported item kind")
)

// HTTPError describes a failed provider request.
type HTTPError struct {
	Provider   string
	StatusCode int
	Path       string // request path, without query (never contains credentials)
	Message    string // provider-supplied message, if any
	kind       error
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("%s: %s %s: HTTP %d", e.Provider, http.MethodGet, e.Path, e.StatusCode)
	switch {
	case errors.Is(e.kind, ErrAuth):
		msg += " (authentication failed: check the API key)"
	case errors.Is(e.kind, ErrRateLimited):
		msg += " (rate limited)"
	case errors.Is(e.kind, ErrServer):
		msg += " (server error)"
	case errors.Is(e.kind, media.ErrNotFound):
		msg += " (not found)"
	}
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

func (e *HTTPError) Unwrap() error { return e.kind }

// Option configures a provider.
type Option func(*config)

type config struct {
	client       *http.Client
	baseURL      string
	interval     time.Duration
	maxRetries   int
	maxRetryWait time.Duration
	language     string
}

// WithHTTPClient sets the HTTP client (nil keeps the default, which has a
// 15s timeout).
func WithHTTPClient(c *http.Client) Option { return func(o *config) { o.client = c } }

// WithBaseURL overrides the API base URL (for tests / proxies).
func WithBaseURL(u string) Option {
	return func(o *config) { o.baseURL = strings.TrimRight(u, "/") }
}

// WithRateLimit sets the minimum spacing between requests; 0 disables the
// limiter. Defaults: TMDB 250ms (4 req/s), Open Library 1s.
func WithRateLimit(interval time.Duration) Option { return func(o *config) { o.interval = interval } }

// WithMaxRetries sets how often a 429 / 502-504 response is retried
// (default 2).
func WithMaxRetries(n int) Option { return func(o *config) { o.maxRetries = n } }

// WithMaxRetryWait caps how long a single Retry-After wait may be (default
// 10s). A server asking for longer makes the request fail with
// ErrRateLimited instead of blocking.
func WithMaxRetryWait(d time.Duration) Option { return func(o *config) { o.maxRetryWait = d } }

// WithLanguage sets the TMDB response language (default "en-US").
func WithLanguage(lang string) Option { return func(o *config) { o.language = lang } }

func newConfig(base string, interval time.Duration, opts []Option) config {
	c := config{baseURL: base, interval: interval, maxRetries: 2, maxRetryWait: 10 * time.Second, language: "en-US"}
	for _, o := range opts {
		o(&c)
	}
	if c.client == nil {
		c.client = &http.Client{Timeout: 15 * time.Second}
	}
	return c
}

// limiter spaces requests at least interval apart. Safe for concurrent use.
type limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func (l *limiter) wait(ctx context.Context) error {
	if l.interval <= 0 {
		return ctx.Err()
	}
	l.mu.Lock()
	now := time.Now()
	slot := l.next
	if slot.Before(now) {
		slot = now
	}
	l.next = slot.Add(l.interval)
	l.mu.Unlock()
	return sleep(ctx, time.Until(slot))
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type httpClient struct {
	provider string
	cfg      config
	lim      *limiter
	auth     func(req *http.Request, q url.Values)
}

func newHTTPClient(provider string, cfg config) *httpClient {
	return &httpClient{provider: provider, cfg: cfg, lim: &limiter{interval: cfg.interval}}
}

const maxBody = 8 << 20

// getJSON performs a GET request and decodes a JSON response into out.
func (c *httpClient) getJSON(ctx context.Context, path string, q url.Values, out any) error {
	if q == nil {
		q = url.Values{}
	}
	for attempt := 0; ; attempt++ {
		if err := c.lim.wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.baseURL+path, nil)
		if err != nil {
			return fmt.Errorf("%s: %w", c.provider, err)
		}
		qq := url.Values{}
		for k, v := range q {
			qq[k] = v
		}
		if c.auth != nil {
			c.auth(req, qq)
		}
		req.URL.RawQuery = qq.Encode()
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", UserAgent)

		resp, err := c.cfg.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%s: GET %s: %w", c.provider, path, scrubURLError(err))
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		resp.Body.Close()
		if readErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%s: GET %s: reading response: %w", c.provider, path, readErr)
		}

		code := resp.StatusCode
		switch {
		case code >= 200 && code < 300:
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("%s: GET %s: decoding response: %w", c.provider, path, err)
			}
			return nil
		case code == http.StatusTooManyRequests || code == http.StatusBadGateway || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout:
			kind := ErrServer
			if code == http.StatusTooManyRequests {
				kind = ErrRateLimited
			}
			wait, ok := retryAfter(resp.Header.Get("Retry-After"))
			if !ok {
				wait = time.Duration(attempt+1) * 500 * time.Millisecond
			}
			if attempt < c.cfg.maxRetries && wait <= c.cfg.maxRetryWait {
				if err := sleep(ctx, wait); err != nil {
					return err
				}
				continue
			}
			return &HTTPError{Provider: c.provider, StatusCode: code, Path: path, Message: providerMessage(body), kind: kind}
		case code == http.StatusUnauthorized || code == http.StatusForbidden:
			return &HTTPError{Provider: c.provider, StatusCode: code, Path: path, Message: providerMessage(body), kind: ErrAuth}
		case code == http.StatusNotFound:
			return &HTTPError{Provider: c.provider, StatusCode: code, Path: path, Message: providerMessage(body), kind: media.ErrNotFound}
		case code >= 500:
			return &HTTPError{Provider: c.provider, StatusCode: code, Path: path, Message: providerMessage(body), kind: ErrServer}
		default:
			return &HTTPError{Provider: c.provider, StatusCode: code, Path: path, Message: providerMessage(body)}
		}
	}
}

// retryAfter parses a Retry-After header (seconds or HTTP date).
func retryAfter(h string) (time.Duration, bool) {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0, false
	}
	if s, err := strconv.Atoi(h); err == nil && s >= 0 {
		return time.Duration(s) * time.Second, true
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// providerMessage extracts a human message from an error body.
func providerMessage(body []byte) string {
	var m struct {
		StatusMessage string `json:"status_message"` // TMDB
		Error         string `json:"error"`
		Message       string `json:"message"`
	}
	if json.Unmarshal(body, &m) == nil {
		for _, s := range []string{m.StatusMessage, m.Error, m.Message} {
			if s != "" {
				return s
			}
		}
	}
	s := strings.TrimSpace(string(body))
	if len(s) > 200 || strings.HasPrefix(s, "<") {
		return ""
	}
	return s
}

// scrubURLError drops the query string (which may hold an API key) from a
// *url.Error.
func scrubURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		u := ue.URL
		if i := strings.IndexByte(u, '?'); i >= 0 {
			u = u[:i]
		}
		return &url.Error{Op: ue.Op, URL: u, Err: ue.Err}
	}
	return err
}

func notFound(provider, format string, args ...any) error {
	return fmt.Errorf("%s: %w: %s", provider, media.ErrNotFound, fmt.Sprintf(format, args...))
}
