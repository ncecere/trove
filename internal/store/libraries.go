package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"trove/internal/media"
)

// Library is a mapped folder.
type Library struct {
	ID         int64
	Name       string
	Type       media.LibraryType
	Path       string
	CreatedAt  time.Time
	LastScanAt *time.Time
	ItemCount  int64
	TotalSize  int64
}

const libSelect = `SELECT l.id, l.name, l.type, l.path, l.created_at, l.last_scan_at,
	(SELECT COUNT(*) FROM items i WHERE i.library_id = l.id),
	(SELECT COALESCE(SUM(i.size), 0) FROM items i WHERE i.library_id = l.id)
	FROM libraries l`

func scanLibrary(r rowScanner) (*Library, error) {
	var l Library
	var typ string
	var created int64
	var last sql.NullInt64
	if err := r.Scan(&l.ID, &l.Name, &typ, &l.Path, &created, &last, &l.ItemCount, &l.TotalSize); err != nil {
		return nil, err
	}
	l.Type = media.LibraryType(typ)
	l.CreatedAt = fromUnix(created)
	l.LastScanAt = fromNullUnix(last)
	return &l, nil
}

// ListLibraries returns all libraries sorted by name.
func (s *Store) ListLibraries(ctx context.Context) ([]*Library, error) {
	rows, err := s.db.QueryContext(ctx, libSelect+` ORDER BY l.name COLLATE NOCASE, l.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Library
	for rows.Next() {
		l, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetLibrary returns one library.
func (s *Store) GetLibrary(ctx context.Context, id int64) (*Library, error) {
	l, err := scanLibrary(s.db.QueryRowContext(ctx, libSelect+` WHERE l.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return l, err
}

// LibraryExists reports whether all ids exist.
func (s *Store) LibrariesExist(ctx context.Context, ids []int64) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}
	uniq := map[int64]bool{}
	args := []any{}
	for _, id := range ids {
		if !uniq[id] {
			uniq[id] = true
			args = append(args, id)
		}
	}
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM libraries WHERE id IN (`+placeholders(len(args))+`)`, args...).Scan(&n)
	return n == len(args), err
}

// CreateLibrary inserts a library.
func (s *Store) CreateLibrary(ctx context.Context, name string, typ media.LibraryType, path string) (*Library, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO libraries (name, type, path, created_at) VALUES (?, ?, ?, ?)`,
		name, string(typ), path, unix(time.Now()))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetLibrary(ctx, id)
}

// UpdateLibrary changes name/type/path (nil = unchanged). When the type
// changes, remote metadata of the library is reset since it no longer applies.
func (s *Store) UpdateLibrary(ctx context.Context, id int64, name *string, typ *media.LibraryType, path *string) (*Library, error) {
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var curType string
		err := tx.QueryRowContext(ctx, `SELECT type FROM libraries WHERE id = ?`, id).Scan(&curType)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if name != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE libraries SET name = ? WHERE id = ?`, *name, id); err != nil {
				return err
			}
		}
		if path != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE libraries SET path = ? WHERE id = ?`, *path, id); err != nil {
				return err
			}
		}
		if typ != nil && string(*typ) != curType {
			if _, err := tx.ExecContext(ctx, `UPDATE libraries SET type = ? WHERE id = ?`, string(*typ), id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM episodes WHERE item_id IN (SELECT id FROM items WHERE library_id = ?)`, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE items SET remote_json = NULL, meta_state = 'none', meta_provider = '', meta_checked_at = NULL WHERE library_id = ?`, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetLibrary(ctx, id)
}

// DeleteLibrary removes a library and its index (cascade).
func (s *Store) DeleteLibrary(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM libraries WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// NextScanGen increments and returns the scan generation of a library.
func (s *Store) NextScanGen(ctx context.Context, id int64) (int64, error) {
	var gen int64
	err := s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE libraries SET scan_gen = scan_gen + 1 WHERE id = ?`, id); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `SELECT scan_gen FROM libraries WHERE id = ?`, id).Scan(&gen)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return gen, err
}

// SetLibraryScanned records a completed scan.
func (s *Store) SetLibraryScanned(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE libraries SET last_scan_at = ? WHERE id = ?`, unix(at), id)
	return err
}

// GetSettings returns all key/value settings.
func (s *Store) GetSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// PutSettings upserts the given settings.
func (s *Store) PutSettings(ctx context.Context, kv map[string]string) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		for k, v := range kv {
			if _, err := tx.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v); err != nil {
				return err
			}
		}
		return nil
	})
}

// Activity is one activity log row.
type Activity struct {
	ID        int64
	At        time.Time
	UserID    *int64
	Username  string
	Action    string
	ItemID    *int64
	ItemTitle *string
	Detail    *string
	Size      *int64
}

// AddActivity appends to the activity log.
func (s *Store) AddActivity(ctx context.Context, a Activity) error {
	if a.At.IsZero() {
		a.At = time.Now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO activity (at, user_id, username, action, item_id, item_title, detail, size) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		unix(a.At), a.UserID, a.Username, a.Action, a.ItemID, a.ItemTitle, a.Detail, a.Size)
	return err
}

// ListActivity returns activity newest first.
func (s *Store) ListActivity(ctx context.Context, limit, offset int) ([]Activity, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, at, user_id, username, action, item_id, item_title, detail, size FROM activity ORDER BY at DESC, id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		var a Activity
		var at int64
		var uid, iid, size sql.NullInt64
		var title, detail sql.NullString
		if err := rows.Scan(&a.ID, &at, &uid, &a.Username, &a.Action, &iid, &title, &detail, &size); err != nil {
			return nil, 0, err
		}
		a.At = fromUnix(at)
		if uid.Valid {
			a.UserID = &uid.Int64
		}
		if iid.Valid {
			a.ItemID = &iid.Int64
		}
		if title.Valid {
			a.ItemTitle = &title.String
		}
		if detail.Valid {
			a.Detail = &detail.String
		}
		if size.Valid {
			a.Size = &size.Int64
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// PruneActivity removes rows older than before.
func (s *Store) PruneActivity(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM activity WHERE at < ?`, unix(before))
	return err
}
