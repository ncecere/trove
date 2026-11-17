import type {
  Activity,
  ApiErrorBody,
  Credentials,
  FsListing,
  HomeData,
  ItemDetail,
  ItemQuery,
  ItemSummary,
  Library,
  LibraryCreate,
  LibraryUpdate,
  Page,
  Settings,
  SettingsUpdate,
  SetupStatus,
  User,
  UserCreate,
  UserUpdate,
} from './types'

export const API_BASE = '/api/v1'

/** A typed error parsed from the API's `{ error: { code, message } }` envelope. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export function isApiError(e: unknown): e is ApiError {
  return e instanceof ApiError
}

/** Human message for any thrown value. */
export function errorMessage(e: unknown): string {
  if (isApiError(e)) return e.message
  if (e instanceof Error) return e.message
  return 'Something went wrong'
}

type UnauthorizedHandler = () => void
let onUnauthorized: UnauthorizedHandler | null = null

/** Registered by the app: called whenever an authenticated call returns 401. */
export function setUnauthorizedHandler(fn: UnauthorizedHandler | null): void {
  onUnauthorized = fn
}

function isErrorBody(v: unknown): v is ApiErrorBody {
  if (typeof v !== 'object' || v === null || !('error' in v)) return false
  const err = (v as { error: unknown }).error
  return typeof err === 'object' && err !== null && 'code' in err && 'message' in err
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  signal?: AbortSignal
  /** Don't trigger the global "session expired" handler on 401 (login, me). */
  quiet401?: boolean
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const method = opts.method ?? 'GET'
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (method !== 'GET') headers['X-Requested-With'] = 'trove'
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'

  let res: Response
  try {
    res = await fetch(API_BASE + path, {
      method,
      headers,
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
      credentials: 'same-origin',
      signal: opts.signal,
    })
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    throw new ApiError(0, 'network', 'Can’t reach the Trove server. Check your connection.')
  }

  if (res.status === 204) return undefined as T

  const text = await res.text()
  let data: unknown = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }

  if (!res.ok) {
    if (res.status === 401 && !opts.quiet401) onUnauthorized?.()
    if (isErrorBody(data)) throw new ApiError(res.status, data.error.code, data.error.message)
    throw new ApiError(res.status, res.status >= 500 ? 'internal' : 'http_error', `Request failed (${res.status})`)
  }
  return data as T
}

function qs(params: Record<string, string | number | undefined | null>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') sp.set(k, String(v))
  }
  const s = sp.toString()
  return s ? `?${s}` : ''
}

// ---- URL builders for plain navigations (downloads / streams never go through fetch) ----

export const urls = {
  fileDownload: (fileId: number) => `${API_BASE}/files/${fileId}/download`,
  fileStream: (fileId: number) => `${API_BASE}/files/${fileId}/stream`,
  itemDownload: (itemId: number, season?: number) =>
    `${API_BASE}/items/${itemId}/download${season !== undefined ? `?season=${season}` : ''}`,
}

// ---- Endpoints ----

export const api = {
  setupStatus: (signal?: AbortSignal) => request<SetupStatus>('/setup/status', { signal, quiet401: true }),
  setup: (body: Credentials) =>
    request<{ user: User }>('/setup', { method: 'POST', body, quiet401: true }).then((r) => r.user),

  login: (body: Credentials) =>
    request<{ user: User }>('/auth/login', { method: 'POST', body, quiet401: true }).then((r) => r.user),
  logout: () => request<void>('/auth/logout', { method: 'POST', quiet401: true }),
  me: async (signal?: AbortSignal): Promise<User | null> => {
    try {
      return (await request<{ user: User }>('/auth/me', { signal, quiet401: true })).user
    } catch (e) {
      if (isApiError(e) && e.status === 401) return null
      throw e
    }
  },
  changePassword: (currentPassword: string, newPassword: string) =>
    // quiet401: the contract doesn't say which status a wrong current password returns; a 401 here
    // must not be mistaken for an expired session.
    request<void>('/auth/password', { method: 'PUT', body: { currentPassword, newPassword }, quiet401: true }),

  users: (signal?: AbortSignal) => request<{ users: User[] }>('/users', { signal }).then((r) => r.users),
  createUser: (body: UserCreate) => request<{ user: User }>('/users', { method: 'POST', body }).then((r) => r.user),
  updateUser: (id: number, body: UserUpdate) =>
    request<{ user: User }>(`/users/${id}`, { method: 'PATCH', body }).then((r) => r.user),
  deleteUser: (id: number) => request<void>(`/users/${id}`, { method: 'DELETE' }),

  libraries: (signal?: AbortSignal) =>
    request<{ libraries: Library[] }>('/libraries', { signal }).then((r) => r.libraries),
  createLibrary: (body: LibraryCreate) =>
    request<{ library: Library }>('/libraries', { method: 'POST', body }).then((r) => r.library),
  updateLibrary: (id: number, body: LibraryUpdate) =>
    request<{ library: Library }>(`/libraries/${id}`, { method: 'PATCH', body }).then((r) => r.library),
  deleteLibrary: (id: number) => request<void>(`/libraries/${id}`, { method: 'DELETE' }),
  scanLibrary: (id: number) =>
    request<{ library: Library }>(`/libraries/${id}/scan`, { method: 'POST' }).then((r) => r.library),
  scanAll: () => request<{ libraries: Library[] }>('/libraries/scan', { method: 'POST' }).then((r) => r.libraries),
  fs: (path: string, signal?: AbortSignal) => request<FsListing>(`/admin/fs${qs({ path })}`, { signal }),

  home: (signal?: AbortSignal) => request<HomeData>('/home', { signal }),
  items: (q: ItemQuery, signal?: AbortSignal) =>
    request<Page<ItemSummary>>(
      `/items${qs({
        library: q.library,
        kind: q.kind,
        q: q.q,
        genre: q.genre,
        sort: q.sort,
        order: q.order,
        page: q.page,
        limit: q.limit,
      })}`,
      { signal },
    ),
  item: (id: number, signal?: AbortSignal) => request<ItemDetail>(`/items/${id}`, { signal }),
  genres: (library: number | undefined, signal?: AbortSignal) =>
    request<{ genres: string[] }>(`/genres${qs({ library })}`, { signal }).then((r) => r.genres),
  refreshItem: (id: number) => request<ItemDetail>(`/items/${id}/refresh`, { method: 'POST' }),

  settings: (signal?: AbortSignal) => request<Settings>('/settings', { signal }),
  updateSettings: (body: SettingsUpdate) => request<Settings>('/settings', { method: 'PUT', body }),
  activity: (page: number, limit: number, signal?: AbortSignal) =>
    request<Page<Activity>>(`/admin/activity${qs({ page, limit })}`, { signal }),
}
