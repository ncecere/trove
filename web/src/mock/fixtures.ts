// Realistic-looking fixture data for the dev mock. Dev only — never shipped.
import type { Activity, Episode, ItemDetail, Library, LibraryType, MediaFile, Season, User } from '../api/types'
import { backdropArt, posterArt, squareArt } from './art'

const GB = 1024 ** 3
const MB = 1024 ** 2
const now = Date.now()
const daysAgo = (d: number) => new Date(now - d * 86_400_000).toISOString()

let fileId = 1000
function file(p: Partial<MediaFile> & Pick<MediaFile, 'name' | 'size' | 'role' | 'container'>): MediaFile {
  return {
    id: fileId++,
    relPath: p.relPath ?? p.name,
    resolution: null,
    source: null,
    videoCodec: null,
    audioCodec: null,
    hdr: null,
    edition: null,
    language: null,
    track: null,
    modifiedAt: daysAgo(30),
    ...p,
  }
}

const idle = (finishedDaysAgo = 0.2): Library['scan'] => ({
  state: 'idle',
  phase: null,
  done: 0,
  total: 0,
  error: null,
  startedAt: daysAgo(finishedDaysAgo + 0.01),
  finishedAt: daysAgo(finishedDaysAgo),
})

export const libraries: Library[] = [
  { id: 1, name: 'Movies', type: 'movies', path: '/mnt/media/movies', itemCount: 0, totalSize: 0, scan: idle(), createdAt: daysAgo(400) },
  { id: 2, name: 'TV Shows', type: 'tv', path: '/mnt/media/tv', itemCount: 0, totalSize: 0, scan: idle(0.5), createdAt: daysAgo(400) },
  {
    id: 3,
    name: 'Anime',
    type: 'anime',
    path: '/mnt/media/anime',
    itemCount: 0,
    totalSize: 0,
    scan: { state: 'scanning', phase: 'metadata', done: 3, total: 9, error: null, startedAt: daysAgo(0.001), finishedAt: daysAgo(2) },
    createdAt: daysAgo(200),
  },
  { id: 4, name: 'Books', type: 'books', path: '/mnt/media/books', itemCount: 0, totalSize: 0, scan: idle(1), createdAt: daysAgo(380) },
  {
    id: 5,
    name: 'Audiobooks',
    type: 'audiobooks',
    path: '/mnt/media/audiobooks',
    itemCount: 0,
    totalSize: 0,
    scan: { ...idle(3), state: 'error', error: 'permission denied: /mnt/media/audiobooks/Unsorted' },
    createdAt: daysAgo(300),
  },
]

let itemId = 1
interface Base {
  title: string
  year?: number
  rating?: number
  art?: boolean
  backdrop?: boolean
  genres?: string[]
  overview?: string
  tagline?: string
  added: number
  runtime?: number
  studio?: string
  author?: string
  narrator?: string
  series?: string
  seriesIndex?: number
  originalTitle?: string
  status?: string
  externalIds?: Record<string, string>
}

const libType = (libraryId: number): LibraryType => libraries.find((l) => l.id === libraryId)!.type

function detail(libraryId: number, kind: ItemDetail['kind'], b: Base, files: MediaFile[], seasons: Season[] = []): ItemDetail {
  const id = itemId++
  const epSize = seasons.reduce((n, s) => n + s.size, 0)
  const epCount = seasons.reduce((n, s) => n + s.episodes.length + s.episodes.reduce((m, e) => m + e.subtitles.length, 0), 0)
  const art = b.art ?? true
  return {
    id,
    libraryId,
    libraryType: libType(libraryId),
    kind,
    title: b.title,
    year: b.year ?? null,
    author: b.author ?? null,
    posterUrl: art ? (kind === 'audiobook' ? squareArt(b.title) : posterArt(b.title)) : null,
    rating: b.rating ?? null,
    addedAt: daysAgo(b.added),
    size: files.reduce((n, f) => n + f.size, 0) + epSize,
    fileCount: files.length + epCount,
    seasonCount: kind === 'show' ? seasons.filter((s) => s.season > 0).length : null,
    episodeCount: kind === 'show' ? seasons.reduce((n, s) => n + s.episodes.length, 0) : null,
    originalTitle: b.originalTitle ?? null,
    tagline: b.tagline ?? null,
    overview: b.overview ?? null,
    genres: b.genres ?? [],
    runtimeMinutes: b.runtime ?? null,
    releaseDate: b.year ? `${b.year}-0${(id % 9) + 1}-1${id % 9}` : null,
    status: b.status ?? null,
    studio: b.studio ?? null,
    narrator: b.narrator ?? null,
    series: b.series ?? null,
    seriesIndex: b.seriesIndex ?? null,
    backdropUrl: art && b.backdrop !== false && (kind === 'movie' || kind === 'show') ? backdropArt(b.title) : null,
    externalIds: b.externalIds ?? {},
    files,
    seasons,
  }
}

const LOREM = [
  'A quiet harbor town is shaken when a storm uncovers something the tide was meant to keep.',
  'Two estranged siblings drive across the country with their late father’s ashes and a map that makes no sense.',
  'An aging cartographer is hired to chart an island that appears on no map — and keeps moving.',
  'In a city that never sleeps, a night-shift dispatcher starts receiving calls from tomorrow.',
  'A washed-up magician takes one last job: making a whole train disappear.',
  'When the lights go out across the valley, the only radio still working picks up a voice nobody recognises.',
]
const ov = (i: number, long = false) =>
  long
    ? `${LOREM[i % LOREM.length]} ${LOREM[(i + 1) % LOREM.length]} What begins as a simple favour turns into a journey through old grudges, half-remembered promises and the kind of secrets small places keep for generations. Along the way they discover that the real map was never on paper.\n\nShot over three winters on the northern coast, the film is as much about the landscape as the people who refuse to leave it.`
    : LOREM[i % LOREM.length]

function video(name: string, size: number, o: Partial<MediaFile> = {}): MediaFile {
  return file({ name, size, role: 'video', container: name.split('.').pop()!, resolution: '1080p', source: 'BluRay', videoCodec: 'h264', audioCodec: 'ac3', ...o })
}
const sub = (base: string, lang: string) => file({ name: `${base}.${lang}.srt`, size: 80_000 + lang.length * 1000, role: 'subtitle', container: 'srt', language: lang })

// ---------------- Movies ----------------
const movies: ItemDetail[] = [
  detail(
    1,
    'movie',
    {
      title: 'The Lighthouse Keeper’s Daughter',
      year: 2021,
      rating: 8.1,
      genres: ['Drama', 'Mystery'],
      runtime: 132,
      added: 0.3,
      tagline: 'Some lights are meant to warn you away.',
      overview: ov(0, true),
      studio: 'Northlight Pictures',
      externalIds: { tmdb: '550123', imdb: 'tt9900123' },
    },
    [
      video('The Lighthouse Keeper’s Daughter (2021) - 2160p.mkv', 58.4 * GB, { resolution: '2160p', hdr: 'HDR10', videoCodec: 'hevc', audioCodec: 'truehd', source: 'UHD BluRay', relPath: 'The Lighthouse Keeper’s Daughter (2021) - 2160p.mkv' }),
      video('The Lighthouse Keeper’s Daughter (2021) - 1080p.mkv', 14.2 * GB, { audioCodec: 'dts' }),
      video('The Lighthouse Keeper’s Daughter (2021) - Director’s Cut 1080p.mp4', 9.8 * GB, { edition: 'Director’s Cut', container: 'mp4', source: 'WEB-DL', audioCodec: 'aac' }),
      sub('The Lighthouse Keeper’s Daughter (2021)', 'en'),
      sub('The Lighthouse Keeper’s Daughter (2021)', 'es'),
      sub('The Lighthouse Keeper’s Daughter (2021)', 'fr'),
      file({ name: 'Trailer.mp4', relPath: 'Extras/Trailer.mp4', size: 180 * MB, role: 'extra', container: 'mp4' }),
      file({ name: 'Behind the Lamp.mkv', relPath: 'Extras/Behind the Lamp.mkv', size: 1.4 * GB, role: 'extra', container: 'mkv' }),
    ],
  ),
  detail(1, 'movie', { title: 'Northbound', year: 2019, rating: 7.2, genres: ['Adventure', 'Drama'], runtime: 104, added: 2, overview: ov(1), tagline: 'The road remembers.', externalIds: { tmdb: '41122' } }, [
    video('Northbound (2019).mp4', 4.1 * GB, { container: 'mp4', source: 'WEB-DL', audioCodec: 'aac' }),
    sub('Northbound (2019)', 'en'),
  ]),
  detail(1, 'movie', { title: 'Echo Valley', year: 2023, rating: 6.4, genres: ['Thriller'], runtime: 97, added: 4, overview: ov(5) }, [
    video('Echo Valley (2023).mkv', 6.8 * GB),
  ]),
  detail(1, 'movie', { title: 'Velvet Hours', year: 2017, rating: 7.8, genres: ['Romance', 'Drama'], runtime: 118, added: 9, overview: ov(3), art: false }, [
    video('Velvet Hours (2017).mkv', 8.2 * GB),
  ]),
  detail(1, 'movie', { title: 'Midnight Ferry', year: 2015, rating: 6.9, genres: ['Mystery', 'Thriller'], runtime: 101, added: 12, art: false }, [
    video('Midnight Ferry (2015).avi', 1.4 * GB, { resolution: '720p', source: 'DVD', videoCodec: 'xvid', audioCodec: 'mp3' }),
  ]),
  detail(
    1,
    'movie',
    {
      title: 'An Exceptionally Long Movie Title That Goes On and On: The Extended Director’s Edition',
      year: 2012,
      rating: 5.9,
      genres: ['Comedy'],
      runtime: 188,
      added: 14,
      art: false,
      overview: ov(4),
    },
    [video('An Exceptionally Long Movie Title (2012).mkv', 11.3 * GB)],
  ),
  detail(1, 'movie', { title: 'Copper Sky', year: 2020, rating: 7.4, genres: ['Science Fiction', 'Adventure'], runtime: 126, added: 18, overview: ov(2), tagline: 'Look up.' }, [
    video('Copper Sky (2020).mkv', 22.9 * GB, { resolution: '2160p', hdr: 'Dolby Vision', videoCodec: 'hevc', source: 'WEB-DL', audioCodec: 'eac3' }),
  ]),
  detail(1, 'movie', { title: 'Neon Tide', year: 2022, rating: 6.1, genres: ['Action', 'Science Fiction'], runtime: 111, added: 25, overview: ov(3) }, [
    video('Neon Tide (2022).mkv', 7.7 * GB),
  ]),
  detail(1, 'movie', { title: 'The Long Quiet', year: 2009, rating: 8.4, genres: ['Drama'], runtime: 142, added: 40, overview: ov(1, true), studio: 'Grey Owl Films' }, [
    video('The Long Quiet (2009).mkv', 12.1 * GB),
    sub('The Long Quiet (2009)', 'en'),
  ]),
  detail(1, 'movie', { title: 'Paper Lanterns', year: 2018, genres: ['Family', 'Animation'], runtime: 88, added: 60, art: false }, [
    video('Paper Lanterns (2018).m4v', 2.2 * GB, { container: 'm4v', source: 'WEB-DL', audioCodec: 'aac' }),
  ]),
  detail(1, 'movie', { title: 'Harvest Moon Motel', year: 2016, rating: 6.7, genres: ['Horror', 'Comedy'], runtime: 93, added: 75 }, [
    video('Harvest Moon Motel (2016).mkv', 5.5 * GB),
  ]),
  detail(1, 'movie', { title: 'Saltwater', year: 2024, rating: 7.0, genres: ['Drama'], runtime: 109, added: 1, art: false, overview: ov(0) }, [
    video('Saltwater (2024).mkv', 9.1 * GB),
  ]),
]

// ---------------- Shows ----------------
const EP_TITLES = ['Pilot', 'Low Tide', 'The Keeper', 'Breakwater', 'Fog Horn', 'Driftwood', 'The Crossing', 'Undertow', 'Safe Harbor', 'Landfall', 'Ebb', 'Northern Lights']

function seasonsFor(show: string, plan: number[], o: { specials?: number; container?: string; res?: string; subs?: boolean; epSize?: number } = {}): Season[] {
  const seasons: Season[] = []
  const all = o.specials ? [[0, o.specials] as const, ...plan.map((n, i) => [i + 1, n] as const)] : plan.map((n, i) => [i + 1, n] as const)
  for (const [s, count] of all) {
    const eps: Episode[] = []
    for (let e = 1; e <= count; e++) {
      const code = `S${String(s).padStart(2, '0')}E${String(e).padStart(2, '0')}`
      const t = s === 0 ? `Behind the Scenes ${e}` : EP_TITLES[(e - 1 + s) % EP_TITLES.length]
      const name = `${show} - ${code} - ${t}.${o.container ?? 'mkv'}`
      const isDouble = s === 1 && e === count && count > 6
      eps.push({
        season: s,
        episode: e,
        episodeEnd: isDouble ? e + 1 : null,
        title: s === 2 && e === 3 ? null : t,
        overview: e % 3 === 0 ? null : `${LOREM[(e + s) % LOREM.length]} ${e % 2 ? 'Meanwhile, an old friend returns with news nobody wanted to hear, and the town council meets in secret.' : ''}`,
        airDate: `${2018 + s}-${String(((e - 1) % 12) + 1).padStart(2, '0')}-${String(((e * 3) % 27) + 1).padStart(2, '0')}`,
        file: video(name, (o.epSize ?? 2.3 * GB) * (0.85 + ((e * 7) % 5) / 10), {
          relPath: `${s === 0 ? 'Specials' : `Season ${String(s).padStart(2, '0')}`}/${name}`,
          resolution: o.res ?? '1080p',
          container: o.container ?? 'mkv',
          source: 'WEB-DL',
          videoCodec: o.res === '2160p' ? 'hevc' : 'h264',
          hdr: o.res === '2160p' ? 'HDR10' : null,
          audioCodec: 'eac3',
        }),
        subtitles: o.subs ? [sub(`${show} - ${code}`, 'en'), ...(e % 2 ? [sub(`${show} - ${code}`, 'de')] : [])] : [],
      })
      if (isDouble) break
    }
    seasons.push({ season: s, episodeCount: eps.length, size: eps.reduce((n, x) => n + x.file.size + x.subtitles.reduce((m, y) => m + y.size, 0), 0), episodes: eps })
  }
  return seasons
}

const shows: ItemDetail[] = [
  detail(
    2,
    'show',
    {
      title: 'Harbor Lights',
      year: 2019,
      rating: 8.6,
      genres: ['Drama', 'Mystery'],
      added: 0.8,
      runtime: 52,
      status: 'Returning Series',
      studio: 'Channel North',
      tagline: 'Every town has a tide it can’t hold back.',
      overview: ov(0, true),
      externalIds: { tmdb: '84958', imdb: 'tt8111088', tvdb: '362472' },
    },
    [file({ name: 'Harbor Lights - Season 1 Recap.mkv', relPath: 'Extras/Harbor Lights - Season 1 Recap.mkv', size: 640 * MB, role: 'extra', container: 'mkv' })],
    seasonsFor('Harbor Lights', [10, 8, 9], { specials: 2, subs: true, epSize: 3.1 * GB }),
  ),
  detail(2, 'show', { title: 'Saltwater Kings', year: 2021, rating: 7.5, genres: ['Crime', 'Drama'], added: 3, status: 'Ended', overview: ov(4), runtime: 47 }, [], seasonsFor('Saltwater Kings', [6, 6], { epSize: 1.6 * GB })),
  detail(2, 'show', { title: 'The Ninth Wave', year: 2023, rating: 7.9, genres: ['Science Fiction', 'Thriller'], added: 6, runtime: 58, overview: ov(3) }, [], seasonsFor('The Ninth Wave', [8], { res: '2160p', epSize: 6.4 * GB, subs: true })),
  detail(2, 'show', { title: 'Kitchen Table', year: 2014, genres: ['Comedy'], added: 33, art: false, runtime: 24 }, [], seasonsFor('Kitchen Table', [12, 12, 12, 10], { container: 'mp4', res: '720p', epSize: 420 * MB })),
  detail(2, 'show', { title: 'Lanterns', year: 2020, rating: 6.8, genres: ['Fantasy', 'Drama'], added: 50, overview: ov(5) }, [], seasonsFor('Lanterns', [7], {})),
]

const anime: ItemDetail[] = [
  detail(3, 'show', { title: 'Starfall Academy', originalTitle: '星降る学園', year: 2022, rating: 8.2, genres: ['Animation', 'Fantasy', 'Action'], added: 1.5, runtime: 24, overview: ov(2), status: 'Returning Series' }, [], seasonsFor('Starfall Academy', [12, 12], { epSize: 1.1 * GB, subs: true })),
  detail(3, 'show', { title: 'Paper Moon Detective', originalTitle: '紙の月探偵', year: 2018, rating: 7.6, genres: ['Animation', 'Mystery'], added: 20, art: false, runtime: 23 }, [], seasonsFor('Paper Moon Detective', [24], { epSize: 900 * MB })),
  detail(3, 'movie', { title: 'The Wind Between Stars', originalTitle: '星間の風', year: 2020, rating: 8.8, genres: ['Animation', 'Adventure'], added: 8, runtime: 116, overview: ov(1, true) }, [
    video('The Wind Between Stars (2020).mkv', 7.9 * GB, { audioCodec: 'flac' }),
    sub('The Wind Between Stars (2020)', 'en'),
    sub('The Wind Between Stars (2020)', 'ja'),
  ]),
]

// ---------------- Books ----------------
const ebook = (title: string, container: string, size: number) => file({ name: `${title}.${container}`, size, role: 'ebook', container })
const books: ItemDetail[] = [
  detail(4, 'book', { title: 'A Field Guide to Rain', author: 'Mara Ellison', year: 2020, rating: 8.0, genres: ['Nature', 'Essays'], added: 0.5, overview: ov(5, true), studio: 'Wren & Finch', externalIds: { isbn: '9780000000011', openlibrary: 'OL123456W' } }, [
    ebook('A Field Guide to Rain', 'epub', 2.1 * MB),
    ebook('A Field Guide to Rain', 'pdf', 18.4 * MB),
    ebook('A Field Guide to Rain', 'mobi', 3.3 * MB),
  ]),
  detail(4, 'book', { title: 'The Cartographer', author: 'Idris Okafor', year: 2016, rating: 7.3, genres: ['Fiction', 'Literary'], added: 5, series: 'The Drowned Maps', seriesIndex: 1, overview: ov(2) }, [ebook('The Cartographer', 'epub', 1.4 * MB)]),
  detail(4, 'book', { title: 'The Cartographer’s Widow', author: 'Idris Okafor', year: 2018, genres: ['Fiction', 'Literary'], added: 5.1, series: 'The Drowned Maps', seriesIndex: 2, art: false }, [ebook('The Cartographer’s Widow', 'epub', 1.6 * MB), ebook('The Cartographer’s Widow', 'azw3', 2.0 * MB)]),
  detail(4, 'book', { title: 'Iron & Ivy', author: 'Celeste Varga', year: 2022, rating: 6.9, genres: ['Fantasy', 'Romance'], added: 11 }, [ebook('Iron & Ivy', 'epub', 900_000)]),
  detail(4, 'book', { title: 'Small Gods of Autumn', author: 'Teodor Lind', year: 2011, rating: 8.7, genres: ['Fantasy'], added: 22, art: false, overview: ov(3) }, [ebook('Small Gods of Autumn', 'pdf', 6.2 * MB)]),
  detail(4, 'book', { title: 'Practical Bread', author: 'Hanna Moritz', year: 2019, genres: ['Cooking'], added: 30 }, [ebook('Practical Bread', 'pdf', 88 * MB), ebook('Practical Bread', 'epub', 34 * MB)]),
  detail(4, 'book', { title: 'The Weight of Snowfall: A History of Northern Winters, Volume One', author: 'Professor Aurelio Benedetti-Lindqvist', year: 2005, genres: ['History'], added: 90, art: false }, [ebook('The Weight of Snowfall', 'epub', 4.4 * MB)]),
  detail(4, 'book', { title: 'Quiet Machines', author: 'Juno Park', year: 2023, rating: 7.7, genres: ['Science Fiction'], added: 2.5 }, [ebook('Quiet Machines', 'epub', 1.1 * MB), ebook('Quiet Machines', 'mobi', 1.9 * MB)]),
]

// ---------------- Audiobooks ----------------
function mp3Parts(title: string, n: number, part: number): MediaFile[] {
  return Array.from({ length: n }, (_, i) =>
    file({
      name: `${String(i + 1).padStart(2, '0')} - ${title} - Part ${i + 1}.mp3`,
      relPath: `${String(i + 1).padStart(2, '0')} - ${title} - Part ${i + 1}.mp3`,
      size: part * (0.9 + ((i * 13) % 7) / 30),
      role: 'audio',
      container: 'mp3',
      audioCodec: 'mp3',
      track: i + 1,
    }),
  )
}
const audiobooks: ItemDetail[] = [
  detail(5, 'audiobook', { title: 'Glass Garden', author: 'Mara Ellison', narrator: 'Priya Natarajan', year: 2021, rating: 8.3, genres: ['Fiction'], added: 0.2, overview: ov(4, true), series: 'Greenhouse Trilogy', seriesIndex: 1, studio: 'Lantern Audio', externalIds: { asin: 'B0EXAMPLE1' } }, [
    ...mp3Parts('Glass Garden', 14, 48 * MB),
    file({ name: 'cover.jpg', size: 420_000, role: 'extra', container: 'jpg' }),
    file({ name: 'Glass Garden - Companion.pdf', size: 3.2 * MB, role: 'ebook', container: 'pdf' }),
  ]),
  detail(5, 'audiobook', { title: 'Paper Moons', author: 'Teodor Lind', narrator: 'Sam Whitcombe', year: 2019, rating: 7.6, genres: ['Fantasy'], added: 7, overview: ov(2) }, [
    file({ name: 'Paper Moons.m4b', size: 612 * MB, role: 'audio', container: 'm4b', audioCodec: 'aac' }),
  ]),
  detail(5, 'audiobook', { title: 'The Patient Orchard', author: 'Hanna Moritz', narrator: 'Hanna Moritz', year: 2022, genres: ['Memoir'], added: 15, art: false }, [
    file({ name: 'The Patient Orchard.m4b', size: 388 * MB, role: 'audio', container: 'm4b', audioCodec: 'aac' }),
  ]),
  detail(5, 'audiobook', { title: 'Quiet Machines', author: 'Juno Park', narrator: 'Leo Tran', year: 2023, rating: 7.9, genres: ['Science Fiction'], added: 3 }, mp3Parts('Quiet Machines', 9, 61 * MB)),
  detail(5, 'audiobook', { title: 'Longitude of Small Things', author: 'Celeste Varga', narrator: 'Ines Duarte', year: 2017, genres: ['Fiction'], added: 44, art: false, series: 'Small Things', seriesIndex: 3 }, mp3Parts('Longitude', 6, 72 * MB)),
]

export const items: ItemDetail[] = [...movies, ...shows, ...anime, ...books, ...audiobooks]

export function recomputeLibraryStats(libs: Library[], all: ItemDetail[]) {
  for (const l of libs) {
    const its = all.filter((i) => i.libraryId === l.id)
    l.itemCount = its.length
    l.totalSize = its.reduce((n, i) => n + i.size, 0)
  }
}
recomputeLibraryStats(libraries, items)

export const users: User[] = [
  { id: 1, username: 'admin', role: 'admin', canDownload: true, libraryIds: null, createdAt: daysAgo(400), lastLoginAt: daysAgo(0.01) },
  { id: 2, username: 'sam', role: 'user', canDownload: true, libraryIds: null, createdAt: daysAgo(300), lastLoginAt: daysAgo(1.2) },
  { id: 3, username: 'guest', role: 'user', canDownload: false, libraryIds: [1, 4], createdAt: daysAgo(100), lastLoginAt: null },
]

export const MOCK_PASSWORD = 'password123'

export const fsTree: Record<string, string[]> = {
  '/': ['home', 'mnt', 'srv', 'volume1'],
  '/home': ['trove'],
  '/home/trove': ['Downloads'],
  '/mnt': ['media', 'backup'],
  '/mnt/media': ['anime', 'audiobooks', 'books', 'comics', 'movies', 'music', 'photos', 'tv'],
  '/mnt/backup': ['2024', '2025'],
  '/srv': ['nas'],
  '/srv/nas': ['Kids Movies', 'Documentaries', 'Podcasts'],
  '/volume1': ['Media', 'Family'],
  '/volume1/Media': ['Concerts', 'Old TV', 'Stand-up'],
}

export function activityLog(): Activity[] {
  const out: Activity[] = []
  const pool = items.slice(0, 20)
  for (let i = 0; i < 137; i++) {
    const it = pool[i % pool.length]
    const user = users[i % 3 === 0 ? 0 : i % 3 === 1 ? 1 : 0]
    const kind = i % 5 === 0 ? 'login' : i % 4 === 0 ? 'download_item' : 'download_file'
    out.push({
      id: 1000 - i,
      at: new Date(now - i * 3_600_000 * 2.7).toISOString(),
      userId: user.id,
      username: user.username,
      action: kind,
      itemId: kind === 'login' ? null : it.id,
      itemTitle: kind === 'login' ? null : it.title,
      detail: kind === 'login' ? `192.168.1.${20 + (i % 30)}` : kind === 'download_item' ? (it.kind === 'show' ? 'Season 1' : null) : (it.files[0]?.name ?? it.seasons[0]?.episodes[0]?.file.name ?? null),
      size: kind === 'login' ? null : kind === 'download_item' ? it.size : (it.files[0]?.size ?? it.seasons[0]?.episodes[0]?.file.size ?? null),
    })
  }
  return out
}
