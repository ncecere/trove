# Trove HTTP API (v1)

Trove is a self-hosted media library browser: the media lives on a NAS, Trove
indexes mapped library folders (Movies, TV, Anime, Books, Audiobooks) and lets
local users browse and download. This document is the contract between the Go
backend and the TypeScript frontend.

## Conventions

- Base path: `/api/v1`. Everything else is the embedded single-page app
  (unknown non-API paths serve `index.html`).
- JSON in, JSON out. Field names are `camelCase`. Timestamps are RFC 3339
  strings (UTC). Sizes are bytes (number). Absent optional values are `null`
  (never omitted) unless stated otherwise. Arrays are never `null`, use `[]`.
- Auth is a session cookie `trove_session` (HttpOnly, SameSite=Lax, `Secure`
  when the request arrived over HTTPS / `X-Forwarded-Proto: https`).
- CSRF: every non-GET/HEAD request to `/api/` MUST carry the header
  `X-Requested-With: trove`, otherwise `403 csrf`.
- Errors: non-2xx with body `{"error": {"code": "snake_case", "message": "Human readable"}}`.
  Common codes: `unauthorized` (401), `forbidden` (403), `not_found` (404),
  `invalid_request` (400), `conflict` (409), `rate_limited` (429), `internal` (500).
- "Admin" endpoints return 403 for non-admin users.
- Users only ever see libraries they have access to; items/files of other
  libraries behave as `404 not_found`.

## Types

```ts
type Role = "admin" | "user";
type LibraryType = "movies" | "tv" | "anime" | "books" | "audiobooks";
type Kind = "movie" | "show" | "book" | "audiobook";
type FileRole = "video" | "audio" | "ebook" | "subtitle" | "extra";

interface User {
  id: number;
  username: string;
  role: Role;
  canDownload: boolean;          // false = browse only
  libraryIds: number[] | null;   // null = access to all libraries (admins are always null)
  createdAt: string;
  lastLoginAt: string | null;
}

interface ScanStatus {
  state: "idle" | "scanning" | "error";
  phase: "walking" | "indexing" | "metadata" | null; // null when idle
  done: number;                  // items processed in the current phase
  total: number;                 // 0 when unknown
  error: string | null;          // last error, when state == "error"
  startedAt: string | null;
  finishedAt: string | null;     // last completed scan
}

interface Library {
  id: number;
  name: string;
  type: LibraryType;
  path: string | null;           // absolute path on the server; null for non-admins
  itemCount: number;
  totalSize: number;
  scan: ScanStatus;
  createdAt: string;
}

interface ItemSummary {
  id: number;
  libraryId: number;
  libraryType: LibraryType;
  kind: Kind;
  title: string;
  year: number | null;
  author: string | null;         // books / audiobooks
  posterUrl: string | null;      // e.g. "/api/v1/items/12/poster?v=1731700000"
  rating: number | null;         // 0-10
  addedAt: string;
  size: number;                  // total bytes of all files
  fileCount: number;
  seasonCount: number | null;    // shows only
  episodeCount: number | null;   // shows only
}

interface MediaFile {
  id: number;
  name: string;                  // base filename
  relPath: string;               // relative to the item folder (for display)
  size: number;
  role: FileRole;
  container: string;             // "mkv", "epub", ...
  resolution: string | null;
  source: string | null;
  videoCodec: string | null;
  audioCodec: string | null;
  hdr: string | null;
  edition: string | null;
  language: string | null;
  track: number | null;          // audiobook part order
  modifiedAt: string;
}

interface Episode {
  season: number;                // 0 = specials
  episode: number;
  episodeEnd: number | null;     // multi-episode files
  title: string | null;
  overview: string | null;
  airDate: string | null;        // YYYY-MM-DD
  file: MediaFile;               // the video file
  subtitles: MediaFile[];
}

interface Season {
  season: number;                // 0 = specials
  episodeCount: number;
  size: number;
  episodes: Episode[];           // sorted by episode
}

interface ItemDetail extends ItemSummary {
  originalTitle: string | null;
  tagline: string | null;
  overview: string | null;
  genres: string[];
  runtimeMinutes: number | null;
  releaseDate: string | null;    // YYYY-MM-DD
  status: string | null;
  studio: string | null;
  narrator: string | null;
  series: string | null;
  seriesIndex: number | null;
  backdropUrl: string | null;
  externalIds: Record<string, string>; // tmdb, imdb, tvdb, isbn, asin, openlibrary
  files: MediaFile[];            // every file that is NOT an episode (or episode subtitle); sorted by role, track, name
  seasons: Season[];             // shows only, else []
}

interface Page<T> { items: T[]; total: number; page: number; limit: number; }
```

## Setup & auth

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/setup/status` | | `{ "needsSetup": boolean }` (true while no user exists). Public. |
| POST | `/setup` | `{username, password}` | `{ "user": User }`, creates the first admin and logs in. `409 conflict` once any user exists. Public. |
| POST | `/auth/login` | `{username, password}` | `{ "user": User }` + cookie. `401 unauthorized` on bad credentials, `429 rate_limited` after repeated failures. Public. |
| POST | `/auth/logout` | | `204` |
| GET | `/auth/me` | | `{ "user": User }` or `401` |
| PUT | `/auth/password` | `{currentPassword, newPassword}` | `204`. Invalidates the user's other sessions. |

Rules: username 3-32 chars `[a-zA-Z0-9._-]`, case-insensitive unique. Password
min 8 chars.

## Users (admin)

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/users` | | `{ "users": User[] }` |
| POST | `/users` | `{username, password, role, canDownload, libraryIds}` | `201 { "user": User }` |
| PATCH | `/users/{id}` | any of `{username, password, role, canDownload, libraryIds}` | `{ "user": User }` |
| DELETE | `/users/{id}` | | `204` |

The last remaining admin cannot be deleted or demoted (`409 conflict`). An
admin cannot delete themselves.

## Libraries

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/libraries` | | `{ "libraries": Library[] }`, only those the user can access, sorted by name |
| POST | `/libraries` (admin) | `{name, type, path}` | `201 { "library": Library }`; starts a scan. `400` if path is not an existing directory. |
| PATCH | `/libraries/{id}` (admin) | any of `{name, type, path}` | `{ "library": Library }`; changing type/path triggers a rescan |
| DELETE | `/libraries/{id}` (admin) | | `204`; removes the index only, never touches files on disk |
| POST | `/libraries/{id}/scan` (admin) | | `202 { "library": Library }`; no-op if already scanning |
| POST | `/libraries/scan` (admin) | | `202 { "libraries": Library[] }`; scans all |
| GET | `/admin/fs?path=/abs/dir` (admin) | | `{ "path": string, "parent": string \| null, "dirs": [{ "name": string, "path": string }] }`; directory picker; defaults to `/` when path is empty; hidden dirs excluded |

## Browsing

| Method | Path | Response |
|---|---|---|
| GET | `/home` | `{ "recentlyAdded": ItemSummary[], "libraries": [{ "library": Library, "recent": ItemSummary[] }] }`, up to 20 each, newest first |
| GET | `/items` | `Page<ItemSummary>` |
| GET | `/items/{id}` | `ItemDetail` |
| GET | `/genres?library={id}` | `{ "genres": string[] }`, sorted; `library` optional |

`GET /items` query parameters (all optional):

- `library` library id
- `kind` one of Kind
- `q` case-insensitive substring search over title, original title, author, series
- `genre` exact genre name
- `sort` `title` (default) | `added` | `year` | `rating` | `size`
- `order` `asc` | `desc` (default: asc for title, desc otherwise)
- `page` 1-based (default 1), `limit` 1-100 (default 48)

## Artwork

| Method | Path | Response |
|---|---|---|
| GET | `/items/{id}/poster` | image bytes; `404` if none. Long-lived `Cache-Control` (URLs carry a `?v=` cache buster). |
| GET | `/items/{id}/backdrop` | same |

## Downloads

All require `canDownload`; otherwise `403 forbidden`.

| Method | Path | Response |
|---|---|---|
| GET | `/files/{id}/download` | the file, `Content-Disposition: attachment`, HTTP Range supported (resumable) |
| GET | `/files/{id}/stream` | same bytes with `Content-Disposition: inline` and a real media `Content-Type`, Range supported (for in-browser preview) |
| GET | `/items/{id}/download` | `application/zip` stream (stored, no compression) of every file of the item, laid out as on disk under a top-level folder named after the item. `?season=N` limits a show to one season. |

Every download start is recorded in the activity log.

## Admin: metadata, settings, activity

| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/items/{id}/refresh` (admin) | | `202 ItemDetail`; re-fetches remote metadata for one item (in background) |
| GET | `/settings` (admin) | | `Settings` |
| PUT | `/settings` (admin) | partial `SettingsUpdate` | `Settings` |
| GET | `/admin/activity?page=&limit=` (admin) | | `Page<Activity>` newest first |

```ts
interface Settings {
  tmdbApiKeySet: boolean;        // the key itself is never returned
  openLibraryEnabled: boolean;   // look up book/audiobook metadata + covers on openlibrary.org
  scanIntervalMinutes: number;   // 0 = no automatic rescans
}
interface SettingsUpdate {
  tmdbApiKey?: string;           // "" clears it
  openLibraryEnabled?: boolean;
  scanIntervalMinutes?: number;
}
interface Activity {
  id: number;
  at: string;
  userId: number | null;
  username: string;
  action: "download_file" | "download_item" | "login";
  itemId: number | null;
  itemTitle: string | null;
  detail: string | null;         // file name / "Season 2" / ip
  size: number | null;
}
```
