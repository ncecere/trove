import { CircleExclamationPoint, Sparkles } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Navigate, useNavigate } from 'react-router'
import { errorMessage, isApiError } from '../api/client'
import { useSetup, useSetupStatus } from '../api/hooks'
import { TextField } from '../components/Field'
import { Spinner } from '../components/States'
import { Splash } from '../auth/Guards'
import { buttonClass } from '../lib/ui'
import { AuthLayout } from './AuthLayout'
import { passwordError, usernameError } from '../lib/validation'

export function SetupPage() {
  const status = useSetupStatus()
  const setup = useSetup()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [touched, setTouched] = useState(false)

  if (status.isPending) return <Splash />
  if (status.data && !status.data.needsSetup) return <Navigate to="/" replace />

  const uErr = usernameError(username)
  const pErr = passwordError(password)
  const cErr = confirm !== password ? 'Passwords don’t match' : null

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setTouched(true)
    if (uErr || pErr || cErr) return
    setup.mutate(
      { username, password },
      {
        onSuccess: () => navigate('/admin/libraries', { replace: true }),
        onError: (err) => {
          if (isApiError(err) && err.status === 409) void status.refetch()
        },
      },
    )
  }

  return (
    <AuthLayout>
      <div className="mb-1 inline-flex items-center gap-1.5 rounded-full bg-ember-400/10 px-2.5 py-1 text-xs font-medium text-ember-300">
        <Sparkles className="size-3.5" /> First-time setup
      </div>
      <h1 className="mt-3 text-xl font-semibold tracking-tight text-ink-50">Create the admin account</h1>
      <p className="mt-1 text-sm text-ink-400">
        This account manages libraries, users and settings. You can add more people afterwards.
      </p>
      <form onSubmit={submit} className="mt-6 space-y-4" noValidate>
        <TextField
          label="Username"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          autoFocus
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          hint="3–32 characters: letters, numbers, . _ -"
          error={touched ? uErr : null}
        />
        <TextField
          label="Password"
          type="password"
          autoComplete="new-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          hint="At least 8 characters"
          error={touched ? pErr : null}
        />
        <TextField
          label="Confirm password"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          error={touched ? cErr : null}
        />
        {setup.isError && (
          <div
            role="alert"
            className="flex gap-2.5 rounded-xl border border-red-400/25 bg-red-500/10 px-3 py-2.5 text-sm text-red-200"
          >
            <CircleExclamationPoint className="mt-0.5 size-4 shrink-0" />
            <span>
              {isApiError(setup.error) && setup.error.status === 409
                ? 'Trove is already set up. Sign in instead.'
                : errorMessage(setup.error)}
            </span>
          </div>
        )}
        <button type="submit" className={buttonClass('primary', 'lg', 'w-full')} disabled={setup.isPending}>
          {setup.isPending && <Spinner className="size-4" />}
          Create account &amp; continue
        </button>
      </form>
    </AuthLayout>
  )
}
