import { FileText, ListMusic, Package } from 'lucide-react'
import { urls } from '../../api/client'
import type { ItemDetail, MediaFile } from '../../api/types'
import { DownloadButton } from '../../components/Download'
import { formatBytes, pluralize } from '../../lib/format'
import { Hero } from './Hero'
import { FileRow, ListCard, PlayButton, Section } from './parts'

const FORMAT_ORDER = ['epub', 'azw3', 'mobi', 'pdf', 'cbz', 'cbr', 'djvu', 'txt']
const formatRank = (c: string) => {
  const i = FORMAT_ORDER.indexOf(c.toLowerCase())
  return i === -1 ? 99 : i
}

export function BookDetail({ item, onPlay }: { item: ItemDetail; onPlay: (f: MediaFile) => void }) {
  const formats = item.files
    .filter((f) => f.role === 'ebook')
    .sort((a, b) => formatRank(a.container) - formatRank(b.container) || a.name.localeCompare(b.name))
  const others = item.files.filter((f) => f.role !== 'ebook')
  // two files in the same format (e.g. two EPUB editions) need their file names to tell them apart
  const dupes = new Set(
    formats.map((f) => f.container.toLowerCase()).filter((c, i, arr) => arr.indexOf(c) !== i),
  )

  const actions =
    formats.length > 0 ? (
      formats.map((f, i) => (
        <DownloadButton
          key={f.id}
          href={urls.fileDownload(f.id)}
          label={`Download ${f.container.toUpperCase()} (${formatBytes(f.size)})`}
          variant={i === 0 ? 'primary' : 'secondary'}
          btnSize="lg"
          icon={<FileText className="size-4" />}
        >
          <span className="font-semibold">{f.container.toUpperCase()}</span>
          {dupes.has(f.container.toLowerCase()) && f.edition && <span className="font-normal">{f.edition}</span>}
          <span className="font-normal opacity-75">{formatBytes(f.size)}</span>
        </DownloadButton>
      ))
    ) : (
      <p className="text-sm text-ink-400">No e-book file found.</p>
    )

  return (
    <>
      <Hero item={item} actions={actions} />
      {(others.length > 0 || formats.length > 1) && (
        <div className="mt-12 grid gap-10 px-4 sm:px-6 lg:grid-cols-2 lg:px-8">
          {formats.length > 1 && (
            <Section title="Formats">
              <ListCard>
                {formats.map((f) => (
                  <FileRow key={f.id} file={f} />
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

const trackSort = (a: MediaFile, b: MediaFile) =>
  (a.track ?? Number.MAX_SAFE_INTEGER) - (b.track ?? Number.MAX_SAFE_INTEGER) ||
  a.relPath.localeCompare(b.relPath, undefined, { numeric: true })

export function AudiobookDetail({ item, onPlay }: { item: ItemDetail; onPlay: (f: MediaFile) => void }) {
  const tracks = item.files.filter((f) => f.role === 'audio').sort(trackSort)
  const others = item.files.filter((f) => f.role !== 'audio')
  const single = tracks.length === 1 ? tracks[0] : null

  const actions = single ? (
    <>
      <DownloadButton href={urls.fileDownload(single.id)} label={`Download ${item.title}`} variant="primary" btnSize="lg">
        Download <span className="font-semibold uppercase">{single.container}</span>
        <span className="font-normal opacity-75">{formatBytes(single.size)}</span>
      </DownloadButton>
      <PlayButton file={single} onPlay={onPlay} size="lg" />
    </>
  ) : tracks.length > 1 ? (
    <>
      <DownloadButton
        href={urls.itemDownload(item.id)}
        label={`Download all of ${item.title} as zip`}
        size={item.size}
        zip
        variant="primary"
        btnSize="lg"
        icon={<Package className="size-4" />}
      >
        Download all (zip)
        <span className="font-normal opacity-75">{formatBytes(item.size)}</span>
      </DownloadButton>
      <PlayButton file={tracks[0]} onPlay={onPlay} size="lg" label="Listen to part 1" />
    </>
  ) : (
    <p className="text-sm text-ink-400">No audio files found.</p>
  )

  return (
    <>
      <Hero item={item} actions={actions} />
      <div className="mt-12 grid gap-10 px-4 sm:px-6 lg:px-8 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        {tracks.length > 1 && (
          <Section
            title={
              <span className="flex items-center gap-2">
                <ListMusic className="size-5 text-ink-400" /> Tracks
                <span className="text-sm font-normal text-ink-500">
                  {pluralize(tracks.length, 'file')} · {formatBytes(tracks.reduce((n, t) => n + t.size, 0))}
                </span>
              </span>
            }
          >
            <ListCard>
              {tracks.map((t, i) => (
                <FileRow
                  key={t.id}
                  file={t}
                  onPlay={onPlay}
                  leading={
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-ink-800 text-sm font-semibold text-ink-300 tabular-nums">
                      {t.track ?? i + 1}
                    </span>
                  }
                />
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
    </>
  )
}
