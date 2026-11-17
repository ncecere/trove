import { CircleExclamationPoint, RefreshCw } from 'lucide-react'
import type { ScanStatus } from '../api/types'
import { scanLabel, scanPercent } from '../lib/scan'
import { cx } from '../lib/ui'

export function ScanProgress({ scan, className }: { scan: ScanStatus; className?: string }) {
  const pct = scanPercent(scan)
  return (
    <div className={cx('h-1.5 overflow-hidden rounded-full bg-ink-700', className)}>
      {pct === null ? (
        <div className="bg-ember-gradient h-full w-1/3 animate-indeterminate rounded-full" />
      ) : (
        <div className="bg-ember-gradient h-full rounded-full transition-[width] duration-500" style={{ width: `${pct}%` }} />
      )}
    </div>
  )
}

export function ScanBanner({ scan, className }: { scan: ScanStatus; className?: string }) {
  if (scan.state === 'scanning') {
    return (
      <div
        role="status"
        className={cx('flex items-center gap-4 rounded-2xl border border-ember-400/20 bg-ember-400/[0.06] px-4 py-3', className)}
      >
        <RefreshCw className="size-5 shrink-0 animate-spin text-ember-300 [animation-duration:2s]" />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-ink-50">Scanning this library</p>
          <p className="truncate text-xs text-ink-300">{scanLabel(scan)}</p>
          <ScanProgress scan={scan} className="mt-2" />
        </div>
      </div>
    )
  }
  if (scan.state === 'error') {
    return (
      <div role="alert" className={cx('flex items-start gap-3 rounded-2xl border border-red-400/25 bg-red-500/[0.07] px-4 py-3', className)}>
        <CircleExclamationPoint className="mt-0.5 size-5 shrink-0 text-red-300" />
        <div className="min-w-0">
          <p className="text-sm font-medium text-ink-50">The last scan failed</p>
          {scan.error && <p className="mt-0.5 text-xs break-words text-red-200/80">{scan.error}</p>}
        </div>
      </div>
    )
  }
  return null
}
