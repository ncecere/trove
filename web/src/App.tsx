import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import { isApiError, setUnauthorizedHandler } from './api/client'
import { qk } from './api/hooks'
import { RequireAdmin, RequireAuth } from './auth/Guards'
import { Spinner } from './components/States'
import { Toaster } from './components/Toaster'
import { AppShell } from './layout/AppShell'
import { HomePage } from './pages/HomePage'
import { ItemPage } from './pages/item/ItemPage'
import { LibraryPage } from './pages/LibraryPage'
import { LoginPage } from './pages/LoginPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { SearchPage } from './pages/SearchPage'
import { SetupPage } from './pages/SetupPage'

const AdminLibraries = lazy(() => import('./pages/admin/LibrariesAdmin'))
const AdminUsers = lazy(() => import('./pages/admin/UsersAdmin'))
const AdminSettings = lazy(() => import('./pages/admin/SettingsAdmin'))
const AdminActivity = lazy(() => import('./pages/admin/ActivityAdmin'))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: (count, err) => {
        if (isApiError(err) && err.status >= 400 && err.status < 500) return false
        return count < 2
      },
    },
  },
})

// Any 401 from an authenticated call: the session is gone. Clearing "me" makes the
// auth guard redirect to /login?next=<current url>.
setUnauthorizedHandler(() => {
  if (queryClient.getQueryData(qk.me)) queryClient.setQueryData(qk.me, null)
})

function AdminFallback() {
  return (
    <div className="flex justify-center py-24 text-ink-400">
      <Spinner className="size-6" />
    </div>
  )
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <Toaster>
        <BrowserRouter>
          <Routes>
            <Route path="/setup" element={<SetupPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route
              element={
                <RequireAuth>
                  <AppShell />
                </RequireAuth>
              }
            >
              <Route index element={<HomePage />} />
              <Route path="library/:id" element={<LibraryPage />} />
              <Route path="search" element={<SearchPage />} />
              <Route path="item/:id" element={<ItemPage />} />
              <Route path="admin" element={<RequireAdmin />}>
                <Route index element={<Navigate to="libraries" replace />} />
                <Route
                  path="libraries"
                  element={
                    <Suspense fallback={<AdminFallback />}>
                      <AdminLibraries />
                    </Suspense>
                  }
                />
                <Route
                  path="users"
                  element={
                    <Suspense fallback={<AdminFallback />}>
                      <AdminUsers />
                    </Suspense>
                  }
                />
                <Route
                  path="settings"
                  element={
                    <Suspense fallback={<AdminFallback />}>
                      <AdminSettings />
                    </Suspense>
                  }
                />
                <Route
                  path="activity"
                  element={
                    <Suspense fallback={<AdminFallback />}>
                      <AdminActivity />
                    </Suspense>
                  }
                />
              </Route>
              <Route path="*" element={<NotFoundPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </Toaster>
    </QueryClientProvider>
  )
}
