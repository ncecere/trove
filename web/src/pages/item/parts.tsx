import { Captions, FileText, Film, Music, Play } from 'lucide-react'
import type { ReactNode } from 'react'
import { urls } from '../../api/client'
import type { MediaFile } from '../../api/types'
import { DownloadButton } from '../../components/Download'
import { formatBytes, languageName, playableAs } from '../../lib/format'
import { buttonClass, cx } from '../../lib/ui'

export function Badge({ children, tone = 'default', className }: { children: ReactNode; tone?: 'default' | 'accent' | 'hdr'; className?: string }) {
  return (
    <span
      className={cx(
        'inline-flex h-5 items-center rounded-md px-1.5 text-[0.68rem] font-semibold tracking-wide whitespace-nowrap',
        tone === 'default' && 'bg-white/[0.07] text-ink-200 ring-1 ring-white/[0.08]',
        tone === 'accent' && 'bg-ember-400/15 text-ember-200 ring-1 ring-ember-400/30',
        tone === 'hdr' && 'bg-gradient-to-r from-fuchsia-500/20 to-sky-400/20 text-sky-100 ring-1 ring-sky-300/25',
        className,
      )}
    >
      {children}
    </span>
  )
}

export function FileBadges({ file }: { file: MediaFile }) {
  const res = file.resolution
  return (
    <span className="flex flex-wrap items-center gap-1">
      {res && <Badge tone={/2160|4k/i.test(res) ? 'accent' : 'default'}>{/2160/i.test(res) ? '4K' : res}</Badge>}
      {file.hdr && <Badge tone="hdr">{file.hdr}</Badge>}
      {file.source && <Badge>{file.source}</Badge>}
      {file.videoCodec && <Badge>{file.videoCodec.toUpperCase()}</Badge>}
      {file.audioCodec && <Badge>{file.audioCodec.toUpperCase()}</Badge>}
      {file.edition && <Badge tone="accent">{file.edition}</Badge>}
    </span>
  )
}

export function PlayButton({
  file,
  onPlay,
  label,
  iconOnly,
  size = 'md',
  variant = 'secondary',
}: {
  file: MediaFile
  onPlay: (f: MediaFile) => void
  label?: string
  iconOnly?: boolean
  size?: 'sm' | 'md' | 'lg'
  variant?: 'secondary' | 'ghost' | 'outline'
}) {
  if (!playableAs(file)) return null
  const text = label ?? (playableAs(file) === 'audio' ? 'Listen' : 'Play')
  return (
    <button
      type="button"
      onClick={() => onPlay(file)}
      className={buttonClass(variant, size, iconOnly ? (size === 'sm' ? 'w-9 px-0' : 'w-10 px-0') : undefined)}
      aria-label={iconOnly ? `${text}: ${file.name}` : undefined}
      title={iconOnly ? `Preview in browser` : undefined}
    >
      <Play className="size-4 fill-current" />
      {!iconOnly && text}
    </button>
  )
}

const ROLE_ICON = { video: Film, audio: Music, ebook: FileText, subtitle: Captions, extra: Film }

/** One downloadable file in a list. */
export function FileRow({
  file,
  title,
  meta,
  onPlay,
  leading,
}: {
  file: MediaFile
  title?: ReactNode
  meta?: ReactNode
  onPlay?: (f: MediaFile) => void
  leading?: ReactNode
}) {
  const Icon = ROLE_ICON[file.role]
  return (
    <li className="flex items-center gap-3 px-3 py-2.5 sm:px-4">
      {leading ?? (
        <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-ink-800 text-ink-400">
          <Icon className="size-4" />
        </span>
      )}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-ink-100" title={file.relPath}>
          {title ?? file.name}
        </p>
        <p className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-ink-400">
          {meta ?? (
            <>
              <span className="uppercase">{file.container}</span>
              <span aria-hidden="true">·</span>
              <span>{formatBytes(file.size)}</span>
            </>
          )}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        {onPlay && <PlayButton file={file} onPlay={onPlay} iconOnly size="sm" variant="ghost" />}
        <DownloadButton
          href={urls.fileDownload(file.id)}
          label={`Download ${file.name} (${formatBytes(file.size)})`}
          iconOnly
          btnSize="sm"
          variant="ghost"
        />
      </div>
    </li>
  )
}

export function Section({ title, children, action, id }: { title: ReactNode; children: ReactNode; action?: ReactNode; id?: string }) {
  return (
    <section id={id} className="min-w-0">
      <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
        <h2 className="text-lg font-semibold tracking-tight text-ink-50">{title}</h2>
        {action}
      </div>
      {children}
    </section>
  )
}

export function ListCard({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <ul className={cx('divide-y divide-white/[0.05] overflow-hidden rounded-2xl border border-white/[0.06] bg-ink-900/70', className)}>
      {children}
    </ul>
  )
}

export function SubtitleList({ files }: { files: MediaFile[] }) {
  if (!files.length) return null
  return (
    <Section title={<span className="flex items-center gap-2"><Captions className="size-5 text-ink-400" /> Subtitles</span>}>
      <ListCard>
        {files.map((f) => (
          <FileRow
            key={f.id}
            file={f}
            title={languageName(f.language) ?? f.name}
            meta={
              <>
                <span className="uppercase">{f.container}</span>
                <span aria-hidden="true">·</span>
                <span className="truncate">{f.name}</span>
              </>
            }
          />
        ))}
      </ListCard>
    </Section>
  )
}
