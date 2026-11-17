import type { ReactNode } from 'react'
import { NavLink } from 'react-router'
import { useDocumentTitle } from '../../lib/hooks'
import { cx } from '../../lib/ui'

const TABS = [
  { to: '/admin/libraries', label: 'Libraries' },
  { to: '/admin/users', label: 'Users' },
  { to: '/admin/settings', label: 'Settings' },
  { to: '/admin/activity', label: 'Activity' },
]

export function AdminPage({
  title,
  description,
  actions,
  children,
}: {
  title: string
  description?: ReactNode
  actions?: ReactNode
  children: ReactNode
}) {
  useDocumentTitle(`${title} · Admin`)
  return (
    <div className="mx-auto max-w-6xl px-4 pt-6 sm:px-6 sm:pt-8 lg:px-8">
      <nav aria-label="Admin sections" className="scrollbar-none -mx-4 mb-6 flex gap-1 overflow-x-auto px-4 lg:hidden">
        {TABS.map((t) => (
          <NavLink
            key={t.to}
            to={t.to}
            className={({ isActive }) =>
              cx(
                'flex h-9 shrink-0 items-center rounded-full px-3.5 text-sm font-medium transition',
                isActive ? 'bg-ink-50 text-ink-950' : 'text-ink-300 hover:bg-ink-800',
              )
            }
          >
            {t.label}
          </NavLink>
        ))}
      </nav>
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div className="min-w-0">
          <p className="text-xs font-semibold tracking-[0.14em] text-ember-300/80 uppercase">Admin</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-ink-50 sm:text-3xl">{title}</h1>
          {description && <p className="mt-1 max-w-2xl text-sm text-ink-400">{description}</p>}
        </div>
        {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
      </header>
      {children}
    </div>
  )
}
