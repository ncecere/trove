import { Check, Clapperboard, Layers } from 'lucide-react'
import { useState } from 'react'
import { urls } from '../../api/client'
import type { ItemDetail, MediaFile } from '../../api/types'
import { DownloadButton } from '../../components/Download'
import { formatBytes, resolutionRank } from '../../lib/format'
import { cx } from '../../lib/ui'
import { Hero } from './Hero'
import { FileBadges, FileRow, ListCard, PlayButton, Section, SubtitleList } from './parts'

function sortVersions(files: MediaFile[]): MediaFile[] {
  return [...files].sort((a, b) => resolutionRank(b.resolution) - resolutionRank(a.resolution) || b.size - a.size)
}

function versionLabel(f: MediaFile): string {
  const parts: string[] = []
  if (f.edition) parts.push(f.edition)
  if (f.resolution) parts.push(/2160/i.test(f.resolution) ? '4K' : f.resolution)
  if (f.hdr) parts.push(f.hdr)
  if (!parts.length) parts.push(f.container.toUpperCase())
  return parts.join(' · ')
}

export function MovieDetail({ item, onPlay }: { item: ItemDetail; onPlay: (f: MediaFile) => void }) {
  const versions = sortVersions(item.files.filter((f) => f.role === 'video'))
  const subtitles = item.files.filter((f) => f.role === 'subtitle')
  const extras = item.files.filter((f) => f.role === 'extra')
  const others = item.files.filter((f) => f.role === 'audio' || f.role === 'ebook')
  const [selectedId, setSelectedId] = useState<number | null>(versions[0]?.id ?? null)
  const selected = versions.find((v) => v.id === selectedId) ?? versions[0]

  const actions = selected ? (
    <>
      <DownloadButton
        href={urls.fileDownload(selected.id)}
        label={`Download ${item.title}`}
        variant="primary"
        btnSize="lg"
      >
        Download
        <span className="font-normal opacity-75">{formatBytes(selected.size)}</span>
      </DownloadButton>
      <PlayButton file={selected} onPlay={onPlay} size="lg" />
      {versions.length > 1 && (
        <span className="text-center text-sm text-ink-400 sm:text-left">
          {versionLabel(selected)} · {versions.length} versions
        </span>
      )}
    </>
  ) : (
    <p className="text-sm text-ink-400">No video file found for this movie.</p>
  )

  const below =
    versions.length > 1 ? (
      <fieldset className="mt-5 max-w-3xl">
        <legend className="mb-2 flex items-center gap-2 text-xs font-semibold tracking-[0.12em] text-ink-500 uppercase">
          <Layers className="size-3.5" /> Choose a version
        </legend>
        <div className="grid gap-2 sm:grid-cols-2">
          {versions.map((v) => {
            const active = v.id === selected?.id
            return (
              <label
                key={v.id}
                className={cx(
                  'relative flex cursor-pointer flex-col gap-2 rounded-xl border p-3 transition has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ember-400',
                  active
                    ? 'border-ember-400/60 bg-ember-400/[0.07]'
                    : 'border-white/[0.08] bg-ink-900/60 hover:border-white/20',
                )}
              >
                <input
                  type="radio"
                  name="version"
                  className="sr-only"
                  checked={active}
                  onChange={() => setSelectedId(v.id)}
                />
                <span className="flex items-center justify-between gap-2">
                  <span className="min-w-0 truncate text-sm font-semibold text-ink-50">{versionLabel(v)}</span>
                  <span className="flex shrink-0 items-center gap-2 text-sm text-ink-300">
                    {formatBytes(v.size)}
                    <span
                      className={cx(
                        'flex size-5 items-center justify-center rounded-full border',
                        active ? 'border-ember-400 bg-ember-500 text-ink-950' : 'border-ink-500',
                      )}
                      aria-hidden="true"
                    >
                      {active && <Check className="size-3" strokeWidth={3} />}
                    </span>
                  </span>
                </span>
                <FileBadges file={v} />
                <span className="truncate text-xs text-ink-500" title={v.relPath}>
                  {v.name}
                </span>
              </label>
            )
          })}
        </div>
      </fieldset>
    ) : selected ? (
      <div className="mt-4 flex flex-wrap items-center justify-center gap-2 md:justify-start">
        <FileBadges file={selected} />
      </div>
    ) : null

  return (
    <>
      <Hero item={item} actions={actions} below={below} />
      {(subtitles.length > 0 || extras.length > 0 || others.length > 0) && (
        <div className="mt-12 grid gap-10 px-4 sm:px-6 lg:grid-cols-2 lg:px-8">
          <SubtitleList files={subtitles} />
          {extras.length > 0 && (
            <Section title={<span className="flex items-center gap-2"><Clapperboard className="size-5 text-ink-400" /> Extras</span>}>
              <ListCard>
                {extras.map((f) => (
                  <FileRow key={f.id} file={f} onPlay={onPlay} />
                ))}
              </ListCard>
            </Section>
          )}
          {others.length > 0 && (
            <Section title="Other files">
              <ListCard>
                {others.map((f) => (
                  <FileRow key={f.id} file={f} onPlay={onPlay} />
                ))}
              </ListCard>
            </Section>
          )}
        </div>
      )}
    </>
  )
}
