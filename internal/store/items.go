package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"sort"
	"strings"
	"time"
	"unicode"

	"trove/internal/media"
)

// ItemSummary is the listing view of an item (effective/merged values).
type ItemSummary struct {
	ID           int64
	LibraryID    int64
	LibraryType  media.LibraryType
	Kind         media.Kind
	Title        string
	Year         int
	Author       string
	Rating       float64
	PosterV      string
	BackdropV    string
	AddedAt      time.Time
	Size         int64
	FileCount    int
	SeasonCount  int
	EpisodeCount int
}

// Item is the full stored item.
type Item struct {
	ItemSummary
	RelPath        string
	Scanned        media.ScannedItem // Files is nil
	Remote         *media.Metadata   // Episodes is nil; nil when never matched
	MetaState      string
	LocalPosterV   string
	LocalBackdropV string
}

// File is a stored file. RelPath is relative to the library root.
type File struct {
	ID     int64
	ItemID int64
	media.ScannedFile
}

// Details is the merged detail view of an item: remote values win when
// present, scanned values are the fallback.
type Details struct {
	OriginalTitle  string
	Tagline        string
	Overview       string
	Genres         []string
	RuntimeMinutes int
	ReleaseDate    string
	Status         string
	Studio         string
	Narrator       string
	Series         string
	SeriesIndex    float64
	ExternalIDs    map[string]string
}

// Details merges scanned and remote metadata.
func (it *Item) Details() Details {
	d := Details{
		Overview:    it.Scanned.Overview,
		Genres:      cleanGenres(it.Scanned.Genres),
		Narrator:    it.Scanned.Narrator,
		Series:      it.Scanned.Series,
		SeriesIndex: it.Scanned.SeriesIndex,
		ExternalIDs: map[string]string{},
	}
	maps.Copy(d.ExternalIDs, it.Scanned.IDs)
	if r := it.Remote; r != nil {
		d.OriginalTitle = r.OriginalTitle
		d.Tagline = r.Tagline
		if r.Overview != "" {
			d.Overview = r.Overview
		}
		if g := cleanGenres(r.Genres); len(g) > 0 {
			d.Genres = g
		}
		d.RuntimeMinutes = r.RuntimeMinutes
		d.ReleaseDate = r.ReleaseDate
		d.Status = r.Status
		d.Studio = r.Studio
		for k, v := range r.IDs {
			if v != "" {
				d.ExternalIDs[k] = v
			}
		}
	}
	for k, v := range d.ExternalIDs {
		if v == "" {
			delete(d.ExternalIDs, k)
		}
	}
	if d.OriginalTitle == it.Title {
		d.OriginalTitle = ""
	}
	return d
}

func cleanGenres(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, g := range in {
		g = strings.TrimSpace(g)
		k := strings.ToLower(g)
		if g == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, g)
	}
	return out
}

// SortTitle lowercases and strips a leading English article.
func SortTitle(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	t = strings.TrimLeftFunc(t, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for _, a := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(t, a) && len(t) > len(a) {
			t = strings.TrimSpace(t[len(a):])
			break
		}
	}
	if t == "" {
		t = strings.ToLower(title)
	}
	return t
}

type effective struct {
	title, sortTitle, author, search string
	year                             int
	rating                           float64
	genres                           []string
	posterV, backdropV               string
}

func shortHash(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum64())
}

func computeEffective(sc *media.ScannedItem, r *media.Metadata, localPosterV, localBackdropV string) effective {
	e := effective{
		title:     sc.Title,
		sortTitle: sc.SortTitle,
		year:      sc.Year,
		author:    sc.Author,
		genres:    cleanGenres(sc.Genres),
		posterV:   localPosterV,
		backdropV: localBackdropV,
	}
	if e.sortTitle == "" {
		e.sortTitle = SortTitle(sc.Title)
	}
	orig := ""
	if r != nil {
		if r.Title != "" {
			e.title = r.Title
			e.sortTitle = SortTitle(r.Title)
		}
		if r.Year > 0 {
			e.year = r.Year
		}
		if len(r.Authors) > 0 {
			e.author = strings.Join(r.Authors, ", ")
		}
		e.rating = r.Rating
		if g := cleanGenres(r.Genres); len(g) > 0 {
			e.genres = g
		}
		if e.posterV == "" && r.PosterURL != "" {
			e.posterV = "r" + shortHash(r.PosterURL)
		}
		if e.backdropV == "" && r.BackdropURL != "" {
			e.backdropV = "r" + shortHash(r.BackdropURL)
		}
		orig = r.OriginalTitle
	}
	if e.title == "" {
		e.title = sc.RelPath
		e.sortTitle = SortTitle(sc.RelPath)
	}
	e.search = strings.ToLower(strings.Join([]string{e.title, sc.Title, orig, e.author, sc.Author, sc.Series}, "\n"))
	return e
}

// IndexItem is one scanned item plus the cache-buster versions of its local
// artwork (empty when there is none).
type IndexItem struct {
	Item           media.ScannedItem
	LocalPosterV   string
	LocalBackdropV string
}

// IndexItems upserts scanned items of a library (identity: library_id +
// rel_path), stamping them with gen. Items/files not stamped with gen are
// removed later by PruneLibrary. firstScan makes brand-new items use their
// newest file mtime as addedAt (when older than now).
func (s *Store) IndexItems(ctx context.Context, libID, gen int64, items []IndexItem, firstScan bool, progress func(done int)) error {
	const batch = 200
	for start := 0; start < len(items); start += batch {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(start+batch, len(items))
		err := s.tx(ctx, func(tx *sql.Tx) error {
			for i := start; i < end; i++ {
				if err := upsertItem(ctx, tx, libID, gen, &items[i], firstScan); err != nil {
					return fmt.Errorf("index %q: %w", items[i].Item.RelPath, err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if progress != nil {
			progress(end)
		}
	}
	return nil
}

func upsertItem(ctx context.Context, tx *sql.Tx, libID, gen int64, in *IndexItem, firstScan bool) error {
	sc := in.Item
	files := sc.Files
	sc.Files = nil
	now := time.Now()

	var id int64
	var remoteJSON sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT id, remote_json FROM items WHERE library_id = ? AND rel_path = ?`, libID, sc.RelPath).Scan(&id, &remoteJSON)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	remote := decodeRemote(remoteJSON)
	e := computeEffective(&sc, remote, in.LocalPosterV, in.LocalBackdropV)
	scJSON, err := json.Marshal(sc)
	if err != nil {
		return err
	}

	var size int64
	seasons := map[int]bool{}
	episodes := 0
	for _, f := range files {
		size += f.Size
		if sc.Kind == media.KindShow && f.Role == media.RoleVideo && f.Episode > 0 {
			seasons[f.Season] = true
			episodes++
		}
	}

	if exists {
		_, err = tx.ExecContext(ctx, `UPDATE items SET kind = ?, scanned_json = ?, local_poster_v = ?, local_backdrop_v = ?,
			title = ?, sort_title = ?, year = ?, author = ?, rating = ?, search_text = ?, poster_v = ?, backdrop_v = ?,
			size = ?, file_count = ?, season_count = ?, episode_count = ?, updated_at = ?, scan_gen = ?
			WHERE id = ?`,
			string(sc.Kind), string(scJSON), in.LocalPosterV, in.LocalBackdropV,
			e.title, e.sortTitle, nullInt(e.year), nullStr(e.author), nullFloat(e.rating), e.search, e.posterV, e.backdropV,
			size, len(files), len(seasons), episodes, unix(now), gen, id)
		if err != nil {
			return err
		}
	} else {
		added := now
		if firstScan {
			var newest time.Time
			for _, f := range files {
				if f.ModTime.After(newest) {
					newest = f.ModTime
				}
			}
			if !newest.IsZero() && newest.Before(now) {
				added = newest
			}
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO items (library_id, rel_path, kind, scanned_json, local_poster_v, local_backdrop_v,
			title, sort_title, year, author, rating, search_text, poster_v, backdrop_v,
			size, file_count, season_count, episode_count, added_at, updated_at, scan_gen)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			libID, sc.RelPath, string(sc.Kind), string(scJSON), in.LocalPosterV, in.LocalBackdropV,
			e.title, e.sortTitle, nullInt(e.year), nullStr(e.author), nullFloat(e.rating), e.search, e.posterV, e.backdropV,
			size, len(files), len(seasons), episodes, unix(added), unix(now), gen)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
	}
	if err := setGenres(ctx, tx, id, e.genres); err != nil {
		return err
	}

	for _, f := range files {
		_, err := tx.ExecContext(ctx, `INSERT INTO files (item_id, rel_path, size, mod_time, role, container, season, episode, episode_end,
			episode_title, track, resolution, source, video_codec, audio_codec, hdr, edition, language, scan_gen)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(item_id, rel_path) DO UPDATE SET size = excluded.size, mod_time = excluded.mod_time, role = excluded.role,
			container = excluded.container, season = excluded.season, episode = excluded.episode, episode_end = excluded.episode_end,
			episode_title = excluded.episode_title, track = excluded.track, resolution = excluded.resolution, source = excluded.source,
			video_codec = excluded.video_codec, audio_codec = excluded.audio_codec, hdr = excluded.hdr, edition = excluded.edition,
			language = excluded.language, scan_gen = excluded.scan_gen`,
			id, f.RelPath, f.Size, unix(f.ModTime), string(f.Role), f.Container, f.Season, f.Episode, f.EpisodeEnd,
			f.EpisodeTitle, f.Track, f.Resolution, f.Source, f.VideoCodec, f.AudioCodec, f.HDR, f.Edition, f.Language, gen)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM files WHERE item_id = ? AND scan_gen <> ?`, id, gen)
	return err
}

func setGenres(ctx context.Context, tx *sql.Tx, itemID int64, genres []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_genres WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	for _, g := range genres {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_genres (item_id, genre) VALUES (?, ?)`, itemID, g); err != nil {
			return err
		}
	}
	return nil
}

func decodeRemote(v sql.NullString) *media.Metadata {
	if !v.Valid || v.String == "" {
		return nil
	}
	var m media.Metadata
	if json.Unmarshal([]byte(v.String), &m) != nil {
		return nil
	}
	return &m
}

// PruneLibrary deletes items of the library that were not seen in scan gen.
func (s *Store) PruneLibrary(ctx context.Context, libID, gen int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM items WHERE library_id = ? AND scan_gen <> ?`, libID, gen)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// MetaCandidate is an item to look up remotely.
type MetaCandidate struct {
	ID          int64
	LibraryID   int64
	LibraryType media.LibraryType
	Kind        media.Kind
	Scanned     media.ScannedItem
	Seasons     []int
}

// MetadataCandidates returns items of a library without remote metadata
// that were never tried (meta_state = 'none').
func (s *Store) MetadataCandidates(ctx context.Context, libID int64) ([]MetaCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.id FROM items i WHERE i.library_id = ? AND i.meta_state = 'none' ORDER BY i.added_at DESC, i.id`, libID)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]MetaCandidate, 0, len(ids))
	for _, id := range ids {
		c, err := s.MetadataCandidate(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, nil
}

// MetadataCandidate loads the lookup input for one item.
func (s *Store) MetadataCandidate(ctx context.Context, itemID int64) (*MetaCandidate, error) {
	var c MetaCandidate
	var kind, ltype, scJSON string
	err := s.db.QueryRowContext(ctx, `SELECT i.id, i.library_id, l.type, i.kind, i.scanned_json FROM items i JOIN libraries l ON l.id = i.library_id WHERE i.id = ?`, itemID).
		Scan(&c.ID, &c.LibraryID, &ltype, &kind, &scJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.Kind, c.LibraryType = media.Kind(kind), media.LibraryType(ltype)
	if err := json.Unmarshal([]byte(scJSON), &c.Scanned); err != nil {
		return nil, err
	}
	c.Seasons = []int{}
	if c.Kind == media.KindShow {
		rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT season FROM files WHERE item_id = ? AND role = 'video' AND episode > 0 ORDER BY season`, itemID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var n int
			if err := rows.Scan(&n); err != nil {
				return nil, err
			}
			c.Seasons = append(c.Seasons, n)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

// SetRemoteMetadata stores provider metadata (and its episodes) for an item.
func (s *Store) SetRemoteMetadata(ctx context.Context, itemID int64, md *media.Metadata) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		var scJSON, lp, lb string
		err := tx.QueryRowContext(ctx, `SELECT scanned_json, local_poster_v, local_backdrop_v FROM items WHERE id = ?`, itemID).Scan(&scJSON, &lp, &lb)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var sc media.ScannedItem
		if err := json.Unmarshal([]byte(scJSON), &sc); err != nil {
			return err
		}
		eps := md.Episodes
		stored := *md
		stored.Episodes = nil
		rJSON, err := json.Marshal(stored)
		if err != nil {
			return err
		}
		e := computeEffective(&sc, &stored, lp, lb)
		_, err = tx.ExecContext(ctx, `UPDATE items SET remote_json = ?, meta_state = 'matched', meta_provider = ?, meta_checked_at = ?,
			title = ?, sort_title = ?, year = ?, author = ?, rating = ?, search_text = ?, poster_v = ?, backdrop_v = ?, updated_at = ?
			WHERE id = ?`,
			string(rJSON), md.Provider, unix(time.Now()),
			e.title, e.sortTitle, nullInt(e.year), nullStr(e.author), nullFloat(e.rating), e.search, e.posterV, e.backdropV, unix(time.Now()), itemID)
		if err != nil {
			return err
		}
		if err := setGenres(ctx, tx, itemID, e.genres); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM episodes WHERE item_id = ?`, itemID); err != nil {
			return err
		}
		for _, ep := range eps {
			if ep.Episode <= 0 || ep.Season < 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO episodes (item_id, season, episode, title, overview, air_date) VALUES (?, ?, ?, ?, ?, ?)`,
				itemID, ep.Season, ep.Episode, ep.Title, ep.Overview, ep.AirDate); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetMetadataNotFound remembers that the lookup found nothing. Existing
// matched metadata is kept.
func (s *Store) SetMetadataNotFound(ctx context.Context, itemID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET meta_state = CASE WHEN meta_state = 'matched' THEN 'matched' ELSE 'notfound' END, meta_checked_at = ? WHERE id = ?`,
		unix(time.Now()), itemID)
	return err
}

// ResetNotFound makes every "not found" item eligible for lookup again.
func (s *Store) ResetNotFound(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET meta_state = 'none' WHERE meta_state = 'notfound'`)
	return err
}

const summaryCols = `i.id, i.library_id, l.type, i.kind, i.title, i.year, i.author, i.rating, i.poster_v, i.backdrop_v,
	i.added_at, i.size, i.file_count, i.season_count, i.episode_count`

func scanSummary(r rowScanner, extra ...any) (*ItemSummary, error) {
	var it ItemSummary
	var ltype, kind string
	var year sql.NullInt64
	var author sql.NullString
	var rating sql.NullFloat64
	var added int64
	dest := []any{&it.ID, &it.LibraryID, &ltype, &kind, &it.Title, &year, &author, &rating, &it.PosterV, &it.BackdropV,
		&added, &it.Size, &it.FileCount, &it.SeasonCount, &it.EpisodeCount}
	if err := r.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}
	it.LibraryType, it.Kind = media.LibraryType(ltype), media.Kind(kind)
	it.Year = int(year.Int64)
	it.Author = author.String
	it.Rating = rating.Float64
	it.AddedAt = fromUnix(added)
	return &it, nil
}

// GetItem loads one item.
func (s *Store) GetItem(ctx context.Context, id int64) (*Item, error) {
	var it Item
	var scJSON string
	var remote sql.NullString
	sum, err := scanSummary(s.db.QueryRowContext(ctx, `SELECT `+summaryCols+`, i.rel_path, i.scanned_json, i.remote_json, i.meta_state,
		i.local_poster_v, i.local_backdrop_v FROM items i JOIN libraries l ON l.id = i.library_id WHERE i.id = ?`, id),
		&it.RelPath, &scJSON, &remote, &it.MetaState, &it.LocalPosterV, &it.LocalBackdropV)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	it.ItemSummary = *sum
	if err := json.Unmarshal([]byte(scJSON), &it.Scanned); err != nil {
		return nil, err
	}
	it.Remote = decodeRemote(remote)
	return &it, nil
}

// ItemQuery filters /items.
type ItemQuery struct {
	// Accessible restricts to these libraries; nil = no restriction.
	Accessible []int64
	LibraryID  int64
	Kind       media.Kind
	Q          string
	Genre      string
	Sort       string // title | added | year | rating | size
	Desc       bool
	Limit      int
	Offset     int
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// QueryItems runs a filtered, sorted, paged listing.
func (s *Store) QueryItems(ctx context.Context, q ItemQuery) ([]ItemSummary, int, error) {
	var where []string
	var args []any
	if q.Accessible != nil {
		if len(q.Accessible) == 0 {
			return []ItemSummary{}, 0, nil
		}
		where = append(where, `i.library_id IN (`+placeholders(len(q.Accessible))+`)`)
		for _, id := range q.Accessible {
			args = append(args, id)
		}
	}
	if q.LibraryID != 0 {
		where = append(where, `i.library_id = ?`)
		args = append(args, q.LibraryID)
	}
	if q.Kind != "" {
		where = append(where, `i.kind = ?`)
		args = append(args, string(q.Kind))
	}
	if t := strings.TrimSpace(q.Q); t != "" {
		where = append(where, `i.search_text LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(strings.ToLower(t))+"%")
	}
	if q.Genre != "" {
		where = append(where, `EXISTS (SELECT 1 FROM item_genres g WHERE g.item_id = i.id AND g.genre = ?)`)
		args = append(args, q.Genre)
	}
	w := ""
	if len(where) > 0 {
		w = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items i`+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}
	var order string
	switch q.Sort {
	case "added":
		order = "i.added_at " + dir + ", i.sort_title, i.id"
	case "year":
		order = "(i.year IS NULL), i.year " + dir + ", i.sort_title, i.id"
	case "rating":
		order = "(i.rating IS NULL), i.rating " + dir + ", i.sort_title, i.id"
	case "size":
		order = "i.size " + dir + ", i.sort_title, i.id"
	default:
		order = "i.sort_title " + dir + ", i.id " + dir
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 48
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+summaryCols+` FROM items i JOIN libraries l ON l.id = i.library_id`+w+
		` ORDER BY `+order+` LIMIT ? OFFSET ?`, append(args, limit, q.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []ItemSummary{}
	for rows.Next() {
		it, err := scanSummary(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *it)
	}
	return out, total, rows.Err()
}

// Genres returns the distinct genres of the accessible items, sorted.
func (s *Store) Genres(ctx context.Context, accessible []int64, libraryID int64) ([]string, error) {
	var where []string
	var args []any
	if accessible != nil {
		if len(accessible) == 0 {
			return []string{}, nil
		}
		where = append(where, `i.library_id IN (`+placeholders(len(accessible))+`)`)
		for _, id := range accessible {
			args = append(args, id)
		}
	}
	if libraryID != 0 {
		where = append(where, `i.library_id = ?`)
		args = append(args, libraryID)
	}
	w := ""
	if len(where) > 0 {
		w = " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT g.genre FROM item_genres g JOIN items i ON i.id = g.item_id`+w, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i]), strings.ToLower(out[j])
		if a != b {
			return a < b
		}
		return out[i] < out[j]
	})
	return out, rows.Err()
}

const fileCols = `id, item_id, rel_path, size, mod_time, role, container, season, episode, episode_end, episode_title, track,
	resolution, source, video_codec, audio_codec, hdr, edition, language`

func scanFile(r rowScanner) (*File, error) {
	var f File
	var mod int64
	var role string
	if err := r.Scan(&f.ID, &f.ItemID, &f.RelPath, &f.Size, &mod, &role, &f.Container, &f.Season, &f.Episode, &f.EpisodeEnd,
		&f.EpisodeTitle, &f.Track, &f.Resolution, &f.Source, &f.VideoCodec, &f.AudioCodec, &f.HDR, &f.Edition, &f.Language); err != nil {
		return nil, err
	}
	f.ModTime = fromUnix(mod)
	f.Role = media.FileRole(role)
	return &f, nil
}

// ItemFiles returns every file of an item ordered by rel path.
func (s *Store) ItemFiles(ctx context.Context, itemID int64) ([]File, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+fileCols+` FROM files WHERE item_id = ? ORDER BY rel_path`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []File{}
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// GetFile returns one file.
func (s *Store) GetFile(ctx context.Context, id int64) (*File, error) {
	f, err := scanFile(s.db.QueryRowContext(ctx, `SELECT `+fileCols+` FROM files WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// EpisodeKey identifies an episode within a show.
type EpisodeKey struct{ Season, Episode int }

// ItemEpisodes returns stored remote episode metadata keyed by (season, episode).
func (s *Store) ItemEpisodes(ctx context.Context, itemID int64) (map[EpisodeKey]media.EpisodeMetadata, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT season, episode, title, overview, air_date FROM episodes WHERE item_id = ?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[EpisodeKey]media.EpisodeMetadata{}
	for rows.Next() {
		var e media.EpisodeMetadata
		if err := rows.Scan(&e.Season, &e.Episode, &e.Title, &e.Overview, &e.AirDate); err != nil {
			return nil, err
		}
		out[EpisodeKey{Season: e.Season, Episode: e.Episode}] = e
	}
	return out, rows.Err()
}

// ItemIDs returns the set of all item ids (used for cache cleanup).
func (s *Store) ItemIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM items`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
