-- Initial schema. Times are unix seconds (UTC).

CREATE TABLE users (
    id             INTEGER PRIMARY KEY,
    username       TEXT    NOT NULL COLLATE NOCASE UNIQUE,
    password_hash  TEXT    NOT NULL,
    role           TEXT    NOT NULL CHECK (role IN ('admin', 'user')),
    can_download   INTEGER NOT NULL DEFAULT 1,
    all_libraries  INTEGER NOT NULL DEFAULT 1,
    created_at     INTEGER NOT NULL,
    last_login_at  INTEGER
);

CREATE TABLE sessions (
    token_hash  TEXT    PRIMARY KEY,           -- hex sha256 of the cookie token
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL,
    ip          TEXT    NOT NULL DEFAULT '',
    user_agent  TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE INDEX sessions_expires ON sessions(expires_at);

CREATE TABLE libraries (
    id            INTEGER PRIMARY KEY,
    name          TEXT    NOT NULL,
    type          TEXT    NOT NULL,
    path          TEXT    NOT NULL,
    created_at    INTEGER NOT NULL,
    last_scan_at  INTEGER,                     -- last completed scan
    scan_gen      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE user_libraries (
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    library_id  INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, library_id)
) WITHOUT ROWID;
CREATE INDEX user_libraries_library ON user_libraries(library_id);

CREATE TABLE items (
    id              INTEGER PRIMARY KEY,
    library_id      INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    rel_path        TEXT    NOT NULL,
    kind            TEXT    NOT NULL,

    -- What the scanner found (media.ScannedItem without Files) and what the
    -- metadata provider returned (media.Metadata without Episodes). Both are
    -- kept so a rescan never loses remote data and vice versa.
    scanned_json    TEXT    NOT NULL,
    remote_json     TEXT,
    meta_state      TEXT    NOT NULL DEFAULT 'none', -- none | matched | notfound
    meta_provider   TEXT    NOT NULL DEFAULT '',
    meta_checked_at INTEGER,
    local_poster_v  TEXT    NOT NULL DEFAULT '',
    local_backdrop_v TEXT   NOT NULL DEFAULT '',

    -- Effective (merged) values used for listing, sorting and searching.
    title           TEXT    NOT NULL,
    sort_title      TEXT    NOT NULL,
    year            INTEGER,
    author          TEXT,
    rating          REAL,
    search_text     TEXT    NOT NULL DEFAULT '',
    poster_v        TEXT    NOT NULL DEFAULT '',
    backdrop_v      TEXT    NOT NULL DEFAULT '',

    -- Aggregates over files.
    size            INTEGER NOT NULL DEFAULT 0,
    file_count      INTEGER NOT NULL DEFAULT 0,
    season_count    INTEGER NOT NULL DEFAULT 0,
    episode_count   INTEGER NOT NULL DEFAULT 0,

    added_at        INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    scan_gen        INTEGER NOT NULL DEFAULT 0,
    UNIQUE (library_id, rel_path)
);
CREATE INDEX items_lib_sort  ON items(library_id, sort_title);
CREATE INDEX items_sort      ON items(sort_title);
CREATE INDEX items_lib_added ON items(library_id, added_at);
CREATE INDEX items_added     ON items(added_at);
CREATE INDEX items_year      ON items(year);
CREATE INDEX items_rating    ON items(rating);
CREATE INDEX items_size      ON items(size);
CREATE INDEX items_kind      ON items(kind);
CREATE INDEX items_meta      ON items(library_id, meta_state);

CREATE TABLE item_genres (
    item_id  INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    genre    TEXT    NOT NULL,
    PRIMARY KEY (item_id, genre)
) WITHOUT ROWID;
CREATE INDEX item_genres_genre ON item_genres(genre, item_id);

CREATE TABLE files (
    id            INTEGER PRIMARY KEY,
    item_id       INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    rel_path      TEXT    NOT NULL,          -- relative to the library root
    size          INTEGER NOT NULL,
    mod_time      INTEGER NOT NULL,
    role          TEXT    NOT NULL,
    container     TEXT    NOT NULL DEFAULT '',
    season        INTEGER NOT NULL DEFAULT 0,
    episode       INTEGER NOT NULL DEFAULT 0,
    episode_end   INTEGER NOT NULL DEFAULT 0,
    episode_title TEXT    NOT NULL DEFAULT '',
    track         INTEGER NOT NULL DEFAULT 0,
    resolution    TEXT    NOT NULL DEFAULT '',
    source        TEXT    NOT NULL DEFAULT '',
    video_codec   TEXT    NOT NULL DEFAULT '',
    audio_codec   TEXT    NOT NULL DEFAULT '',
    hdr           TEXT    NOT NULL DEFAULT '',
    edition       TEXT    NOT NULL DEFAULT '',
    language      TEXT    NOT NULL DEFAULT '',
    scan_gen      INTEGER NOT NULL DEFAULT 0,
    UNIQUE (item_id, rel_path)
);

-- Remote per-episode metadata, joined to files by (season, episode).
CREATE TABLE episodes (
    item_id   INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    season    INTEGER NOT NULL,
    episode   INTEGER NOT NULL,
    title     TEXT    NOT NULL DEFAULT '',
    overview  TEXT    NOT NULL DEFAULT '',
    air_date  TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (item_id, season, episode)
) WITHOUT ROWID;

CREATE TABLE settings (
    key    TEXT PRIMARY KEY,
    value  TEXT NOT NULL
) WITHOUT ROWID;

CREATE TABLE activity (
    id          INTEGER PRIMARY KEY,
    at          INTEGER NOT NULL,
    user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    username    TEXT    NOT NULL,
    action      TEXT    NOT NULL,
    item_id     INTEGER REFERENCES items(id) ON DELETE SET NULL,
    item_title  TEXT,
    detail      TEXT,
    size        INTEGER
);
CREATE INDEX activity_at ON activity(at);
CREATE INDEX activity_user ON activity(user_id);
CREATE INDEX activity_item ON activity(item_id);
