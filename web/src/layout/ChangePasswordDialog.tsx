import { useState, type FormEvent } from 'react'
import { errorMessage, isApiError } from '../api/client'
import { useChangePassword } from '../api/hooks'
import { Dialog } from '../components/Dialog'
import { TextField } from '../components/Field'
import { Spinner } from '../components/States'
import { useToast } from '../lib/toast'
import { buttonClass } from '../lib/ui'
import { passwordError } from '../lib/validation'

export function ChangePasswordDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} onClose={onClose} title="Change password" description="You’ll stay signed in here; other devices are signed out." size="sm">
      {open && <ChangePasswordForm onDone={onClose} />}
    </Dialog>
  )
}

function ChangePasswordForm({ onDone }: { onDone: () => void }) {
  const toast = useToast()
  const change = useChangePassword()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [touched, setTouched] = useState(false)

  const nErr = passwordError(next)
  const cErr = confirm !== next ? 'Passwords don’t match' : null
  const serverErr = change.error
    ? isApiError(change.error) && [400, 401, 403].includes(change.error.status)
      ? change.error.status === 401
        ? 'Current password is incorrect'
        : change.error.message
      : errorMessage(change.error)
    : null

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setTouched(true)
    if (!current || nErr || cErr) return
    change.mutate(
      { currentPassword: current, newPassword: next },
      {
        onSuccess: () => {
          toast.success('Password changed', 'Other sessions have been signed out.')
          onDone()
        },
      },
    )
  }

  return (
    <form onSubmit={submit} className="space-y-4" noValidate>
      <TextField
        label="Current password"
        type="password"
        autoComplete="current-password"
        autoFocus
        value={current}
        onChange={(e) => setCurrent(e.target.value)}
        error={touched && !current ? 'Enter your current password' : serverErr}
      />
      <TextField
        label="New password"
        type="password"
        autoComplete="new-password"
        value={next}
        onChange={(e) => setNext(e.target.value)}
        hint="At least 8 characters"
        error={touched ? nErr : null}
      />
      <TextField
        label="Confirm new password"
        type="password"
        autoComplete="new-password"
        value={confirm}
        onChange={(e) => setConfirm(e.target.value)}
        error={touched ? cErr : null}
      />
      <div className="flex justify-end gap-2 pt-2">
        <button type="button" className={buttonClass('ghost')} onClick={onDone}>
          Cancel
        </button>
        <button type="submit" className={buttonClass('primary')} disabled={change.isPending}>
          {change.isPending && <Spinner className="size-4" />}
          Change password
        </button>
      </div>
    </form>
  )
}
