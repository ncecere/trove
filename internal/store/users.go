package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// User is an account. LibraryIDs == nil means access to all libraries.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string // "admin" | "user"
	CanDownload  bool
	LibraryIDs   []int64
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

func (u *User) IsAdmin() bool { return u.Role == "admin" }

// CanAccess reports whether the user may see the library.
func (u *User) CanAccess(libraryID int64) bool {
	if u.IsAdmin() || u.LibraryIDs == nil {
		return true
	}
	for _, id := range u.LibraryIDs {
		if id == libraryID {
			return true
		}
	}
	return false
}

const userCols = `id, username, password_hash, role, can_download, all_libraries, created_at, last_login_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(r rowScanner) (*User, bool, error) {
	var u User
	var canDL, all int
	var created int64
	var last sql.NullInt64
	if err := r.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &canDL, &all, &created, &last); err != nil {
		return nil, false, err
	}
	u.CanDownload = canDL != 0
	u.CreatedAt = fromUnix(created)
	u.LastLoginAt = fromNullUnix(last)
	return &u, all != 0, nil
}

func (s *Store) loadUserLibraries(ctx context.Context, q queryer, users map[int64]*User) error {
	if len(users) == 0 {
		return nil
	}
	rows, err := q.QueryContext(ctx, `SELECT user_id, library_id FROM user_libraries ORDER BY library_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var uid, lid int64
		if err := rows.Scan(&uid, &lid); err != nil {
			return err
		}
		if u, ok := users[uid]; ok && u.LibraryIDs != nil {
			u.LibraryIDs = append(u.LibraryIDs, lid)
		}
	}
	return rows.Err()
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *Store) getUser(ctx context.Context, q queryer, where string, arg any) (*User, error) {
	u, all, err := scanUser(q.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE `+where, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !all && u.Role != "admin" {
		u.LibraryIDs = []int64{}
		if err := s.loadUserLibraries(ctx, q, map[int64]*User{u.ID: u}); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// GetUser returns the user by id.
func (s *Store) GetUser(ctx context.Context, id int64) (*User, error) {
	return s.getUser(ctx, s.db, "id = ?", id)
}

// GetUserByUsername looks a user up case-insensitively.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return s.getUser(ctx, s.db, "username = ?", username)
}

// ListUsers returns every user sorted by username.
func (s *Store) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY username COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	var out []*User
	byID := map[int64]*User{}
	for rows.Next() {
		u, all, err := scanUser(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		if !all && u.Role != "admin" {
			u.LibraryIDs = []int64{}
		}
		out = append(out, u)
		byID[u.ID] = u
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.loadUserLibraries(ctx, s.db, byID); err != nil {
		return nil, err
	}
	return out, nil
}

// CountUsers returns the number of users.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// NewUser is the input for CreateUser.
type NewUser struct {
	Username     string
	PasswordHash string
	Role         string
	CanDownload  bool
	LibraryIDs   []int64 // nil = all
}

// CreateUser inserts a user. If onlyIfFirst is set, the insert only happens
// when no user exists yet (returns ErrConflict otherwise).
func (s *Store) CreateUser(ctx context.Context, nu NewUser, onlyIfFirst bool) (*User, error) {
	var id int64
	err := s.tx(ctx, func(tx *sql.Tx) error {
		if onlyIfFirst {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return ErrConflict
			}
		}
		all := nu.LibraryIDs == nil || nu.Role == "admin"
		res, err := tx.ExecContext(ctx, `INSERT INTO users (username, password_hash, role, can_download, all_libraries, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`, nu.Username, nu.PasswordHash, nu.Role, nu.CanDownload, all, unix(time.Now()))
		if isUnique(err) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		if !all {
			return setUserLibraries(ctx, tx, id, nu.LibraryIDs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetUser(ctx, id)
}

func setUserLibraries(ctx context.Context, tx *sql.Tx, userID int64, ids []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_libraries WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, lid := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO user_libraries (user_id, library_id) VALUES (?, ?)`, userID, lid); err != nil {
			return err
		}
	}
	return nil
}

// UserUpdate holds optional changes. LibraryIDsSet distinguishes "set to
// null" (all) from "not provided".
type UserUpdate struct {
	Username      *string
	PasswordHash  *string
	Role          *string
	CanDownload   *bool
	LibraryIDsSet bool
	LibraryIDs    []int64
}

// UpdateUser applies the update. Returns ErrLastAdmin if it would demote the
// last admin, ErrConflict on duplicate username.
func (s *Store) UpdateUser(ctx context.Context, id int64, up UserUpdate) (*User, error) {
	err := s.tx(ctx, func(tx *sql.Tx) error {
		cur, err := s.getUser(ctx, tx, "id = ?", id)
		if err != nil {
			return err
		}
		role := cur.Role
		if up.Role != nil {
			role = *up.Role
		}
		if cur.Role == "admin" && role != "admin" {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> ?`, id).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return ErrLastAdmin
			}
		}
		if up.Username != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET username = ? WHERE id = ?`, *up.Username, id); err != nil {
				if isUnique(err) {
					return ErrConflict
				}
				return err
			}
		}
		if up.PasswordHash != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, *up.PasswordHash, id); err != nil {
				return err
			}
		}
		if up.CanDownload != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET can_download = ? WHERE id = ?`, *up.CanDownload, id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, role, id); err != nil {
			return err
		}
		switch {
		case role == "admin":
			if _, err := tx.ExecContext(ctx, `UPDATE users SET all_libraries = 1 WHERE id = ?`, id); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `DELETE FROM user_libraries WHERE user_id = ?`, id)
			return err
		case up.LibraryIDsSet && up.LibraryIDs == nil:
			if _, err := tx.ExecContext(ctx, `UPDATE users SET all_libraries = 1 WHERE id = ?`, id); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `DELETE FROM user_libraries WHERE user_id = ?`, id)
			return err
		case up.LibraryIDsSet:
			if _, err := tx.ExecContext(ctx, `UPDATE users SET all_libraries = 0 WHERE id = ?`, id); err != nil {
				return err
			}
			return setUserLibraries(ctx, tx, id, up.LibraryIDs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetUser(ctx, id)
}

// DeleteUser removes a user. Returns ErrLastAdmin if it is the last admin.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		var role string
		err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, id).Scan(&role)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if role == "admin" {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> ?`, id).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return ErrLastAdmin
			}
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
		return err
	})
}

// SetLastLogin records a successful login.
func (s *Store) SetLastLogin(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_login_at = ? WHERE id = ?`, unix(at), id)
	return err
}

// Session is a login session; the token itself is never stored.
type Session struct {
	TokenHash string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateSession stores a new session.
func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, expires time.Time, ip, ua string) error {
	if len(ua) > 512 {
		ua = ua[:512]
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, created_at, expires_at, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?)`,
		tokenHash, userID, unix(time.Now()), unix(expires), ip, ua)
	return err
}

// GetSession returns a non-expired session and its user.
func (s *Store) GetSession(ctx context.Context, tokenHash string, now time.Time) (*Session, *User, error) {
	var sess Session
	var created, expires int64
	err := s.db.QueryRowContext(ctx, `SELECT token_hash, user_id, created_at, expires_at FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		tokenHash, unix(now)).Scan(&sess.TokenHash, &sess.UserID, &created, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	sess.CreatedAt, sess.ExpiresAt = fromUnix(created), fromUnix(expires)
	u, err := s.GetUser(ctx, sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	return &sess, u, nil
}

// ExtendSession moves the expiry of a session.
func (s *Store) ExtendSession(ctx context.Context, tokenHash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`, unix(expires), tokenHash)
	return err
}

// DeleteSession removes one session.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteUserSessions removes all sessions of a user except keepHash.
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64, keepHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`, userID, keepHash)
	return err
}

// DeleteExpiredSessions purges expired sessions.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, unix(now))
	return err
}
