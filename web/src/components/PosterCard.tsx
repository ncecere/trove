import { Star } from 'lucide-react'
import { Link } from 'react-router'
import type { ItemSummary } from '../api/types'
import { formatRating, pluralize } from '../lib/format'
import { cx } from '../lib/ui'
import { PosterImage } from './Artwork'

function cardMeta(item: ItemSummary): string {
  const parts: string[] = []
  if (item.kind === 'book' || item.kind === 'audiobook') {
    if (item.author) parts.push(item.author)
    else if (item.year) parts.push(String(item.year))
    return parts.join(' · ')
  }
  if (item.year) parts.push(String(item.year))
  if (item.kind === 'show') {
    if (item.seasonCount && item.seasonCount > 1) parts.push(pluralize(item.seasonCount, 'season'))
    else if (item.episodeCount) parts.push(pluralize(item.episodeCount, 'episode'))
  }
  return parts.join(' · ')
}

interface Props {
  item: ItemSummary
  /** Force a 2:3 frame even for square (audiobook) art, so mixed rows line up. */
  uniform?: boolean
  className?: string
  eager?: boolean
}

export function PosterCard({ item, uniform, className, eager }: Props) {
  const square = item.kind === 'audiobook'
  const rating = formatRating(item.rating)
  const meta = cardMeta(item)
  const subtitle = item.kind === 'book' || item.kind === 'audiobook' ? item.author : null
  const framed = square && uniform

  return (
    <Link
      to={`/item/${item.id}`}
      className={cx('group block rounded-xl focus-visible:outline-offset-4', className)}
      aria-label={[item.title, meta].filter(Boolean).join(', ')}
    >
      <div
        className={cx(
          'relative overflow-hidden rounded-xl bg-ink-800 shadow-card ring-1 ring-white/[0.06] transition duration-300 ease-out',
          'group-hover:-translate-y-1 group-hover:shadow-lift group-hover:ring-ember-400/35 motion-reduce:group-hover:translate-y-0',
          square && !uniform ? 'aspect-square' : 'aspect-[2/3]',
        )}
      >
        {framed && item.posterUrl ? (
          <>
            <img
              src={item.posterUrl}
              alt=""
              aria-hidden="true"
              loading="lazy"
              className="absolute inset-0 h-full w-full scale-125 object-cover opacity-60 blur-xl"
            />
            <div className="absolute inset-x-0 top-1/2 aspect-square -translate-y-1/2 shadow-[0_10px_30px_rgb(0_0_0/0.5)]">
              <PosterImage src={item.posterUrl} title={item.title} kind={item.kind} subtitle={subtitle} eager={eager} />
            </div>
          </>
        ) : (
          <PosterImage src={item.posterUrl} title={item.title} kind={item.kind} subtitle={subtitle} eager={eager} />
        )}

        {item.kind === 'book' && (
          // a whisper of a spine on book covers
          <div className="pointer-events-none absolute inset-y-0 left-0 w-2.5 bg-gradient-to-r from-black/35 via-white/10 to-transparent" />
        )}

        {rating && (
          <span className="absolute top-2 right-2 inline-flex items-center gap-1 rounded-md bg-ink-950/75 px-1.5 py-0.5 text-[0.7rem] font-semibold text-ink-50 ring-1 ring-white/10 backdrop-blur">
            <Star className="size-3 fill-ember-400 text-ember-400" aria-hidden="true" />
            {rating}
          </span>
        )}

        {item.kind === 'show' && item.episodeCount ? (
          <span className="absolute top-2 left-2 rounded-md bg-ink-950/75 px-1.5 py-0.5 text-[0.65rem] font-semibold tracking-wide text-ink-100 ring-1 ring-white/10 backdrop-blur">
            {item.episodeCount} EP
          </span>
        ) : null}

        {/* hover overlay (pointer devices) */}
        <div className="pointer-events-none absolute inset-0 flex flex-col justify-end bg-gradient-to-t from-ink-950/95 via-ink-950/30 to-transparent p-3 opacity-0 transition duration-300 group-hover:opacity-100 group-focus-visible:opacity-100">
          <p className="line-clamp-3 text-sm leading-snug font-semibold text-white">{item.title}</p>
          {meta && <p className="mt-0.5 line-clamp-1 text-xs text-ink-300">{meta}</p>}
        </div>
      </div>
      <div className="mt-2 min-w-0 px-0.5">
        <p className="truncate text-sm font-medium text-ink-100 group-hover:text-white">{item.title}</p>
        <p className="h-4 truncate text-xs text-ink-400">{meta}</p>
      </div>
    </Link>
  )
}
