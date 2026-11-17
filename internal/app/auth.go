package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"trove/internal/store"
)

const (
	cookieName      = "trove_session"
	sessionLifetime = 30 * 24 * time.Hour
	// Sliding expiry is only written back when it moved by at least this much.
	sessionRefresh = time.Hour
)

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxSession
)

func userFrom(r *http.Request) *store.User {
	u, _ := r.Context().Value(ctxUser).(*store.User)
	return u
}

func sessionHashFrom(r *http.Request) string {
	s, _ := r.Context().Value(ctxSession).(string)
	return s
}

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func (a *App) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
}

// startSession creates a session for u and sets the cookie.
func (a *App) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request, u *store.User) error {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	tok := base64.RawURLEncoding.EncodeToString(buf)
	exp := time.Now().Add(sessionLifetime)
	if err := a.store.CreateSession(ctx, hashToken(tok), u.ID, exp, displayIP(r), r.UserAgent()); err != nil {
		return err
	}
	a.setSessionCookie(w, r, tok, exp)
	return nil
}

// sessionUser resolves the request's session (if any).
func (a *App) sessionUser(w http.ResponseWriter, r *http.Request) (*store.User, string, error) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" || len(c.Value) > 128 {
		return nil, "", nil
	}
	h := hashToken(c.Value)
	now := time.Now()
	sess, u, err := a.store.GetSession(r.Context(), h, now)
	if errors.Is(err, store.ErrNotFound) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	// Sliding expiry.
	newExp := now.Add(sessionLifetime)
	if newExp.Sub(sess.ExpiresAt) >= sessionRefresh {
		if err := a.store.ExtendSession(r.Context(), h, newExp); err == nil {
			a.setSessionCookie(w, r, c.Value, newExp)
		}
	}
	return u, h, nil
}

func (a *App) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, h, err := a.sessionUser(w, r)
		if err != nil {
			a.internalError(w, r, err)
			return
		}
		if u == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Not logged in")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, u)
		ctx = context.WithValue(ctx, ctxSession, h)
		next(w, r.WithContext(ctx))
	}
}

func (a *App) admin(next http.HandlerFunc) http.HandlerFunc {
	return a.authed(func(w http.ResponseWriter, r *http.Request) {
		if !userFrom(r).IsAdmin() {
			forbidden(w, "Admin only")
			return
		}
		next(w, r)
	})
}

// ---- JSON ----

type userJSON struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Role        string  `json:"role"`
	CanDownload bool    `json:"canDownload"`
	LibraryIDs  []int64 `json:"libraryIds"`
	CreatedAt   string  `json:"createdAt"`
	LastLoginAt *string `json:"lastLoginAt"`
}

func toUserJSON(u *store.User) userJSON {
	j := userJSON{
		ID: u.ID, Username: u.Username, Role: u.Role, CanDownload: u.CanDownload,
		CreatedAt: fmtTime(u.CreatedAt), LastLoginAt: fmtTimePtr(u.LastLoginAt),
	}
	if !u.IsAdmin() && u.LibraryIDs != nil {
		j.LibraryIDs = append([]int64{}, u.LibraryIDs...)
	}
	return j
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ---- handlers ----

func (a *App) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	n, err := a.store.CountUsers(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needsSetup": n == 0})
}

func (a *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decodeJSON(w, r, &c) {
		return
	}
	n, err := a.store.CountUsers(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	if n > 0 {
		conflict(w, "Setup has already been completed")
		return
	}
	c.Username = trimmed(c.Username)
	if msg := validateUsername(c.Username); msg != "" {
		badRequest(w, msg)
		return
	}
	if msg := validatePassword(c.Password); msg != "" {
		badRequest(w, msg)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcryptCost)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	u, err := a.store.CreateUser(r.Context(), store.NewUser{Username: c.Username, PasswordHash: string(hash), Role: "admin", CanDownload: true}, true)
	if errors.Is(err, store.ErrConflict) {
		conflict(w, "Setup has already been completed")
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.completeLogin(w, r, u)
}

func (a *App) completeLogin(w http.ResponseWriter, r *http.Request, u *store.User) {
	if err := a.startSession(r.Context(), w, r, u); err != nil {
		a.internalError(w, r, err)
		return
	}
	now := time.Now()
	_ = a.store.SetLastLogin(r.Context(), u.ID, now)
	u.LastLoginAt = &now
	ip := displayIP(r)
	uid := u.ID
	if err := a.store.AddActivity(r.Context(), store.Activity{At: now, UserID: &uid, Username: u.Username, Action: "login", Detail: &ip}); err != nil {
		a.log.Warn("record login activity", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserJSON(u)})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decodeJSON(w, r, &c) {
		return
	}
	c.Username = trimmed(c.Username)
	ip := remoteIP(r)
	key := ip + "|" + strings.ToLower(c.Username)
	if ok, retry := a.limiter.allow(ip, key, time.Now()); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many failed login attempts; try again later")
		return
	}
	var u *store.User
	var err error
	if c.Username != "" && len(c.Username) <= 64 {
		u, err = a.store.GetUserByUsername(r.Context(), c.Username)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			a.internalError(w, r, err)
			return
		}
	}
	pw := c.Password
	if len(pw) > 72 {
		pw = pw[:72]
	}
	if u == nil {
		// Equalize timing with the known-user path.
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(pw))
	} else if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pw)) != nil || len(c.Password) > 72 {
		u = nil
	}
	if u == nil {
		a.limiter.fail(ip, key, time.Now())
		a.log.Info("login failed", "username", c.Username, "ip", displayIP(r))
		writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid username or password")
		return
	}
	a.limiter.success(key)
	a.completeLogin(w, r, u)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		_ = a.store.DeleteSession(r.Context(), hashToken(c.Value))
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserJSON(userFrom(r))})
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	u := userFrom(r)
	ip := remoteIP(r)
	key := ip + "|pw|" + strconv.FormatInt(u.ID, 10)
	if ok, retry := a.limiter.allow(ip, key, time.Now()); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many failed attempts; try again later")
		return
	}
	if len(body.CurrentPassword) > 72 || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.CurrentPassword)) != nil {
		a.limiter.fail(ip, key, time.Now())
		badRequest(w, "Current password is incorrect")
		return
	}
	if msg := validatePassword(body.NewPassword); msg != "" {
		badRequest(w, msg)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcryptCost)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	hs := string(hash)
	if _, err := a.store.UpdateUser(r.Context(), u.ID, store.UserUpdate{PasswordHash: &hs}); err != nil {
		a.storeErr(w, r, err)
		return
	}
	if err := a.store.DeleteUserSessions(r.Context(), u.ID, sessionHashFrom(r)); err != nil {
		a.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- login rate limiting ----

const (
	limitWindow  = 15 * time.Minute
	limitPerUser = 5  // failures per IP+username per window
	limitPerIP   = 30 // failures per IP per window
)

type loginLimiter struct {
	mu    sync.Mutex
	fails map[string][]time.Time
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{fails: map[string][]time.Time{}} }

func (l *loginLimiter) recent(key string, now time.Time) []time.Time {
	ts := l.fails[key]
	i := 0
	for i < len(ts) && now.Sub(ts[i]) >= limitWindow {
		i++
	}
	ts = ts[i:]
	if len(ts) == 0 {
		delete(l.fails, key)
	} else {
		l.fails[key] = ts
	}
	return ts
}

func (l *loginLimiter) allow(ip, key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	check := func(k string, max int) (bool, time.Duration) {
		ts := l.recent(k, now)
		if len(ts) >= max {
			return false, limitWindow - now.Sub(ts[len(ts)-max])
		}
		return true, 0
	}
	if ok, d := check(key, limitPerUser); !ok {
		return false, d
	}
	return check("ip|"+ip, limitPerIP)
}

func (l *loginLimiter) fail(ip, key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range []string{key, "ip|" + ip} {
		ts := append(l.recent(k, now), now)
		if len(ts) > 64 {
			ts = ts[len(ts)-64:]
		}
		l.fails[k] = ts
	}
}

func (l *loginLimiter) success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

func (l *loginLimiter) gc(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k := range l.fails {
		l.recent(k, now)
	}
}
