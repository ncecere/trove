import { ArrowUpRight, Calendar, Clock, HardDrive, Lock, RefreshCw, Star } from 'lucide-react'
import { useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { errorMessage } from '../../api/client'
import { useLibraries, useRefreshItem, useUser } from '../../api/hooks'
import type { ItemDetail } from '../../api/types'
import { PosterImage } from '../../components/Artwork'
import { NO_DOWNLOAD_HINT } from '../../components/Download'
import { Spinner } from '../../components/States'
import { placeholderBackground } from '../../lib/art'
import { externalLinks } from '../../lib/external'
import { formatBytes, formatDate, formatRating, formatRuntime, KIND_LABEL, pluralize, relativeTime } from '../../lib/format'
import { LIBRARY_ICON } from '../../lib/icons'
import { useToast } from '../../lib/toast'
import { buttonClass, cx } from '../../lib/ui'

export function ClampText({ text, lines = 4, className }: { text: string; lines?: 2 | 3 | 4 | 5; className?: string }) {
  const ref = useRef<HTMLParagraphElement>(null)
  const [open, setOpen] = useState(false)
  const [overflows, setOverflows] = useState(false)
  useLayoutEffect(() => {
    const el = ref.current
    if (!el || open) return
    const check = () => setOverflows(el.scrollHeight - el.clientHeight > 2)
    check()
    const ro = new ResizeObserver(check)
    ro.observe(el)
    return () => ro.disconnect()
  }, [text, open])
  const clamp = { 2: 'line-clamp-2', 3: 'line-clamp-3', 4: 'line-clamp-4', 5: 'line-clamp-5' }[lines]
  return (
    <div className={className}>
      <p ref={ref} className={cx('whitespace-pre-line', !open && clamp)}>
        {text}
      </p>
      {(overflows || open) && (
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          className="mt-1 rounded text-sm font-medium text-ember-300 hover:text-ember-200"
          aria-expanded={open}
        >
          {open ? 'Less' : 'More'}
        </button>
      )}
    </div>
  )
}

function Backdrop({ item }: { item: ItemDetail }) {
  const [failed, setFailed] = useState(false)
  const src = item.backdropUrl && !failed ? item.backdropUrl : null
  return (
    <div className="absolute inset-0 -z-10 overflow-hidden" aria-hidden="true">
      {src ? (
        <img
          src={src}
          alt=""
          onError={() => setFailed(true)}
          className="h-full w-full animate-fade-in object-cover object-[center_25%] opacity-55"
        />
      ) : item.posterUrl ? (
        <img src={item.posterUrl} alt="" className="h-full w-full scale-125 object-cover opacity-45 blur-3xl saturate-150" />
      ) : (
        <div className="h-full w-full opacity-70 blur-2xl" style={{ background: placeholderBackground(item.title) }} />
      )}
      <div className="absolute inset-0 bg-gradient-to-b from-ink-950/30 via-ink-950/70 to-ink-950" />
      <div className="absolute inset-0 hidden bg-gradient-to-r from-ink-950/90 via-ink-950/40 to-transparent md:block" />
    </div>
  )
}

function posterWidth(item: ItemDetail): string {
  switch (item.kind) {
    case 'book':
      return 'w-44 sm:w-52 md:w-60 lg:w-72 xl:w-80' // cover-forward
    case 'audiobook':
      return 'w-56 sm:w-60 md:w-64 lg:w-72 xl:w-80'
    default:
      return 'w-40 sm:w-48 md:w-56 lg:w-64 xl:w-72'
  }
}

interface HeroProps {
  item: ItemDetail
  actions?: ReactNode
  /** extra content under the actions inside the hero (e.g. version picker) */
  below?: ReactNode
}

export function Hero({ item, actions, below }: HeroProps) {
  const user = useUser()
  const libs = useLibraries()
  const lib = libs.data?.find((l) => l.id === item.libraryId)
  const LibIcon = LIBRARY_ICON[item.libraryType]
  const rating = formatRating(item.rating)
  const runtime = formatRuntime(item.runtimeMinutes)
  const links = externalLinks(item)
  const isBookish = item.kind === 'book' || item.kind === 'audiobook'
  const square = item.kind === 'audiobook'

  const meta: ReactNode[] = []
  if (item.year) meta.push(<span key="y">{item.year}</span>)
  if (runtime) meta.push(<span key="r" className="inline-flex items-center gap-1"><Clock className="size-3.5" />{runtime}</span>)
  if (item.kind === 'show' && item.seasonCount)
    meta.push(<span key="s">{pluralize(item.seasonCount, 'season')}</span>)
  if (item.kind === 'show' && item.episodeCount)
    meta.push(<span key="e">{pluralize(item.episodeCount, 'episode')}</span>)
  if (item.status) meta.push(<span key="st">{item.status}</span>)

  return (
    <div className="relative isolate">
      <Backdrop item={item} />
      <div className="px-4 pt-6 sm:px-6 sm:pt-10 lg:px-8 lg:pt-16 xl:pt-24">
        <div className="flex flex-col items-center gap-6 md:flex-row md:items-start md:gap-8 lg:gap-12">
          <div className={cx('shrink-0', posterWidth(item))}>
            <div
              className={cx(
                'relative overflow-hidden rounded-2xl shadow-[0_30px_60px_-20px_rgb(0_0_0/0.85)] ring-1 ring-white/10',
                square ? 'aspect-square' : 'aspect-[2/3]',
                item.kind === 'book' && 'rounded-l-md',
              )}
            >
              <PosterImage
                src={item.posterUrl}
                title={item.title}
                kind={item.kind}
                subtitle={isBookish ? item.author : item.year ? String(item.year) : null}
                size="hero"
                eager
              />
              {item.kind === 'book' && (
                <div className="pointer-events-none absolute inset-y-0 left-0 w-4 bg-gradient-to-r from-black/40 via-white/10 to-transparent" />
              )}
            </div>
          </div>

          <div className="w-full min-w-0 flex-1 md:pt-2 lg:pt-6">
            <div className="flex flex-wrap items-center justify-center gap-2 md:justify-start">
              <Link
                to={`/library/${item.libraryId}`}
                className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-ink-950/40 px-2.5 py-1 text-xs font-medium text-ink-200 backdrop-blur transition hover:border-ember-400/40 hover:text-ink-50"
              >
                <LibIcon className="size-3.5 text-ember-300" />
                {lib?.name ?? KIND_LABEL[item.kind].many}
              </Link>
              {rating && (
                <span className="inline-flex items-center gap-1 rounded-full bg-ink-950/40 px-2.5 py-1 text-xs font-semibold text-ink-50 ring-1 ring-white/10 backdrop-blur">
                  <Star className="size-3.5 fill-ember-400 text-ember-400" /> {rating}
                  <span className="font-normal text-ink-400">/ 10</span>
                </span>
              )}
            </div>

            <h1 className="mt-3 text-center text-3xl leading-[1.08] font-bold tracking-[-0.025em] text-balance break-words text-white sm:text-4xl md:text-left lg:text-5xl">
              {item.title}
            </h1>
            {item.originalTitle && item.originalTitle !== item.title && (
              <p className="mt-1.5 text-center text-sm text-ink-400 md:text-left" lang="und">
                {item.originalTitle}
              </p>
            )}

            {isBookish && (item.author || item.series || item.narrator) && (
              <div className="mt-3 space-y-1 text-center text-[0.95rem] text-ink-200 md:text-left">
                {item.author && (
                  <p>
                    by <span className="font-semibold text-ink-50">{item.author}</span>
                  </p>
                )}
                {item.narrator && (
                  <p className="text-sm text-ink-300">
                    Narrated by <span className="font-medium text-ink-100">{item.narrator}</span>
                  </p>
                )}
                {item.series && (
                  <p className="text-sm text-ink-300">
                    <span className="font-medium text-ember-200">{item.series}</span>
                    {item.seriesIndex !== null && <> · Book {item.seriesIndex}</>}
                  </p>
                )}
              </div>
            )}

            {meta.length > 0 && (
              <div className="mt-3 flex flex-wrap items-center justify-center gap-x-2.5 gap-y-1 text-sm text-ink-300 md:justify-start">
                {meta.map((m, i) => (
                  <span key={i} className="inline-flex items-center gap-2.5">
                    {i > 0 && <span className="text-ink-600" aria-hidden="true">•</span>}
                    {m}
                  </span>
                ))}
              </div>
            )}

            {item.genres.length > 0 && (
              <ul className="mt-4 flex flex-wrap justify-center gap-1.5 md:justify-start" aria-label="Genres">
                {item.genres.map((g) => (
                  <li key={g}>
                    <Link
                      to={`/library/${item.libraryId}?genre=${encodeURIComponent(g)}`}
                      className="inline-flex h-7 items-center rounded-full border border-white/10 bg-white/[0.04] px-3 text-xs font-medium text-ink-200 transition hover:border-ember-400/40 hover:text-ink-50"
                    >
                      {g}
                    </Link>
                  </li>
                ))}
              </ul>
            )}

            {actions && (
              <div className="mt-6 flex flex-col gap-2.5 sm:flex-row sm:flex-wrap sm:items-center">{actions}</div>
            )}
            {actions && !user.canDownload && (
              <p className="mt-3 flex items-center justify-center gap-2 text-sm text-ink-400 md:justify-start">
                <Lock className="size-4 shrink-0" /> {NO_DOWNLOAD_HINT}
              </p>
            )}
            {below}

            {item.tagline && (
              <p className="mt-6 text-lg font-medium text-ink-200 italic">“{item.tagline}”</p>
            )}
            {item.overview ? (
              <ClampText text={item.overview} lines={4} className={cx('max-w-3xl text-[0.95rem] leading-relaxed text-ink-300', item.tagline ? 'mt-2' : 'mt-6')} />
            ) : (
              <p className="mt-6 text-sm text-ink-500 italic">
                No description yet.
                {user.role === 'admin' && ' Configure a metadata source in Admin › Settings, then refresh.'}
              </p>
            )}

            <Facts item={item} />

            {(links.length > 0 || user.role === 'admin') && (
              <div className="mt-6 flex flex-wrap items-center gap-2">
                {links.map((l) => (
                  <a
                    key={l.key}
                    href={l.href}
                    target="_blank"
                    rel="noreferrer noopener"
                    className="inline-flex h-8 items-center gap-1 rounded-lg border border-white/10 px-2.5 text-xs font-medium text-ink-300 transition hover:border-white/25 hover:text-ink-50"
                  >
                    {l.label}
                    <ArrowUpRight className="size-3.5" />
                  </a>
                ))}
                {user.role === 'admin' && <RefreshButton id={item.id} />}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function Facts({ item }: { item: ItemDetail }) {
  const facts: { label: string; value: ReactNode; icon?: ReactNode }[] = []
  if (item.releaseDate)
    facts.push({ label: item.kind === 'show' ? 'First aired' : item.kind === 'movie' ? 'Released' : 'Published', value: formatDate(item.releaseDate), icon: <Calendar className="size-3.5" /> })
  if (item.studio)
    facts.push({ label: item.kind === 'book' || item.kind === 'audiobook' ? 'Publisher' : item.kind === 'show' ? 'Network' : 'Studio', value: item.studio })
  facts.push({ label: 'Size', value: `${formatBytes(item.size)} · ${pluralize(item.fileCount, 'file')}`, icon: <HardDrive className="size-3.5" /> })
  facts.push({ label: 'Added', value: <time dateTime={item.addedAt} title={formatDate(item.addedAt)}>{relativeTime(item.addedAt)}</time> })
  if (item.externalIds.isbn) facts.push({ label: 'ISBN', value: item.externalIds.isbn })
  return (
    <dl className="mt-6 grid max-w-3xl grid-cols-2 gap-x-6 gap-y-3 border-t border-white/[0.06] pt-5 sm:grid-cols-4">
      {facts.map((f) => (
        <div key={f.label} className="min-w-0">
          <dt className="text-[0.7rem] font-semibold tracking-[0.12em] text-ink-500 uppercase">{f.label}</dt>
          <dd className="mt-1 truncate text-sm text-ink-200">{f.value}</dd>
        </div>
      ))}
    </dl>
  )
}

function RefreshButton({ id }: { id: number }) {
  const refresh = useRefreshItem()
  const toast = useToast()
  return (
    <button
      type="button"
      className={buttonClass('ghost', 'sm', 'h-8 text-xs')}
      disabled={refresh.isPending}
      onClick={() =>
        refresh.mutate(id, {
          onSuccess: () => toast.success('Refreshing metadata', 'New artwork and details will appear in a moment.'),
          onError: (e) => toast.error('Couldn’t refresh metadata', errorMessage(e)),
        })
      }
    >
      {refresh.isPending ? <Spinner className="size-3.5" /> : <RefreshCw className="size-3.5" />}
      Refresh metadata
    </button>
  )
}
