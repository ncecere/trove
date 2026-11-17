import type { ScanStatus } from '../api/types'

export const SCAN_PHASE: Record<NonNullable<ScanStatus['phase']>, string> = {
  walking: 'Finding files',
  indexing: 'Indexing titles',
  metadata: 'Fetching artwork & details',
}

export function scanPercent(s: ScanStatus): number | null {
  if (s.state !== 'scanning' || s.total <= 0) return null
  return Math.min(100, Math.round((s.done / s.total) * 100))
}

export function scanLabel(scan: ScanStatus): string {
  const phase = scan.phase ? SCAN_PHASE[scan.phase] : 'Scanning'
  if (scan.total > 0) return `${phase} · ${scan.done.toLocaleString()} of ${scan.total.toLocaleString()}`
  if (scan.done > 0) return `${phase} · ${scan.done.toLocaleString()} so far`
  return `${phase}…`
}
