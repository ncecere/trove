import { Download, Lock, Pencil, Shield, Trash, UserPlus, Users } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { errorMessage, isApiError } from '../../api/client'
import { useCreateUser, useDeleteUser, useLibraries, useUpdateUser, useUser, useUsers } from '../../api/hooks'
import type { Role, User, UserUpdate } from '../../api/types'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Checkbox, TextField, Toggle } from '../../components/Field'
import { EmptyState, ErrorState, Skeleton, Spinner } from '../../components/States'
import { formatDateTime, relativeTime } from '../../lib/format'
import { LIBRARY_ICON } from '../../lib/icons'
import { useToast } from '../../lib/toast'
import { buttonClass, cardClass, cx } from '../../lib/ui'
import { passwordError, usernameError } from '../../lib/validation'
import { AdminPage } from './AdminHeader'

function conflictMessage(e: unknown, fallback: string): string {
  if (isApiError(e) && e.status === 409) {
    // the API's message is the most specific; fall back to our own wording
    return e.message || fallback
  }
  return errorMessage(e)
}

export default function UsersAdmin() {
  const users = useUsers()
  const me = useUser()
  const libs = useLibraries()
  const [editing, setEditing] = useState<User | null>(null)
  const [creating, setCreating] = useState(false)
  const [deleting, setDeleting] = useState<User | null>(null)
  const libName = (id: number) => libs.data?.find((l) => l.id === id)?.name ?? `#${id}`
  const adminCount = users.data?.filter((u) => u.role === 'admin').length ?? 0

  return (
    <AdminPage
      title="Users"
      description="Who can sign in, what they can see, and whether they can download."
      actions={
        <button type="button" className={buttonClass('primary')} onClick={() => setCreating(true)}>
          <UserPlus className="size-4" /> Add user
        </button>
      }
    >
      {users.isPending ? (
        <div className="space-y-2">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-20 rounded-2xl" />
          ))}
        </div>
      ) : users.isError ? (
        <ErrorState error={users.error} onRetry={() => void users.refetch()} />
      ) : users.data.length === 0 ? (
        <EmptyState icon={<Users className="size-7" />} title="No users" />
      ) : (
        <ul className={cx(cardClass, 'divide-y divide-white/[0.05] overflow-hidden')}>
          {users.data.map((u) => {
            const isMe = u.id === me.id
            const lastAdmin = u.role === 'admin' && adminCount <= 1
            return (
              <li key={u.id} className="flex flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:px-5">
                <div className="flex min-w-0 flex-1 items-center gap-3.5">
                  <span
                    className={cx(
                      'flex size-10 shrink-0 items-center justify-center rounded-full text-sm font-semibold uppercase',
                      u.role === 'admin' ? 'bg-ember-gradient text-ink-950' : 'bg-ink-700 text-ink-100',
                    )}
                    aria-hidden="true"
                  >
                    {u.username.slice(0, 1)}
                  </span>
                  <div className="min-w-0">
                    <p className="flex flex-wrap items-center gap-2 text-sm font-semibold text-ink-50">
                      <span className="truncate">{u.username}</span>
                      {isMe && <span className="rounded bg-ink-700 px-1.5 py-0.5 text-[0.65rem] font-medium text-ink-200">You</span>}
                      {u.role === 'admin' && (
                        <span className="inline-flex items-center gap-1 rounded bg-ember-400/15 px-1.5 py-0.5 text-[0.65rem] font-medium text-ember-200">
                          <Shield className="size-3" /> Admin
                        </span>
                      )}
                    </p>
                    <p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-400">
                      <span className="inline-flex items-center gap-1">
                        {u.canDownload ? <Download className="size-3.5" /> : <Lock className="size-3.5" />}
                        {u.canDownload ? 'Can download' : 'Browse only'}
                      </span>
                      <span>
                        {u.libraryIds === null
                          ? 'All libraries'
                          : u.libraryIds.length === 0
                            ? 'No libraries'
                            : u.libraryIds.map(libName).join(', ')}
                      </span>
                      <span title={formatDateTime(u.lastLoginAt)}>
                        {u.lastLoginAt ? `Last seen ${relativeTime(u.lastLoginAt)}` : 'Never signed in'}
                      </span>
                    </p>
                  </div>
                </div>
                <div className="flex shrink-0 gap-1.5 self-end sm:self-auto">
                  <button type="button" className={buttonClass('outline', 'sm')} onClick={() => setEditing(u)} aria-label={`Edit ${u.username}`}>
                    <Pencil className="size-4" /> Edit
                  </button>
                  <button
                    type="button"
                    className={buttonClass('ghost', 'sm', 'w-9 px-0 hover:text-red-300')}
                    onClick={() => setDeleting(u)}
                    disabled={isMe || lastAdmin}
                    aria-label={`Delete ${u.username}`}
                    title={isMe ? 'You can’t delete your own account' : lastAdmin ? 'The last admin can’t be deleted' : 'Delete'}
                  >
                    <Trash className="size-4" />
                  </button>
                </div>
              </li>
            )
          })}
        </ul>
      )}

      <UserDialog open={creating} onClose={() => setCreating(false)} />
      <UserDialog open={!!editing} user={editing ?? undefined} onClose={() => setEditing(null)} lastAdmin={editing?.role === 'admin' && adminCount <= 1} />
      <DeleteUser user={deleting} onClose={() => setDeleting(null)} />
    </AdminPage>
  )
}

function UserDialog({ open, user, onClose, lastAdmin }: { open: boolean; user?: User; onClose: () => void; lastAdmin?: boolean }) {
  return (
    <Dialog open={open} onClose={onClose} title={user ? `Edit ${user.username}` : 'Add a user'} size="md">
      {open && <UserForm key={user?.id ?? 'new'} user={user} onDone={onClose} lastAdmin={!!lastAdmin} />}
    </Dialog>
  )
}

function UserForm({ user, onDone, lastAdmin }: { user?: User; onDone: () => void; lastAdmin: boolean }) {
  const toast = useToast()
  const libs = useLibraries()
  const create = useCreateUser()
  const update = useUpdateUser()
  const [username, setUsername] = useState(user?.username ?? '')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>(user?.role ?? 'user')
  const [canDownload, setCanDownload] = useState(user?.canDownload ?? true)
  const [allLibs, setAllLibs] = useState(user ? user.libraryIds === null : true)
  const [libIds, setLibIds] = useState<number[]>(user?.libraryIds ?? [])
  const [touched, setTouched] = useState(false)
  const pending = create.isPending || update.isPending
  const err = create.error ?? update.error

  const uErr = usernameError(username)
  const pErr = user ? (password ? passwordError(password) : null) : passwordError(password)
  const isAdmin = role === 'admin'

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setTouched(true)
    if (uErr || pErr) return
    const libraryIds = isAdmin || allLibs ? null : libIds
    if (!user) {
      create.mutate(
        { username, password, role, canDownload, libraryIds },
        {
          onSuccess: (u) => {
            toast.success(`Added ${u.username}`)
            onDone()
          },
        },
      )
      return
    }
    const patch: UserUpdate = {}
    if (username !== user.username) patch.username = username
    if (password) patch.password = password
    if (role !== user.role) patch.role = role
    if (canDownload !== user.canDownload) patch.canDownload = canDownload
    if (JSON.stringify(libraryIds) !== JSON.stringify(user.libraryIds)) patch.libraryIds = libraryIds
    if (!Object.keys(patch).length) return onDone()
    update.mutate(
      { id: user.id, body: patch },
      {
        onSuccess: (u) => {
          toast.success(`Saved ${u.username}`)
          onDone()
        },
      },
    )
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-5">
      <TextField
        label="Username"
        value={username}
        onChange={(e) => setUsername(e.target.value)}
        autoCapitalize="none"
        spellCheck={false}
        autoComplete="off"
        autoFocus={!user}
        error={touched ? uErr : null}
        hint="3–32 characters: letters, numbers, . _ -"
      />
      <TextField
        label={user ? 'New password' : 'Password'}
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        autoComplete="new-password"
        placeholder={user ? 'Leave blank to keep the current password' : undefined}
        error={touched ? pErr : null}
        hint={user ? 'Setting a new password signs them out everywhere.' : 'At least 8 characters'}
      />

      <fieldset>
        <legend className="mb-2 text-sm font-medium text-ink-200">Role</legend>
        <div className="grid grid-cols-2 gap-2">
          {(['user', 'admin'] as const).map((r) => {
            const active = role === r
            const disabled = r === 'user' && !!user && user.role === 'admin' && lastAdmin
            return (
              <label
                key={r}
                className={cx(
                  'flex cursor-pointer flex-col rounded-xl border p-3 transition has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ember-400',
                  active ? 'border-ember-400/60 bg-ember-400/[0.07]' : 'border-white/[0.08] hover:border-white/20',
                  disabled && 'pointer-events-none opacity-50',
                )}
              >
                <input type="radio" name="role" className="sr-only" checked={active} disabled={disabled} onChange={() => setRole(r)} />
                <span className="text-sm font-semibold text-ink-50">{r === 'admin' ? 'Admin' : 'Member'}</span>
                <span className="mt-0.5 text-xs text-ink-400">
                  {r === 'admin' ? 'Manages libraries, users & settings' : 'Browses the libraries they’re given'}
                </span>
              </label>
            )
          })}
        </div>
        {user && user.role === 'admin' && lastAdmin && (
          <p className="mt-2 text-xs text-ink-400">This is the only admin, so it can’t be changed to a member.</p>
        )}
      </fieldset>

      <Toggle
        checked={canDownload}
        onChange={setCanDownload}
        label="Can download"
        description="Off = browse only: they can see titles and details but not download files."
      />

      <fieldset>
        <legend className="mb-2 text-sm font-medium text-ink-200">Library access</legend>
        {isAdmin ? (
          <p className="rounded-xl bg-ink-900 px-3 py-2.5 text-sm text-ink-400">Admins can always see every library.</p>
        ) : (
          <>
            <Toggle
              checked={allLibs}
              onChange={setAllLibs}
              label="All libraries"
              description="Including libraries added later."
            />
            {!allLibs && (
              <div className="mt-3 rounded-xl border border-white/[0.06] bg-ink-950/40 p-1.5">
                {libs.data?.length ? (
                  libs.data.map((l) => {
                    const Icon = LIBRARY_ICON[l.type]
                    return (
                      <Checkbox
                        key={l.id}
                        label={l.name}
                        icon={<Icon className="size-4" />}
                        checked={libIds.includes(l.id)}
                        onChange={(v) => setLibIds((ids) => (v ? [...ids, l.id] : ids.filter((i) => i !== l.id)))}
                      />
                    )
                  })
                ) : (
                  <p className="px-2 py-3 text-sm text-ink-500">No libraries yet.</p>
                )}
                {libIds.length === 0 && libs.data?.length ? (
                  <p className="px-2 pt-1 pb-1.5 text-xs text-ember-200/80">With nothing ticked they won’t see any titles.</p>
                ) : null}
              </div>
            )}
          </>
        )}
      </fieldset>

      {err && (
        <p role="alert" className="rounded-xl border border-red-400/25 bg-red-500/10 px-3 py-2.5 text-sm text-red-200">
          {isApiError(err) && err.status === 409
            ? conflictMessage(err, 'That change isn’t allowed: the username may be taken, or this is the last admin.')
            : errorMessage(err)}
        </p>
      )}

      <div className="flex justify-end gap-2 border-t border-white/[0.06] pt-5">
        <button type="button" className={buttonClass('ghost')} onClick={onDone} disabled={pending}>
          Cancel
        </button>
        <button type="submit" className={buttonClass('primary')} disabled={pending}>
          {pending && <Spinner className="size-4" />}
          {user ? 'Save changes' : 'Add user'}
        </button>
      </div>
    </form>
  )
}

function DeleteUser({ user, onClose }: { user: User | null; onClose: () => void }) {
  const del = useDeleteUser()
  const toast = useToast()
  return (
    <ConfirmDialog
      open={!!user}
      onClose={onClose}
      danger
      pending={del.isPending}
      title={user ? `Delete ${user.username}?` : ''}
      confirmLabel="Delete user"
      icon={<Trash className="size-4" />}
      onConfirm={() =>
        user &&
        del.mutate(user.id, {
          onSuccess: () => {
            toast.success(`Deleted ${user.username}`)
            onClose()
          },
          onError: (e) => {
            toast.error(
              'Couldn’t delete user',
              conflictMessage(e, 'The last admin can’t be deleted, and you can’t delete yourself.'),
            )
            onClose()
          },
        })
      }
    >
      They’ll be signed out and won’t be able to sign in again. Their download history stays in the activity log.
    </ConfirmDialog>
  )
}
