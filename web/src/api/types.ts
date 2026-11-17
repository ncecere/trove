// Types copied from docs/API.md (Trove HTTP API v1). Keep in sync with the contract.

export type Role = 'admin' | 'user'
export type LibraryType = 'movies' | 'tv' | 'anime' | 'books' | 'audiobooks'
export type Kind = 'movie' | 'show' | 'book' | 'audiobook'
export type FileRole = 'video' | 'audio' | 'ebook' | 'subtitle' | 'extra'

export interface User {
  id: number
  username: string
  role: Role
  canDownload: boolean // false = browse only
  libraryIds: number[] | null // null = access to all libraries (admins are always null)
  createdAt: string
  lastLoginAt: string | null
}

export interface ScanStatus {
  state: 'idle' | 'scanning' | 'error'
  phase: 'walking' | 'indexing' | 'metadata' | null // null when idle
  done: number // items processed in the current phase
  total: number // 0 when unknown
  error: string | null // last error, when state == "error"
  startedAt: string | null
  finishedAt: string | null // last completed scan
}

export interface Library {
  id: number
  name: string
  type: LibraryType
  path: string | null // absolute path on the server; null for non-admins
  itemCount: number
  totalSize: number
  scan: ScanStatus
  createdAt: string
}

export interface ItemSummary {
  id: number
  libraryId: number
  libraryType: LibraryType
  kind: Kind
  title: string
  year: number | null
  author: string | null // books / audiobooks
  posterUrl: string | null // e.g. "/api/v1/items/12/poster?v=1731700000"
  rating: number | null // 0-10
  addedAt: string
  size: number // total bytes of all files
  fileCount: number
  seasonCount: number | null // shows only
  episodeCount: number | null // shows only
}

export interface MediaFile {
  id: number
  name: string // base filename
  relPath: string // relative to the item folder (for display)
  size: number
  role: FileRole
  container: string // "mkv", "epub", ...
  resolution: string | null
  source: string | null
  videoCodec: string | null
  audioCodec: string | null
  hdr: string | null
  edition: string | null
  language: string | null
  track: number | null // audiobook part order
  modifiedAt: string
}

export interface Episode {
  season: number // 0 = specials
  episode: number
  episodeEnd: number | null // multi-episode files
  title: string | null
  overview: string | null
  airDate: string | null // YYYY-MM-DD
  file: MediaFile // the video file
  subtitles: MediaFile[]
}

export interface Season {
  season: number // 0 = specials
  episodeCount: number
  size: number
  episodes: Episode[] // sorted by episode
}

export interface ItemDetail extends ItemSummary {
  originalTitle: string | null
  tagline: string | null
  overview: string | null
  genres: string[]
  runtimeMinutes: number | null
  releaseDate: string | null // YYYY-MM-DD
  status: string | null
  studio: string | null
  narrator: string | null
  series: string | null
  seriesIndex: number | null
  backdropUrl: string | null
  externalIds: Record<string, string> // tmdb, imdb, tvdb, isbn, asin, openlibrary
  files: MediaFile[] // every file that is NOT an episode (or episode subtitle); sorted by role, track, name
  seasons: Season[] // shows only, else []
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  limit: number
}

export interface Settings {
  tmdbApiKeySet: boolean // the key itself is never returned
  openLibraryEnabled: boolean // look up book/audiobook metadata + covers on openlibrary.org
  scanIntervalMinutes: number // 0 = no automatic rescans
}

export interface SettingsUpdate {
  tmdbApiKey?: string // "" clears it
  openLibraryEnabled?: boolean
  scanIntervalMinutes?: number
}

export interface Activity {
  id: number
  at: string
  userId: number | null
  username: string
  action: 'download_file' | 'download_item' | 'login'
  itemId: number | null
  itemTitle: string | null
  detail: string | null // file name / "Season 2" / ip
  size: number | null
}

// ---- Request / response envelopes ----

export interface ApiErrorBody {
  error: { code: string; message: string }
}

export interface SetupStatus {
  needsSetup: boolean
}

export interface Credentials {
  username: string
  password: string
}

export interface UserCreate {
  username: string
  password: string
  role: Role
  canDownload: boolean
  libraryIds: number[] | null
}

export type UserUpdate = Partial<UserCreate>

export interface LibraryCreate {
  name: string
  type: LibraryType
  path: string
}

export type LibraryUpdate = Partial<LibraryCreate>

export interface FsListing {
  path: string
  parent: string | null
  dirs: { name: string; path: string }[]
}

export interface HomeData {
  recentlyAdded: ItemSummary[]
  libraries: { library: Library; recent: ItemSummary[] }[]
}

export type ItemSort = 'title' | 'added' | 'year' | 'rating' | 'size'
export type SortOrder = 'asc' | 'desc'

export interface ItemQuery {
  library?: number
  kind?: Kind
  q?: string
  genre?: string
  sort?: ItemSort
  order?: SortOrder
  page?: number
  limit?: number
}
