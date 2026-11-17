import { ChevronLeft, ChevronRight, Download, History, LogIn, Package } from 'lucide-react'
import { Link, useSearchParams } from 'react-router'
import { useActivity } from '../../api/hooks'
import type { Activity } from '../../api/types'
import { EmptyState, ErrorState, Skeleton } from '../../components/States'
import { formatBytes, formatDateTime, relativeTime } from '../../lib/format'
import { buttonClass, cardClass, cx } from '../../lib/ui'
import { AdminPage } from './AdminHeader'

const LIMIT = 50

const ACTION: Record<Activity['action'], { label: string; icon: typeof Download }> = {
  download_file: { label: 'Downloaded file', icon: Download },
  download_item: { label: 'Downloaded zip', icon: Package },
  login: { label: 'Signed in', icon: LogIn },
}

export default function ActivityAdmin() {
  const [params, setParams] = useSearchParams()
  const page = Math.max(1, Number(params.get('page')) || 1)
  const act = useActivity(page, LIMIT)
  const pages = act.data ? Math.max(1, Math.ceil(act.data.total / act.data.limit)) : 1
  const go = (p: number) => {
    setParams(p > 1 ? { page: String(p) } : {})
    window.scrollTo({ top: 0 })
  }

  return (
    <AdminPage title="Activity" description="Downloads and sign-ins, newest first.">
      {act.isPending ? (
        <Skeleton className="h-96 rounded-2xl" />
      ) : act.isError ? (
        <ErrorState error={act.error} onRetry={() => void act.refetch()} />
      ) : act.data.items.length === 0 ? (
        <EmptyState icon={<History className="size-7" />} title="No activity yet">
          Downloads and sign-ins will be listed here.
        </EmptyState>
      ) : (
        <>
          <div className={cx(cardClass, 'overflow-hidden', act.isPlaceholderData && 'opacity-60')}>
            <div className="scrollbar-thin overflow-x-auto">
              <table className="w-full min-w-[40rem] text-left text-sm">
                <thead className="border-b border-white/[0.06] text-[0.7rem] font-semibold tracking-[0.12em] text-ink-500 uppercase">
                  <tr>
                    <th scope="col" className="px-4 py-3 sm:px-5">When</th>
                    <th scope="col" className="px-4 py-3">User</th>
                    <th scope="col" className="px-4 py-3">Action</th>
                    <th scope="col" className="px-4 py-3">Detail</th>
                    <th scope="col" className="px-4 py-3 text-right sm:px-5">Size</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-white/[0.04]">
                  {act.data.items.map((a) => {
                    const A = ACTION[a.action] ?? { label: a.action, icon: History }
                    return (
                      <tr key={a.id} className="hover:bg-white/[0.02]">
                        <td className="px-4 py-3 whitespace-nowrap text-ink-300 sm:px-5" title={formatDateTime(a.at)}>
                          {relativeTime(a.at)}
                        </td>
                        <td className="px-4 py-3 font-medium whitespace-nowrap text-ink-100">{a.username}</td>
                        <td className="px-4 py-3 whitespace-nowrap">
                          <span className="inline-flex items-center gap-1.5 text-ink-300">
                            <A.icon className="size-4 text-ink-500" />
                            {A.label}
                          </span>
                        </td>
                        <td className="max-w-[22rem] px-4 py-3">
                          {a.itemId && a.itemTitle ? (
                            <Link to={`/item/${a.itemId}`} className="block truncate font-medium text-ink-100 hover:text-ember-200">
                              {a.itemTitle}
                            </Link>
                          ) : a.itemTitle ? (
                            <span className="block truncate text-ink-100">{a.itemTitle}</span>
                          ) : null}
                          {a.detail && (
                            <span className={cx('block truncate text-xs', a.action === 'login' ? 'font-mono text-ink-400' : 'text-ink-500')} title={a.detail}>
                              {a.detail}
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-right whitespace-nowrap text-ink-300 tabular-nums sm:px-5">
                          {a.size !== null ? formatBytes(a.size) : '—'}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </div>
          <nav aria-label="Pagination" className="mt-4 flex items-center justify-between gap-3 text-sm text-ink-400">
            <span>
              Page {page} of {pages} · {act.data.total.toLocaleString()} events
            </span>
            <div className="flex gap-2">
              <button type="button" className={buttonClass('outline', 'sm')} disabled={page <= 1} onClick={() => go(page - 1)}>
                <ChevronLeft className="size-4" /> Newer
              </button>
              <button type="button" className={buttonClass('outline', 'sm')} disabled={page >= pages} onClick={() => go(page + 1)}>
                Older <ChevronRight className="size-4" />
              </button>
            </div>
          </nav>
        </>
      )}
    </AdminPage>
  )
}
