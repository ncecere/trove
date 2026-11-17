import { ChevronRight, CornerLeftUp, Folder, FolderOpen, HardDrive } from 'lucide-react'
import { useState } from 'react'
import { useFs } from '../../api/hooks'
import { ErrorState, Skeleton } from '../../components/States'
import { buttonClass, cx } from '../../lib/ui'

function crumbs(path: string): { name: string; path: string }[] {
  const parts = path.split('/').filter(Boolean)
  const out = [{ name: '/', path: '/' }]
  let acc = ''
  for (const p of parts) {
    acc += `/${p}`
    out.push({ name: p, path: acc })
  }
  return out
}

/** Server-side directory browser built on GET /admin/fs. */
export function FolderPicker({ initial, onPick }: { initial: string; onPick: (path: string) => void }) {
  const [path, setPath] = useState(initial || '/')
  const fs = useFs(path, true)
  const listing = fs.data
  const shownPath = listing?.path ?? path

  return (
    <div className="overflow-hidden rounded-xl border border-white/[0.08] bg-ink-950/60">
      <div className="flex items-center gap-1 border-b border-white/[0.06] px-2 py-2">
        <button
          type="button"
          className={buttonClass('ghost', 'sm', 'w-9 shrink-0 px-0')}
          onClick={() => listing?.parent && setPath(listing.parent)}
          disabled={!listing?.parent}
          aria-label="Up one folder"
          title="Up one folder"
        >
          <CornerLeftUp className="size-4" />
        </button>
        <nav aria-label="Current folder" className="scrollbar-none flex min-w-0 flex-1 items-center overflow-x-auto text-sm">
          {crumbs(shownPath).map((c, i, arr) => (
            <span key={c.path} className="flex shrink-0 items-center">
              {i > 1 && <ChevronRight className="size-3.5 text-ink-600" />}
              <button
                type="button"
                onClick={() => setPath(c.path)}
                className={cx(
                  'rounded-md px-1.5 py-1 font-mono text-xs',
                  i === arr.length - 1 ? 'text-ink-50' : 'text-ink-400 hover:bg-ink-800 hover:text-ink-100',
                )}
                aria-current={i === arr.length - 1 ? 'location' : undefined}
              >
                {i === 0 ? <HardDrive className="size-3.5" aria-label="Root" /> : c.name}
              </button>
            </span>
          ))}
        </nav>
      </div>
      <div className="scrollbar-thin h-56 overflow-y-auto p-1.5">
        {fs.isPending ? (
          <div className="space-y-1 p-1">
            {Array.from({ length: 6 }, (_, i) => (
              <Skeleton key={i} className="h-8" />
            ))}
          </div>
        ) : fs.isError ? (
          <ErrorState error={fs.error} onRetry={() => void fs.refetch()} className="py-6" />
        ) : listing && listing.dirs.length === 0 ? (
          <p className="px-3 py-8 text-center text-sm text-ink-500">No sub-folders here.</p>
        ) : (
          <ul className={cx(fs.isPlaceholderData && 'opacity-60')}>
            {listing?.dirs.map((d) => (
              <li key={d.path}>
                <button
                  type="button"
                  onClick={() => setPath(d.path)}
                  onDoubleClick={() => onPick(d.path)}
                  className="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm text-ink-200 transition hover:bg-ink-800 hover:text-ink-50"
                >
                  <Folder className="size-4 shrink-0 text-ember-300/80" />
                  <span className="truncate">{d.name}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="flex items-center justify-between gap-3 border-t border-white/[0.06] px-3 py-2.5">
        <span className="min-w-0 truncate font-mono text-xs text-ink-400" title={shownPath}>
          {shownPath}
        </span>
        <button type="button" className={buttonClass('primary', 'sm', 'shrink-0')} onClick={() => onPick(shownPath)} disabled={!listing}>
          <FolderOpen className="size-4" /> Use this folder
        </button>
      </div>
    </div>
  )
}
