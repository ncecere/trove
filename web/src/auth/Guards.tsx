import { Navigate, Outlet, useLocation } from 'react-router'
import type { ReactNode } from 'react'
import { useMe, useSetupStatus } from '../api/hooks'
import { LogoMark } from '../components/Logo'
import { ErrorState } from '../components/States'

export function Splash() {
  return (
    <div className="flex min-h-dvh items-center justify-center" role="status" aria-label="Loading Trove">
      <LogoMark className="size-12 animate-pulse" />
    </div>
  )
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const setup = useSetupStatus()
  const me = useMe()
  const loc = useLocation()

  if (setup.isPending || me.isPending) return <Splash />
  if (setup.data?.needsSetup) return <Navigate to="/setup" replace />
  if (me.isError) {
    return (
      <div className="flex min-h-dvh items-center justify-center">
        <ErrorState error={me.error} title="Can’t reach Trove" onRetry={() => void me.refetch()} />
      </div>
    )
  }
  if (!me.data) {
    const next = loc.pathname + loc.search
    return <Navigate to={next === '/' ? '/login' : `/login?next=${encodeURIComponent(next)}`} replace />
  }
  return <>{children}</>
}

export function RequireAdmin() {
  const me = useMe()
  if (me.data?.role !== 'admin') return <Navigate to="/" replace />
  return <Outlet />
}
