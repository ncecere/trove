import { CircleExclamationPoint, LoaderCircle, RefreshCw } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { errorMessage, isApiError } from '../api/client'
import { buttonClass, cx } from '../lib/ui'

export function Spinner({ className, label = 'Loading' }: { className?: string; label?: string }) {
  return <LoaderCircle className={cx('animate-spin', className ?? 'size-5')} aria-label={label} role="status" />
}

interface EmptyProps {
  icon?: ReactNode
  title: string
  children?: ReactNode
  action?: ReactNode
  className?: string
}

export function EmptyState({ icon, title, children, action, className }: EmptyProps) {
  return (
    <div className={cx('mx-auto flex max-w-md flex-col items-center px-6 py-16 text-center', className)}>
      {icon && (
        <div className="mb-5 flex size-16 items-center justify-center rounded-2xl border border-white/[0.06] bg-gradient-to-b from-ink-800 to-ink-900 text-ember-300 shadow-card">
          {icon}
        </div>
      )}
      <h2 className="text-lg font-semibold text-ink-50">{title}</h2>
      {children && <div className="mt-2 text-sm leading-relaxed text-ink-400">{children}</div>}
      {action && <div className="mt-6 flex flex-wrap justify-center gap-3">{action}</div>}
    </div>
  )
}

export function ErrorState({
  error,
  onRetry,
  title,
  className,
}: {
  error: unknown
  onRetry?: () => void
  title?: string
  className?: string
}) {
  const notFound = isApiError(error) && error.status === 404
  return (
    <EmptyState
      className={className}
      icon={<CircleExclamationPoint className="size-7 text-red-300" />}
      title={title ?? (notFound ? 'Not found' : 'Couldn’t load this')}
      action={
        notFound ? (
          <Link to="/" className={buttonClass('secondary')}>
            Back to home
          </Link>
        ) : onRetry ? (
          <button type="button" className={buttonClass('secondary')} onClick={onRetry}>
            <RefreshCw className="size-4" /> Try again
          </button>
        ) : undefined
      }
    >
      {notFound ? 'It may have been removed, or you may not have access to it.' : errorMessage(error)}
    </EmptyState>
  )
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cx('skeleton rounded-lg', className)} aria-hidden="true" />
}

export function CardSkeleton({ square }: { square?: boolean }) {
  return (
    <div aria-hidden="true">
      <Skeleton className={cx('rounded-xl', square ? 'aspect-square' : 'aspect-[2/3]')} />
      <Skeleton className="mt-2 h-3.5 w-4/5" />
      <Skeleton className="mt-1.5 h-3 w-2/5" />
    </div>
  )
}
