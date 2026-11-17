// Dev-only in-browser mock of the Trove API (enabled with VITE_MOCK=1 npm run dev).
// Patches window.fetch for /api/v1/* and answers from in-memory fixtures.
//
// Accounts: admin / sam / guest (browse-only), password "password123".
// URL switches (append to any page once): ?mock=setup (fresh install), ?mock=empty (no libraries),
// ?mock=reset (back to defaults).
import type { ItemDetail, ItemSummary, Library, LibraryType, Settings, User } from '../api/types'
import { activityLog, fsTree, items as fixtureItems, libraries as fixtureLibs, MOCK_PASSWORD, recomputeLibraryStats, users as fixtureUsers } from './fixtures'

const SESSION_KEY = 'trove-mock-session'
const MODE_KEY = 'trove-mock-mode'

class HttpError extends Error {
  readonly status: number
  readonly code: string
  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

const state = {
  users: structuredClone(fixtureUsers),
  libraries: structuredClone(fixtureLibs),
  items: fixtureItems,
  settings: { tmdbApiKeySet: false, openLibraryEnabled: true, scanIntervalMinutes: 360 } as Settings,
  activity: activityLog(),
  failedLogins: 0,
  nextUserId: 10,
  nextLibId: 10,
}

function applyMode() {
  const url = new URL(window.location.href)
  const m = url.searchParams.get('mock')
  if (m) {
    if (m === 'reset') localStorage.removeItem(MODE_KEY)
    else localStorage.setItem(MODE_KEY, m)
    url.searchParams.delete('mock')
    window.history.replaceState(null, '', url)
    sessionStorage.removeItem(SESSION_KEY)
  }
  const mode = localStorage.getItem(MODE_KEY)
  if (mode === 'setup') {
    state.users = []
  } else if (mode === 'empty') {
    state.libraries = []
    state.items = []
  }
}

function currentUser(): User | null {
  const id = Number(sessionStorage.getItem(SESSION_KEY))
  return state.users.find((u) => u.id === id) ?? null
}

function requireUser(): User {
  const u = currentUser()
  if (!u) throw new HttpError(401, 'unauthorized', 'Not signed in')
  return u
}

function requireAdmin(): User {
  const u = requireUser()
  if (u.role !== 'admin') throw new HttpError(403, 'forbidden', 'Admins only')
  return u
}

const canSee = (u: User, libraryId: number) => u.libraryIds === null || u.libraryIds.includes(libraryId)

function summary(i: ItemDetail): ItemSummary {
  return {
    id: i.id,
    libraryId: i.libraryId,
    libraryType: i.libraryType,
    kind: i.kind,
    title: i.title,
    year: i.year,
    author: i.author,
    posterUrl: i.posterUrl,
    rating: i.rating,
    addedAt: i.addedAt,
    size: i.size,
    fileCount: i.fileCount,
    seasonCount: i.seasonCount,
    episodeCount: i.episodeCount,
  }
}

function visibleItems(u: User): ItemDetail[] {
  const libIds = new Set(state.libraries.map((l) => l.id))
  return state.items.filter((i) => libIds.has(i.libraryId) && canSee(u, i.libraryId))
}

function visibleLibs(u: User): Library[] {
  return state.libraries
    .filter((l) => canSee(u, l.id))
    .map((l) => (u.role === 'admin' ? l : { ...l, path: null }))
    .sort((a, b) => a.name.localeCompare(b.name))
}

// ---- scan simulation ----
const timers = new Map<number, number>()
function startScan(lib: Library) {
  if (lib.scan.state === 'scanning' && timers.has(lib.id)) return
  lib.scan = { state: 'scanning', phase: 'walking', done: 0, total: 0, error: null, startedAt: new Date().toISOString(), finishedAt: lib.scan.finishedAt }
  runScan(lib)
}
function runScan(lib: Library) {
  window.clearInterval(timers.get(lib.id))
  const t = window.setInterval(() => {
    const s = lib.scan
    const n = Math.max(lib.itemCount, 4)
    if (s.phase === 'walking') {
      s.done += 7
      if (s.done > n * 6) Object.assign(s, { phase: 'indexing', done: 0, total: n })
    } else if (s.phase === 'indexing') {
      s.done = Math.min(s.total, s.done + 2)
      if (s.done >= s.total) Object.assign(s, { phase: 'metadata', done: 0, total: n })
    } else if (s.phase === 'metadata') {
      s.done = Math.min(s.total, s.done + 1)
      if (s.done >= s.total) {
        lib.scan = { state: 'idle', phase: null, done: 0, total: 0, error: null, startedAt: s.startedAt, finishedAt: new Date().toISOString() }
        window.clearInterval(t)
        timers.delete(lib.id)
      }
    }
  }, 700)
  timers.set(lib.id, t)
}

// ---- routing ----
type Handler = (ctx: { params: string[]; query: URLSearchParams; body: unknown }) => unknown
const routes: [string, RegExp, Handler][] = []
const route = (method: string, pattern: string, h: Handler) =>
  routes.push([method, new RegExp(`^${pattern.replace(/:\w+/g, '([^/]+)')}$`), h])

interface Body {
  [k: string]: unknown
}
const asBody = (b: unknown): Body => (typeof b === 'object' && b !== null ? (b as Body) : {})

function findItem(u: User, id: number): ItemDetail {
  const it = visibleItems(u).find((i) => i.id === id)
  if (!it) throw new HttpError(404, 'not_found', 'Item not found')
  return it
}

route('GET', '/setup/status', () => ({ needsSetup: state.users.length === 0 }))
route('POST', '/setup', ({ body }) => {
  if (state.users.length) throw new HttpError(409, 'conflict', 'Setup has already been completed')
  const b = asBody(body)
  const user: User = { id: state.nextUserId++, username: String(b.username), role: 'admin', canDownload: true, libraryIds: null, createdAt: new Date().toISOString(), lastLoginAt: new Date().toISOString() }
  state.users.push(user)
  localStorage.removeItem(MODE_KEY)
  sessionStorage.setItem(SESSION_KEY, String(user.id))
  return { user }
})
route('POST', '/auth/login', ({ body }) => {
  const b = asBody(body)
  if (state.failedLogins >= 5) throw new HttpError(429, 'rate_limited', 'Too many login attempts, try again later')
  const u = state.users.find((x) => x.username.toLowerCase() === String(b.username).toLowerCase())
  if (!u || b.password !== MOCK_PASSWORD) {
    state.failedLogins++
    if (state.failedLogins >= 5) window.setTimeout(() => (state.failedLogins = 0), 30_000)
    throw new HttpError(401, 'unauthorized', 'Invalid username or password')
  }
  state.failedLogins = 0
  u.lastLoginAt = new Date().toISOString()
  sessionStorage.setItem(SESSION_KEY, String(u.id))
  return { user: u }
})
route('POST', '/auth/logout', () => {
  sessionStorage.removeItem(SESSION_KEY)
  return null
})
route('GET', '/auth/me', () => ({ user: requireUser() }))
route('PUT', '/auth/password', ({ body }) => {
  requireUser()
  const b = asBody(body)
  if (b.currentPassword !== MOCK_PASSWORD) throw new HttpError(400, 'invalid_request', 'Current password is incorrect')
  if (String(b.newPassword).length < 8) throw new HttpError(400, 'invalid_request', 'Password must be at least 8 characters')
  return null
})

route('GET', '/users', () => (requireAdmin(), { users: state.users }))
route('POST', '/users', ({ body }) => {
  requireAdmin()
  const b = asBody(body)
  const username = String(b.username)
  if (state.users.some((u) => u.username.toLowerCase() === username.toLowerCase()))
    throw new HttpError(409, 'conflict', `The username “${username}” is already taken`)
  const role = b.role === 'admin' ? 'admin' : 'user'
  const user: User = {
    id: state.nextUserId++,
    username,
    role,
    canDownload: !!b.canDownload,
    libraryIds: role === 'admin' ? null : (b.libraryIds as number[] | null),
    createdAt: new Date().toISOString(),
    lastLoginAt: null,
  }
  state.users.push(user)
  return { user }
})
route('PATCH', '/users/:id', ({ params, body }) => {
  requireAdmin()
  const u = state.users.find((x) => x.id === Number(params[0]))
  if (!u) throw new HttpError(404, 'not_found', 'User not found')
  const b = asBody(body)
  if (b.role === 'user' && u.role === 'admin' && state.users.filter((x) => x.role === 'admin').length <= 1)
    throw new HttpError(409, 'conflict', 'The last admin can’t be demoted. Make someone else an admin first.')
  if (typeof b.username === 'string') {
    if (state.users.some((x) => x.id !== u.id && x.username.toLowerCase() === String(b.username).toLowerCase()))
      throw new HttpError(409, 'conflict', `The username “${b.username}” is already taken`)
    u.username = b.username
  }
  if (b.role === 'admin' || b.role === 'user') u.role = b.role
  if (typeof b.canDownload === 'boolean') u.canDownload = b.canDownload
  if ('libraryIds' in b) u.libraryIds = b.libraryIds as number[] | null
  if (u.role === 'admin') u.libraryIds = null
  return { user: u }
})
route('DELETE', '/users/:id', ({ params }) => {
  const me = requireAdmin()
  const id = Number(params[0])
  const u = state.users.find((x) => x.id === id)
  if (!u) throw new HttpError(404, 'not_found', 'User not found')
  if (u.id === me.id) throw new HttpError(409, 'conflict', 'You can’t delete your own account')
  if (u.role === 'admin' && state.users.filter((x) => x.role === 'admin').length <= 1)
    throw new HttpError(409, 'conflict', 'The last admin can’t be deleted')
  state.users = state.users.filter((x) => x.id !== id)
  return null
})

route('GET', '/libraries', () => ({ libraries: visibleLibs(requireUser()) }))
route('POST', '/libraries', ({ body }) => {
  requireAdmin()
  const b = asBody(body)
  const path = String(b.path)
  if (!(path in fsTree) && !Object.values(fsTree).some((d) => d.some((n) => path.endsWith(`/${n}`))))
    throw new HttpError(400, 'invalid_request', `${path} is not an existing directory`)
  const lib: Library = {
    id: state.nextLibId++,
    name: String(b.name),
    type: b.type as LibraryType,
    path,
    itemCount: 0,
    totalSize: 0,
    scan: { state: 'idle', phase: null, done: 0, total: 0, error: null, startedAt: null, finishedAt: null },
    createdAt: new Date().toISOString(),
  }
  state.libraries.push(lib)
  startScan(lib)
  return { library: lib }
})
route('PATCH', '/libraries/:id', ({ params, body }) => {
  requireAdmin()
  const lib = state.libraries.find((l) => l.id === Number(params[0]))
  if (!lib) throw new HttpError(404, 'not_found', 'Library not found')
  const b = asBody(body)
  if (typeof b.name === 'string') lib.name = b.name
  let rescan = false
  if (typeof b.type === 'string' && b.type !== lib.type) {
    lib.type = b.type as LibraryType
    rescan = true
  }
  if (typeof b.path === 'string' && b.path !== lib.path) {
    lib.path = b.path
    rescan = true
  }
  if (rescan) startScan(lib)
  return { library: lib }
})
route('DELETE', '/libraries/:id', ({ params }) => {
  requireAdmin()
  state.libraries = state.libraries.filter((l) => l.id !== Number(params[0]))
  return null
})
route('POST', '/libraries/:id/scan', ({ params }) => {
  requireAdmin()
  const lib = state.libraries.find((l) => l.id === Number(params[0]))
  if (!lib) throw new HttpError(404, 'not_found', 'Library not found')
  startScan(lib)
  return { library: lib }
})
route('POST', '/libraries/scan', () => {
  requireAdmin()
  state.libraries.forEach(startScan)
  return { libraries: visibleLibs(requireUser()) }
})
route('GET', '/admin/fs', ({ query }) => {
  requireAdmin()
  const path = query.get('path') || '/'
  const norm = path.length > 1 ? path.replace(/\/+$/, '') : path
  const dirs = fsTree[norm]
  if (!dirs && norm !== '/') {
    // unknown leaf folders are empty, unless they look made up
    const parent = norm.slice(0, norm.lastIndexOf('/')) || '/'
    const name = norm.slice(norm.lastIndexOf('/') + 1)
    if (!fsTree[parent]?.includes(name)) throw new HttpError(400, 'invalid_request', `${norm} is not a directory`)
  }
  const parent = norm === '/' ? null : norm.slice(0, norm.lastIndexOf('/')) || '/'
  return {
    path: norm,
    parent,
    dirs: (dirs ?? []).map((name) => ({ name, path: `${norm === '/' ? '' : norm}/${name}` })),
  }
})

route('GET', '/home', () => {
  const u = requireUser()
  const vis = visibleItems(u).sort((a, b) => b.addedAt.localeCompare(a.addedAt))
  return {
    recentlyAdded: vis.slice(0, 20).map(summary),
    libraries: visibleLibs(u).map((library) => ({
      library,
      recent: vis.filter((i) => i.libraryId === library.id).slice(0, 20).map(summary),
    })),
  }
})
route('GET', '/items', ({ query }) => {
  const u = requireUser()
  let list = visibleItems(u)
  const lib = query.get('library')
  if (lib) list = list.filter((i) => i.libraryId === Number(lib))
  const kind = query.get('kind')
  if (kind) list = list.filter((i) => i.kind === kind)
  const q = query.get('q')?.toLowerCase()
  if (q)
    list = list.filter((i) =>
      [i.title, i.originalTitle, i.author, i.series].some((f) => f && f.toLowerCase().includes(q)),
    )
  const genre = query.get('genre')
  if (genre) list = list.filter((i) => i.genres.includes(genre))
  const sort = query.get('sort') ?? 'title'
  const order = query.get('order') ?? (sort === 'title' ? 'asc' : 'desc')
  const key = (i: ItemDetail): string | number =>
    sort === 'added' ? i.addedAt : sort === 'year' ? (i.year ?? 0) : sort === 'rating' ? (i.rating ?? -1) : sort === 'size' ? i.size : i.title.toLowerCase().replace(/^(the|a|an) /, '')
  list = [...list].sort((a, b) => {
    const ka = key(a)
    const kb = key(b)
    const c = typeof ka === 'number' && typeof kb === 'number' ? ka - kb : String(ka).localeCompare(String(kb))
    return order === 'asc' ? c : -c
  })
  const page = Math.max(1, Number(query.get('page')) || 1)
  const limit = Math.min(100, Math.max(1, Number(query.get('limit')) || 48))
  return { items: list.slice((page - 1) * limit, page * limit).map(summary), total: list.length, page, limit }
})
route('GET', '/items/:id', ({ params }) => findItem(requireUser(), Number(params[0])))
route('GET', '/genres', ({ query }) => {
  const u = requireUser()
  const lib = query.get('library')
  const set = new Set<string>()
  for (const i of visibleItems(u)) if (!lib || i.libraryId === Number(lib)) i.genres.forEach((g) => set.add(g))
  return { genres: [...set].sort() }
})
route('POST', '/items/:id/refresh', ({ params }) => {
  requireAdmin()
  return findItem(requireUser(), Number(params[0]))
})

route('GET', '/settings', () => (requireAdmin(), state.settings))
route('PUT', '/settings', ({ body }) => {
  requireAdmin()
  const b = asBody(body)
  if (typeof b.tmdbApiKey === 'string') state.settings.tmdbApiKeySet = b.tmdbApiKey !== ''
  if (typeof b.openLibraryEnabled === 'boolean') state.settings.openLibraryEnabled = b.openLibraryEnabled
  if (typeof b.scanIntervalMinutes === 'number') state.settings.scanIntervalMinutes = b.scanIntervalMinutes
  return state.settings
})
route('GET', '/admin/activity', ({ query }) => {
  requireAdmin()
  const page = Math.max(1, Number(query.get('page')) || 1)
  const limit = Math.min(100, Math.max(1, Number(query.get('limit')) || 50))
  return { items: state.activity.slice((page - 1) * limit, page * limit), total: state.activity.length, page, limit }
})

const json = (status: number, data: unknown) =>
  new Response(data === null ? null : JSON.stringify(data), {
    status: data === null && status === 200 ? 204 : status,
    headers: { 'Content-Type': 'application/json' },
  })

export function installMock() {
  applyMode()
  // resume the fixture library that starts mid-scan
  state.libraries.filter((l) => l.scan.state === 'scanning').forEach(runScan)
  recomputeLibraryStats(state.libraries, state.items)

  const realFetch = window.fetch.bind(window)
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, window.location.origin)
    if (!url.pathname.startsWith('/api/v1/')) return realFetch(input, init)
    const method = (init?.method ?? 'GET').toUpperCase()
    const path = url.pathname.slice('/api/v1'.length)
    await new Promise((r) => setTimeout(r, 120 + Math.random() * 330))

    const headers = new Headers(init?.headers)
    if (method !== 'GET' && headers.get('X-Requested-With') !== 'trove')
      return json(403, { error: { code: 'csrf', message: 'Missing X-Requested-With header' } })

    let body: unknown = undefined
    if (typeof init?.body === 'string') body = JSON.parse(init.body)

    for (const [m, re, h] of routes) {
      if (m !== method) continue
      const match = re.exec(path)
      if (!match) continue
      try {
        const out = h({ params: match.slice(1), query: url.searchParams, body })
        const status = method === 'POST' && /^\/(users|libraries)$/.test(path) ? 201 : /scan$/.test(path) ? 202 : 200
        return json(out === null ? 200 : status, out ?? null)
      } catch (e) {
        if (e instanceof HttpError) return json(e.status, { error: { code: e.code, message: e.message } })
        console.error('[mock]', e)
        return json(500, { error: { code: 'internal', message: 'Mock handler crashed' } })
      }
    }
    return json(404, { error: { code: 'not_found', message: `No mock for ${method} ${path}` } })
  }
  console.info('%c[trove] API mock enabled', 'color:#fbb94f', '— sign in as admin / sam / guest with "password123"')
}
