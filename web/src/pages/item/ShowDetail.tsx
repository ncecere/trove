import { Captions, Tv } from 'lucide-react'
import { useRef, type KeyboardEvent } from 'react'
import { useSearchParams } from 'react-router'
import { urls } from '../../api/client'
import { useUser } from '../../api/hooks'
import type { Episode, ItemDetail, MediaFile, Season } from '../../api/types'
import { DownloadButton } from '../../components/Download'
import { EmptyState } from '../../components/States'
import { episodeCode, formatBytes, formatDate, languageName, pluralize, seasonLabel } from '../../lib/format'
import { cx } from '../../lib/ui'
import { ClampText, Hero } from './Hero'
import { FileBadges, FileRow, ListCard, PlayButton, Section } from './parts'

const orderSeasons = (s: Season[]) => [...s].sort((a, b) => (a.season === 0 ? 1 : b.season === 0 ? -1 : a.season - b.season))

export function ShowDetail({ item, onPlay }: { item: ItemDetail; onPlay: (f: MediaFile) => void }) {
  const seasons = orderSeasons(item.seasons)
  const [params, setParams] = useSearchParams()
  const raw = params.get('season')
  const requested = raw === null ? NaN : Number(raw)
  const current = seasons.find((s) => s.season === requested) ?? seasons[0]
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([])
  const extraFiles = item.files

  const select = (n: number) =>
    setParams(
      (p) => {
        const next = new URLSearchParams(p)
        next.set('season', String(n))
        return next
      },
      { replace: true, preventScrollReset: true },
    )

  const onTabKey = (e: KeyboardEvent, i: number) => {
    let j = -1
    if (e.key === 'ArrowRight') j = (i + 1) % seasons.length
    else if (e.key === 'ArrowLeft') j = (i - 1 + seasons.length) % seasons.length
    else if (e.key === 'Home') j = 0
    else if (e.key === 'End') j = seasons.length - 1
    if (j >= 0) {
      e.preventDefault()
      select(seasons[j].season)
      tabRefs.current[j]?.focus()
    }
  }

  const actions =
    seasons.length > 0 ? (
      <>
        <DownloadButton
          href={urls.itemDownload(item.id)}
          label={`Download all of ${item.title}`}
          size={item.size}
          zip
          variant="primary"
          btnSize="lg"
        >
          Download all
          <span className="font-normal opacity-75">{formatBytes(item.size)}</span>
        </DownloadButton>
      </>
    ) : undefined

  return (
    <>
      <Hero item={item} actions={actions} />
      <div className="mt-12 px-4 sm:px-6 lg:px-8">
        {seasons.length === 0 ? (
          <EmptyState icon={<Tv className="size-7" />} title="No episodes found">
            No episode files were recognised for this show. Episodes are matched by names like “S01E02”.
          </EmptyState>
        ) : (
          <>
            <div
              role="tablist"
              aria-label="Seasons"
              className="scrollbar-none -mx-4 flex gap-1.5 overflow-x-auto px-4 pb-1 sm:-mx-6 sm:px-6 lg:mx-0 lg:px-0"
            >
              {seasons.map((s, i) => {
                const active = s.season === current?.season
                return (
                  <button
                    key={s.season}
                    ref={(el) => {
                      tabRefs.current[i] = el
                    }}
                    type="button"
                    role="tab"
                    id={`season-tab-${s.season}`}
                    aria-selected={active}
                    aria-controls="season-panel"
                    tabIndex={active ? 0 : -1}
                    onClick={() => select(s.season)}
                    onKeyDown={(e) => onTabKey(e, i)}
                    className={cx(
                      'flex h-10 shrink-0 items-center gap-2 rounded-full px-4 text-sm font-medium transition',
                      active
                        ? 'bg-ink-50 text-ink-950'
                        : 'border border-white/[0.08] bg-ink-900 text-ink-300 hover:border-white/20 hover:text-ink-50',
                    )}
                  >
                    {seasonLabel(s.season)}
                    <span className={cx('text-xs', active ? 'text-ink-600' : 'text-ink-500')}>{s.episodeCount}</span>
                  </button>
                )
              })}
            </div>

            {current && (
              <div id="season-panel" role="tabpanel" aria-labelledby={`season-tab-${current.season}`} className="mt-5">
                <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h2 className="text-xl font-semibold tracking-tight text-ink-50">{seasonLabel(current.season)}</h2>
                    <p className="mt-0.5 text-sm text-ink-400">
                      {pluralize(current.episodeCount, 'episode')} · {formatBytes(current.size)}
                    </p>
                  </div>
                  <DownloadButton
                    href={urls.itemDownload(item.id, current.season)}
                    label={`Download ${seasonLabel(current.season)} as zip`}
                    size={current.size}
                    zip
                    variant="outline"
                  >
                    Download season
                  </DownloadButton>
                </div>
                <ListCard>
                  {current.episodes.map((ep) => (
                    <EpisodeRow key={ep.file.id} ep={ep} onPlay={onPlay} />
                  ))}
                </ListCard>
              </div>
            )}
          </>
        )}

        {extraFiles.length > 0 && (
          <div className="mt-12 max-w-3xl">
            <Section title="Other files">
              <ListCard>
                {extraFiles.map((f) => (
                  <FileRow key={f.id} file={f} onPlay={onPlay} />
                ))}
              </ListCard>
            </Section>
          </div>
        )}
      </div>
    </>
  )
}

function EpisodeRow({ ep, onPlay }: { ep: Episode; onPlay: (f: MediaFile) => void }) {
  const user = useUser()
  const title = ep.title ?? `Episode ${ep.episode}`
  return (
    <li className="flex gap-3 px-3 py-4 sm:gap-5 sm:px-5">
      <div className="w-9 shrink-0 pt-0.5 text-right sm:w-12">
        <span className="text-xl font-bold text-ink-500 tabular-nums sm:text-2xl">
          {ep.episodeEnd && ep.episodeEnd !== ep.episode ? `${ep.episode}–${ep.episodeEnd}` : ep.episode}
        </span>
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-start gap-3">
          <div className="min-w-0 flex-1">
            <h3 className="text-[0.95rem] leading-snug font-semibold text-ink-50 break-words">{title}</h3>
            <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-ink-400">
              <span className="font-medium text-ink-300 tabular-nums">{episodeCode(ep)}</span>
              {ep.airDate && (
                <>
                  <span aria-hidden="true">·</span>
                  <time dateTime={ep.airDate}>{formatDate(ep.airDate)}</time>
                </>
              )}
              <span aria-hidden="true">·</span>
              <span>{formatBytes(ep.file.size)}</span>
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <PlayButton file={ep.file} onPlay={onPlay} iconOnly size="sm" variant="ghost" label={`Play ${title}`} />
            <DownloadButton
              href={urls.fileDownload(ep.file.id)}
              label={`Download ${episodeCode(ep)} ${title} (${formatBytes(ep.file.size)})`}
              iconOnly
              btnSize="sm"
              variant="outline"
            />
          </div>
        </div>
        <div className="mt-2">
          <FileBadges file={ep.file} />
        </div>
        {ep.overview && <ClampText text={ep.overview} lines={2} className="mt-2 max-w-3xl text-sm leading-relaxed text-ink-400" />}
        {ep.subtitles.length > 0 && (
          <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
            <Captions className="size-4 text-ink-500" aria-label="Subtitles" />
            {ep.subtitles.map((s) => {
              const lang = languageName(s.language) ?? s.name
              const label = `${lang} · ${s.container.toUpperCase()}`
              return user.canDownload ? (
                <a
                  key={s.id}
                  href={urls.fileDownload(s.id)}
                  download
                  title={`Download subtitle ${s.name}`}
                  className="inline-flex h-7 items-center rounded-md border border-white/[0.08] px-2 text-xs text-ink-300 transition hover:border-ember-400/40 hover:text-ink-50"
                >
                  {label}
                </a>
              ) : (
                <span key={s.id} className="inline-flex h-7 items-center rounded-md border border-white/[0.06] px-2 text-xs text-ink-500">
                  {label}
                </span>
              )
            })}
          </div>
        )}
      </div>
    </li>
  )
}
