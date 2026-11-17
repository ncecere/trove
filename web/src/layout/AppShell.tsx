import {
  Activity,
  ChevronDown,
  House,
  KeyRound,
  LayoutGrid,
  LibraryBig,
  LogOut,
  Search,
  Settings,
  Shield,
  Users,
  X,
  type LucideIcon,
} from 'lucide-react'
import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router'
import { useLibraries, useLogout, useUser } from '../api/hooks'
import type { Library } from '../api/types'
import { Logo } from '../components/Logo'
import { Menu } from '../components/Menu'
import { Skeleton } from '../components/States'
import { LIBRARY_ICON } from '../lib/icons'
import { cx } from '../lib/ui'
import { ChangePasswordDialog } from './ChangePasswordDialog'
import { SearchField } from './SearchField'

const ADMIN_LINKS: { to: string; label: string; icon: LucideIcon }[] = [
  { to: '/admin/libraries', label: 'Libraries', icon: LibraryBig },
  { to: '/admin/users', label: 'Users', icon: Users },
  { to: '/admin/settings', label: 'Settings', icon: Settings },
  { to: '/admin/activity', label: 'Activity', icon: Activity },
]

function NavItem({
  to,
  icon: Icon,
  children,
  end,
  trailing,
  onClick,
}: {
  to: string
  icon: LucideIcon
  children: ReactNode
  end?: boolean
  trailing?: ReactNode
  onClick?: () => void
}) {
  return (
    <NavLink
      to={to}
      end={end}
      onClick={onClick}
      className={({ isActive }) =>
        cx(
          'group relative flex min-h-10 items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition',
          isActive ? 'bg-ink-800 text-ink-50' : 'text-ink-300 hover:bg-ink-850 hover:text-ink-50',
        )
      }
    >
      {({ isActive }) => (
        <>
          {isActive && (
            <span className="bg-ember-gradient absolute top-1/2 left-0 h-5 w-[3px] -translate-y-1/2 rounded-full" aria-hidden="true" />
          )}
          <Icon className={cx('size-[1.15rem] shrink-0', isActive ? 'text-ember-300' : 'text-ink-400 group-hover:text-ink-200')} />
          <span className="min-w-0 flex-1 truncate">{children}</span>
          {trailing}
        </>
      )}
    </NavLink>
  )
}

function ScanDot({ lib }: { lib: Library }) {
  if (lib.scan.state === 'scanning')
    return (
      <span className="relative flex size-2" title="Scanning" aria-label="Scanning">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-ember-400 opacity-60" />
        <span className="relative inline-flex size-2 rounded-full bg-ember-400" />
      </span>
    )
  if (lib.scan.state === 'error') return <span className="size-2 rounded-full bg-red-400" title="Last scan failed" aria-label="Last scan failed" />
  return null
}

function NavContent({ onNavigate }: { onNavigate?: () => void }) {
  const user = useUser()
  const libs = useLibraries()
  return (
    <nav aria-label="Main" className="flex flex-col gap-6">
      <div className="flex flex-col gap-0.5">
        <NavItem to="/" icon={House} end onClick={onNavigate}>
          Home
        </NavItem>
      </div>
      <div>
        <p className="mb-2 px-3 text-[0.7rem] font-semibold tracking-[0.14em] text-ink-500 uppercase">Libraries</p>
        <div className="flex flex-col gap-0.5">
          {libs.isPending &&
            Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="mx-3 my-2.5 h-5 w-[calc(100%-1.5rem)]" />)}
          {libs.data?.map((l) => (
            <NavItem key={l.id} to={`/library/${l.id}`} icon={LIBRARY_ICON[l.type]} trailing={<ScanDot lib={l} />} onClick={onNavigate}>
              {l.name}
            </NavItem>
          ))}
          {libs.data?.length === 0 && (
            <p className="px-3 py-2 text-sm text-ink-500">
              {user.role === 'admin' ? (
                <Link to="/admin/libraries" onClick={onNavigate} className="text-ember-300 hover:underline">
                  Add a library
                </Link>
              ) : (
                'No libraries yet'
              )}
            </p>
          )}
        </div>
      </div>
      {user.role === 'admin' && (
        <div>
          <p className="mb-2 flex items-center gap-1.5 px-3 text-[0.7rem] font-semibold tracking-[0.14em] text-ink-500 uppercase">
            <Shield className="size-3" /> Admin
          </p>
          <div className="flex flex-col gap-0.5">
            {ADMIN_LINKS.map((l) => (
              <NavItem key={l.to} to={l.to} icon={l.icon} onClick={onNavigate}>
                {l.label}
              </NavItem>
            ))}
          </div>
        </div>
      )}
    </nav>
  )
}

function Avatar({ name, className }: { name: string; className?: string }) {
  return (
    <span
      className={cx(
        'bg-ember-gradient inline-flex shrink-0 items-center justify-center rounded-full font-semibold text-ink-950 uppercase',
        className ?? 'size-8 text-sm',
      )}
      aria-hidden="true"
    >
      {name.slice(0, 1)}
    </span>
  )
}

function UserMenu() {
  const user = useUser()
  const logout = useLogout()
  const navigate = useNavigate()
  const [pwOpen, setPwOpen] = useState(false)
  return (
    <>
      <Menu
        header={
          <div className="flex items-center gap-3">
            <Avatar name={user.username} className="size-9 text-sm" />
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-ink-50">{user.username}</p>
              <p className="text-xs text-ink-400">
                {user.role === 'admin' ? 'Administrator' : 'Member'}
                {!user.canDownload && ' · browse only'}
              </p>
            </div>
          </div>
        }
        items={[
          { label: 'Change password', icon: <KeyRound className="size-4" />, onSelect: () => setPwOpen(true) },
          {
            label: 'Sign out',
            icon: <LogOut className="size-4" />,
            danger: true,
            onSelect: () => logout.mutate(undefined, { onSettled: () => navigate('/login', { replace: true }) }),
          },
        ]}
        trigger={({ open, toggle, id, menuId }) => (
          <button
            id={id}
            type="button"
            onClick={toggle}
            aria-haspopup="menu"
            aria-expanded={open}
            aria-controls={menuId}
            aria-label={`Account menu for ${user.username}`}
            className="flex h-10 items-center gap-2 rounded-full p-1 transition hover:bg-ink-800 sm:pr-2.5"
          >
            <Avatar name={user.username} />
            <span className="hidden max-w-32 truncate text-sm font-medium text-ink-200 sm:block">{user.username}</span>
            <ChevronDown className="hidden size-4 text-ink-400 sm:block" />
          </button>
        )}
      />
      <ChangePasswordDialog open={pwOpen} onClose={() => setPwOpen(false)} />
    </>
  )
}

function MobileDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    document.documentElement.style.overflow = 'hidden'
    return () => {
      window.removeEventListener('keydown', onKey)
      document.documentElement.style.overflow = ''
    }
  }, [open, onClose])

  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 lg:hidden" role="dialog" aria-modal="true" aria-label="Browse">
      <button type="button" className="absolute inset-0 animate-fade-in bg-black/60 backdrop-blur-sm" onClick={onClose} aria-label="Close menu" tabIndex={-1} />
      <div className="absolute inset-x-0 bottom-0 max-h-[85dvh] animate-slide-up overflow-y-auto rounded-t-3xl border-t border-white/[0.08] bg-ink-900 px-4 pt-3 pb-[calc(1.5rem+env(safe-area-inset-bottom))] shadow-lift">
        <div className="mx-auto mb-3 h-1 w-10 rounded-full bg-ink-600" aria-hidden="true" />
        <div className="mb-4 flex items-center justify-between px-1">
          <Logo />
          <button
            type="button"
            onClick={onClose}
            className="inline-flex size-10 items-center justify-center rounded-xl text-ink-300 hover:bg-ink-800"
            aria-label="Close menu"
            autoFocus
          >
            <X className="size-5" />
          </button>
        </div>
        <NavContent onNavigate={onClose} />
      </div>
    </div>
  )
}

function BottomTab({
  to,
  icon: Icon,
  label,
  end,
  onClick,
  active,
}: {
  to?: string
  icon: LucideIcon
  label: string
  end?: boolean
  onClick?: () => void
  active?: boolean
}) {
  const cls = (isActive: boolean) =>
    cx(
      'flex min-h-14 flex-1 flex-col items-center justify-center gap-1 text-[0.68rem] font-medium transition',
      isActive ? 'text-ember-300' : 'text-ink-400 hover:text-ink-100',
    )
  if (!to)
    return (
      <button type="button" onClick={onClick} className={cls(!!active)} aria-expanded={active}>
        <Icon className="size-5" />
        {label}
      </button>
    )
  return (
    <NavLink to={to} end={end} className={({ isActive }) => cls(isActive)}>
      <Icon className="size-5" />
      {label}
    </NavLink>
  )
}

export function AppShell() {
  const user = useUser()
  const [drawer, setDrawer] = useState(false)
  const closeDrawer = useCallback(() => setDrawer(false), [])
  const loc = useLocation()
  const [lastPath, setLastPath] = useState(loc.pathname)
  if (lastPath !== loc.pathname) {
    setLastPath(loc.pathname)
    setDrawer(false)
  }

  // reset scroll on route change (not on query-string changes)
  useEffect(() => {
    window.scrollTo(0, 0)
  }, [loc.pathname])

  return (
    <div className="min-h-dvh">
      <a
        href="#main"
        className="sr-only z-[200] rounded-lg bg-ink-800 px-4 py-2 focus:not-sr-only focus:fixed focus:top-3 focus:left-3"
      >
        Skip to content
      </a>

      {/* desktop sidebar */}
      <aside className="scrollbar-thin fixed inset-y-0 left-0 z-30 hidden w-64 flex-col overflow-y-auto border-r border-white/[0.05] bg-ink-950/95 px-3 pt-5 pb-6 lg:flex">
        <Link to="/" className="mb-8 self-start rounded-lg px-3" aria-label="Trove home">
          <Logo />
        </Link>
        <NavContent />
      </aside>

      <div className="lg:pl-64">
        <header className="sticky top-0 z-20 border-b border-white/[0.04] bg-ink-950/75 pt-[env(safe-area-inset-top)] backdrop-blur-xl">
          <div className="flex h-16 items-center gap-3 px-4 sm:px-6 lg:px-8">
            <Link to="/" className="shrink-0 rounded-lg lg:hidden" aria-label="Trove home">
              <span className="sm:hidden">
                <Logo compact />
              </span>
              <span className="hidden sm:block">
                <Logo />
              </span>
            </Link>
            <SearchField className="min-w-0 flex-1 lg:max-w-xl" />
            <div className="ml-auto shrink-0">
              <UserMenu />
            </div>
          </div>
        </header>

        <main id="main" className="min-w-0 pb-[calc(5rem+env(safe-area-inset-bottom))] lg:pb-16">
          <Outlet />
        </main>
      </div>

      {/* mobile bottom bar */}
      <nav
        aria-label="Primary"
        className="fixed inset-x-0 bottom-0 z-30 flex border-t border-white/[0.06] bg-ink-950/90 pb-[env(safe-area-inset-bottom)] backdrop-blur-xl lg:hidden"
      >
        <BottomTab to="/" icon={House} label="Home" end />
        <BottomTab icon={LayoutGrid} label="Libraries" onClick={() => setDrawer(true)} active={drawer} />
        <BottomTab to="/search" icon={Search} label="Search" />
        {user.role === 'admin' && <BottomTab to="/admin" icon={Shield} label="Admin" />}
      </nav>
      <MobileDrawer open={drawer} onClose={closeDrawer} />
    </div>
  )
}
