import { useId } from 'react'
import { cx } from '../lib/ui'

export function LogoMark({ className }: { className?: string }) {
  const id = useId()
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden="true">
      <defs>
        <linearGradient id={`${id}-g`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#ffd27a" />
          <stop offset="1" stopColor="#e9772f" />
        </linearGradient>
      </defs>
      <path d="M16 3.5 28 12 16 28.5 4 12Z" fill={`url(#${id}-g)`} />
      <path
        d="M4 12h24M16 3.5 11.2 12 16 28.5 20.8 12 16 3.5"
        fill="none"
        stroke="#0a0c11"
        strokeWidth="1.3"
        strokeLinejoin="round"
        opacity=".5"
      />
    </svg>
  )
}

export function Logo({ className, compact }: { className?: string; compact?: boolean }) {
  return (
    <span className={cx('inline-flex items-center gap-2.5', className)}>
      <LogoMark className="size-7 drop-shadow-[0_4px_12px_rgb(245_154_54/0.35)]" />
      {!compact && <span className="text-[1.35rem] font-semibold tracking-[-0.03em] text-ink-50">trove</span>}
    </span>
  )
}
