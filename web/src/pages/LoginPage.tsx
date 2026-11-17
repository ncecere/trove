import { CircleExclamationPoint, LogIn } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router'
import { errorMessage, isApiError } from '../api/client'
import { useLogin, useMe, useSetupStatus } from '../api/hooks'
import { TextField } from '../components/Field'
import { Spinner } from '../components/States'
import { safeNext } from '../lib/nav'
import { buttonClass } from '../lib/ui'
import { AuthLayout } from './AuthLayout'
import { Splash } from '../auth/Guards'

function loginError(e: unknown): string {
  if (isApiError(e)) {
    if (e.status === 401) return 'That username and password don’t match. Check them and try again.'
    if (e.status === 429) return 'Too many failed attempts. Wait a minute before trying again.'
  }
  return errorMessage(e)
}

export function LoginPage() {
  const [params] = useSearchParams()
  const next = safeNext(params.get('next'))
  const navigate = useNavigate()
  const setup = useSetupStatus()
  const me = useMe()
  const login = useLogin()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  if (setup.isPending || me.isPending) return <Splash />
  if (setup.data?.needsSetup) return <Navigate to="/setup" replace />
  if (me.data) return <Navigate to={next} replace />

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password) return
    login.mutate({ username: username.trim(), password }, { onSuccess: () => navigate(next, { replace: true }) })
  }

  const rateLimited = isApiError(login.error) && login.error.status === 429

  return (
    <AuthLayout>
      <h1 className="text-xl font-semibold tracking-tight text-ink-50">Welcome back</h1>
      <p className="mt-1 text-sm text-ink-400">Sign in to browse the library.</p>
      <form onSubmit={submit} className="mt-6 space-y-4" noValidate>
        <TextField
          label="Username"
          name="username"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          autoFocus
          required
          value={username}
          onChange={(e) => setUsername(e.target.value)}
        />
        <TextField
          label="Password"
          name="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        {login.isError && (
          <div
            role="alert"
            className="flex gap-2.5 rounded-xl border border-red-400/25 bg-red-500/10 px-3 py-2.5 text-sm text-red-200"
          >
            <CircleExclamationPoint className="mt-0.5 size-4 shrink-0" />
            <span>{loginError(login.error)}</span>
          </div>
        )}
        <button
          type="submit"
          className={buttonClass('primary', 'lg', 'w-full')}
          disabled={login.isPending || !username.trim() || !password || rateLimited}
        >
          {login.isPending ? <Spinner className="size-4" /> : <LogIn className="size-4" />}
          Sign in
        </button>
        {rateLimited && (
          <button
            type="button"
            className="w-full text-center text-xs text-ink-400 underline-offset-2 hover:text-ink-200 hover:underline"
            onClick={() => login.reset()}
          >
            Try again
          </button>
        )}
      </form>
      <p className="mt-6 text-center text-xs text-ink-500">Forgot your password? Ask the person who runs this Trove.</p>
    </AuthLayout>
  )
}
