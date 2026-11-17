import type { Episode, Kind, LibraryType, MediaFile } from '../api/types'

export function formatBytes(bytes: number | null | undefined, digits = 1): string {
  if (bytes === null || bytes === undefined || !Number.isFinite(bytes)) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB', 'PB']
  let v = bytes / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  const d = v >= 100 || i === 0 ? 0 : digits
  return `${v.toFixed(d)} ${units[i]}`
}

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
const DIVS: [number, Intl.RelativeTimeFormatUnit][] = [
  [60, 'second'],
  [60, 'minute'],
  [24, 'hour'],
  [7, 'day'],
  [4.34524, 'week'],
  [12, 'month'],
  [Number.POSITIVE_INFINITY, 'year'],
]

export function relativeTime(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return 'never'
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return '—'
  let delta = (t - now) / 1000
  if (Math.abs(delta) < 45) return 'just now'
  for (const [amount, unit] of DIVS) {
    if (Math.abs(delta) < amount) return rtf.format(Math.round(delta), unit)
    delta /= amount
  }
  return '—'
}

const dateFmt = new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
const dateTimeFmt = new Intl.DateTimeFormat(undefined, {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

/** Formats "YYYY-MM-DD" (treated as a calendar date, not UTC midnight) or an RFC 3339 timestamp. */
export function formatDate(s: string | null | undefined): string {
  if (!s) return '—'
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s)
  if (m) return dateFmt.format(new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])))
  const t = Date.parse(s)
  return Number.isNaN(t) ? s : dateFmt.format(new Date(t))
}

export function formatDateTime(s: string | null | undefined): string {
  if (!s) return '—'
  const t = Date.parse(s)
  return Number.isNaN(t) ? s : dateTimeFmt.format(new Date(t))
}

export function formatRuntime(min: number | null | undefined): string | null {
  if (!min || min <= 0) return null
  const h = Math.floor(min / 60)
  const m = min % 60
  if (!h) return `${m}m`
  return m ? `${h}h ${m}m` : `${h}h`
}

export function formatRating(r: number | null | undefined): string | null {
  if (r === null || r === undefined) return null
  return r.toFixed(1)
}

export const pluralize = (n: number, one: string, many = `${one}s`) => `${n.toLocaleString()} ${n === 1 ? one : many}`

export function seasonLabel(n: number): string {
  return n === 0 ? 'Specials' : `Season ${n}`
}

export function episodeCode(e: Episode): string {
  const s = String(e.season).padStart(2, '0')
  const a = String(e.episode).padStart(2, '0')
  const b = e.episodeEnd && e.episodeEnd !== e.episode ? `–E${String(e.episodeEnd).padStart(2, '0')}` : ''
  return `S${s}E${a}${b}`
}

export const LIBRARY_TYPE_LABEL: Record<LibraryType, string> = {
  movies: 'Movies',
  tv: 'TV Shows',
  anime: 'Anime',
  books: 'Books',
  audiobooks: 'Audiobooks',
}

export const KIND_LABEL: Record<Kind, { one: string; many: string }> = {
  movie: { one: 'Movie', many: 'Movies' },
  show: { one: 'Show', many: 'Shows' },
  book: { one: 'Book', many: 'Books' },
  audiobook: { one: 'Audiobook', many: 'Audiobooks' },
}

/** Square artwork for audiobooks, 2:3 for everything else. */
export const isSquareArt = (kind: Kind) => kind === 'audiobook'

const VIDEO_PLAYABLE = new Set(['mp4', 'webm', 'm4v'])
const AUDIO_PLAYABLE = new Set(['mp3', 'm4a', 'm4b', 'ogg', 'opus', 'flac', 'oga', 'aac', 'wav'])

export function playableAs(f: MediaFile): 'video' | 'audio' | null {
  const c = f.container.toLowerCase()
  if ((f.role === 'video' || f.role === 'extra') && VIDEO_PLAYABLE.has(c)) return 'video'
  if ((f.role === 'audio' || f.role === 'extra') && AUDIO_PLAYABLE.has(c)) return 'audio'
  return null
}

const RES_ORDER: Record<string, number> = { '2160p': 4, '4k': 4, '1440p': 3, '1080p': 2, '720p': 1 }

export function resolutionRank(r: string | null): number {
  if (!r) return 0
  return RES_ORDER[r.toLowerCase()] ?? 0.5
}

/** Badge labels for a video file: resolution, HDR, source, codec, edition. */
export function videoBadges(f: MediaFile): string[] {
  const out: string[] = []
  if (f.resolution) out.push(f.resolution.toLowerCase() === '2160p' ? '4K' : f.resolution)
  if (f.hdr) out.push(f.hdr)
  if (f.source) out.push(f.source)
  if (f.videoCodec) out.push(f.videoCodec.toUpperCase())
  if (f.audioCodec) out.push(f.audioCodec.toUpperCase())
  return out
}

export function languageName(code: string | null): string | null {
  if (!code) return null
  try {
    const dn = new Intl.DisplayNames(undefined, { type: 'language' })
    return dn.of(code) ?? code
  } catch {
    return code
  }
}

/** Stable 32-bit string hash (FNV-1a). */
export function hashString(s: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  return h >>> 0
}

export const LARGE_ZIP_BYTES = 20 * 1024 ** 3
