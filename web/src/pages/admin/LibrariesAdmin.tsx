import { FolderOpen, FolderPlus, LibraryBig, Pencil, RefreshCw, Trash } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router'
import { errorMessage } from '../../api/client'
import {
  useCreateLibrary,
  useDeleteLibrary,
  useLibraries,
  useScanAll,
  useScanLibrary,
  useUpdateLibrary,
} from '../../api/hooks'
import type { Library, LibraryType } from '../../api/types'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { TextField } from '../../components/Field'
import { ScanProgress } from '../../components/ScanBanner'
import { EmptyState, ErrorState, Skeleton, Spinner } from '../../components/States'
import { formatBytes, formatDateTime, LIBRARY_TYPE_LABEL, pluralize, relativeTime } from '../../lib/format'
import { LIBRARY_ICON } from '../../lib/icons'
import { scanLabel } from '../../lib/scan'
import { useToast } from '../../lib/toast'
import { buttonClass, cardClass, cx, inputClass } from '../../lib/ui'
import { AdminPage } from './AdminHeader'
import { FolderPicker } from './FolderPicker'

const TYPES: { type: LibraryType; hint: string }[] = [
  { type: 'movies', hint: 'One folder per film: Movies/Title (Year)/Title (Year).mkv' },
  { type: 'tv', hint: 'Show/Season 01/Show - S01E01.mkv' },
  { type: 'anime', hint: 'Like TV: Series/Season 01/… (absolute numbering works too)' },
  { type: 'books', hint: 'Author/Title/Title.epub — EPUB, PDF, MOBI, AZW3, CBZ…' },
  { type: 'audiobooks', hint: 'Author/Title/ with one .m4b or numbered .mp3 parts' },
]

export default function LibrariesAdmin() {
  const libs = useLibraries()
  const scanAll = useScanAll()
  const toast = useToast()
  const [params, setParams] = useSearchParams()
  const [editing, setEditing] = useState<Library | null>(null)
  const [deleting, setDeleting] = useState<Library | null>(null)
  const creating = params.get('new') === '1'
  const setCreating = (v: boolean) =>
    setParams(
      (p) => {
        const n = new URLSearchParams(p)
        if (v) n.set('new', '1')
        else n.delete('new')
        return n
      },
      { replace: true },
    )

  const anyScanning = libs.data?.some((l) => l.scan.state === 'scanning')

  return (
    <AdminPage
      title="Libraries"
      description="Folders on the server that Trove indexes. Trove only reads them — files are never moved, renamed or deleted."
      actions={
        <>
          {libs.data && libs.data.length > 0 && (
            <button
              type="button"
              className={buttonClass('outline')}
              disabled={scanAll.isPending}
              onClick={() =>
                scanAll.mutate(undefined, {
                  onSuccess: () => toast.success('Scanning all libraries'),
                  onError: (e) => toast.error('Couldn’t start the scan', errorMessage(e)),
                })
              }
            >
              <RefreshCw className={cx('size-4', (scanAll.isPending || anyScanning) && 'animate-spin [animation-duration:2s]')} />
              Scan all
            </button>
          )}
          <button type="button" className={buttonClass('primary')} onClick={() => setCreating(true)}>
            <FolderPlus className="size-4" /> Add library
          </button>
        </>
      }
    >
      {libs.isPending ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} className="h-32 rounded-2xl" />
          ))}
        </div>
      ) : libs.isError ? (
        <ErrorState error={libs.error} onRetry={() => void libs.refetch()} />
      ) : libs.data.length === 0 ? (
        <EmptyState
          icon={<LibraryBig className="size-7" />}
          title="No libraries yet"
          action={
            <button type="button" className={buttonClass('primary')} onClick={() => setCreating(true)}>
              <FolderPlus className="size-4" /> Add your first library
            </button>
          }
        >
          Add a folder of movies, TV, anime, books or audiobooks to get started.
        </EmptyState>
      ) : (
        <ul className="space-y-3">
          {libs.data.map((l) => (
            <LibraryRow key={l.id} lib={l} onEdit={() => setEditing(l)} onDelete={() => setDeleting(l)} />
          ))}
        </ul>
      )}

      <LibraryDialog open={creating} onClose={() => setCreating(false)} />
      <LibraryDialog open={!!editing} lib={editing ?? undefined} onClose={() => setEditing(null)} />
      <DeleteLibrary lib={deleting} onClose={() => setDeleting(null)} />
    </AdminPage>
  )
}

function LibraryRow({ lib, onEdit, onDelete }: { lib: Library; onEdit: () => void; onDelete: () => void }) {
  const scan = useScanLibrary()
  const toast = useToast()
  const Icon = LIBRARY_ICON[lib.type]
  const scanning = lib.scan.state === 'scanning'

  return (
    <li className={cx(cardClass, 'p-4 sm:p-5')}>
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
        <div className="flex min-w-0 flex-1 gap-4">
          <div className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-ink-800 text-ember-300">
            <Icon className="size-5" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <Link to={`/library/${lib.id}`} className="truncate text-base font-semibold text-ink-50 hover:text-ember-200">
                {lib.name}
              </Link>
              <span className="rounded-md bg-ink-800 px-2 py-0.5 text-xs text-ink-300">{LIBRARY_TYPE_LABEL[lib.type]}</span>
            </div>
            <p className="mt-1 truncate font-mono text-xs text-ink-400" title={lib.path ?? undefined}>
              {lib.path}
            </p>
            <dl className="mt-3 flex flex-wrap gap-x-6 gap-y-2 text-sm">
              <div>
                <dt className="sr-only">Titles</dt>
                <dd className="text-ink-200">{pluralize(lib.itemCount, 'title')}</dd>
              </div>
              <div>
                <dt className="sr-only">Size</dt>
                <dd className="text-ink-200">{formatBytes(lib.totalSize)}</dd>
              </div>
              <div>
                <dt className="sr-only">Last scan</dt>
                <dd className="text-ink-400" title={formatDateTime(lib.scan.finishedAt)}>
                  Last scan {relativeTime(lib.scan.finishedAt)}
                </dd>
              </div>
            </dl>
          </div>
        </div>
        <div className="flex shrink-0 gap-1.5 sm:justify-end">
          <button
            type="button"
            className={buttonClass('outline', 'sm')}
            disabled={scanning || scan.isPending}
            onClick={() =>
              scan.mutate(lib.id, { onError: (e) => toast.error(`Couldn’t scan ${lib.name}`, errorMessage(e)) })
            }
          >
            <RefreshCw className={cx('size-4', scanning && 'animate-spin [animation-duration:2s]')} />
            {scanning ? 'Scanning' : 'Scan'}
          </button>
          <button type="button" className={buttonClass('ghost', 'sm', 'w-9 px-0')} onClick={onEdit} aria-label={`Edit ${lib.name}`} title="Edit">
            <Pencil className="size-4" />
          </button>
          <button
            type="button"
            className={buttonClass('ghost', 'sm', 'w-9 px-0 hover:text-red-300')}
            onClick={onDelete}
            aria-label={`Remove ${lib.name}`}
            title="Remove"
          >
            <Trash className="size-4" />
          </button>
        </div>
      </div>
      {scanning && (
        <div className="mt-4 rounded-xl bg-ink-950/50 px-3.5 py-3" role="status">
          <p className="mb-2 text-xs text-ink-300">{scanLabel(lib.scan)}</p>
          <ScanProgress scan={lib.scan} />
        </div>
      )}
      {lib.scan.state === 'error' && (
        <p className="mt-4 rounded-xl border border-red-400/20 bg-red-500/[0.07] px-3.5 py-2.5 text-xs break-words text-red-200" role="alert">
          Last scan failed{lib.scan.error ? `: ${lib.scan.error}` : ''}
        </p>
      )}
    </li>
  )
}

function LibraryDialog({ open, lib, onClose }: { open: boolean; lib?: Library; onClose: () => void }) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={lib ? `Edit ${lib.name}` : 'Add a library'}
      description={lib ? 'Changing the type or folder triggers a rescan.' : 'Trove will scan the folder right away.'}
      size="lg"
    >
      {open && <LibraryForm key={lib?.id ?? 'new'} lib={lib} onDone={onClose} />}
    </Dialog>
  )
}

function LibraryForm({ lib, onDone }: { lib?: Library; onDone: () => void }) {
  const toast = useToast()
  const create = useCreateLibrary()
  const update = useUpdateLibrary()
  const [name, setName] = useState(lib?.name ?? '')
  const [type, setType] = useState<LibraryType | null>(lib?.type ?? null)
  const [path, setPath] = useState(lib?.path ?? '')
  const [picking, setPicking] = useState(!lib)
  const [touched, setTouched] = useState(false)
  const [nameEdited, setNameEdited] = useState(!!lib)
  const pending = create.isPending || update.isPending
  const err = create.error ?? update.error

  const pickType = (t: LibraryType) => {
    setType(t)
    if (!nameEdited) setName(LIBRARY_TYPE_LABEL[t])
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setTouched(true)
    if (!name.trim() || !type || !path.trim()) return
    const body = { name: name.trim(), type, path: path.trim() }
    if (lib) {
      const patch: Partial<typeof body> = {}
      if (body.name !== lib.name) patch.name = body.name
      if (body.type !== lib.type) patch.type = body.type
      if (body.path !== lib.path) patch.path = body.path
      if (!Object.keys(patch).length) return onDone()
      update.mutate(
        { id: lib.id, body: patch },
        {
          onSuccess: (l) => {
            toast.success(`Saved ${l.name}`, patch.type || patch.path ? 'Rescanning with the new settings.' : undefined)
            onDone()
          },
        },
      )
    } else {
      create.mutate(body, {
        onSuccess: (l) => {
          toast.success(`Added ${l.name}`, 'Scanning now — titles will appear as they’re found.')
          onDone()
        },
      })
    }
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-6">
      <fieldset>
        <legend className="mb-2 text-sm font-medium text-ink-200">What’s in this folder?</legend>
        <div className="grid gap-2 sm:grid-cols-2">
          {TYPES.map(({ type: t, hint }) => {
            const Icon = LIBRARY_ICON[t]
            const active = type === t
            return (
              <label
                key={t}
                className={cx(
                  'flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ember-400',
                  active ? 'border-ember-400/60 bg-ember-400/[0.07]' : 'border-white/[0.08] hover:border-white/20',
                )}
              >
                <input type="radio" name="type" value={t} checked={active} onChange={() => pickType(t)} className="sr-only" />
                <span
                  className={cx(
                    'flex size-9 shrink-0 items-center justify-center rounded-lg',
                    active ? 'bg-ember-gradient text-ink-950' : 'bg-ink-800 text-ink-300',
                  )}
                >
                  <Icon className="size-4.5" />
                </span>
                <span className="min-w-0">
                  <span className="block text-sm font-semibold text-ink-50">{LIBRARY_TYPE_LABEL[t]}</span>
                  <span className="mt-0.5 block font-mono text-[0.7rem] leading-snug break-words text-ink-400">{hint}</span>
                </span>
              </label>
            )
          })}
        </div>
        {touched && !type && <p className="mt-2 text-xs text-red-300">Choose a library type</p>}
      </fieldset>

      <TextField
        label="Name"
        value={name}
        onChange={(e) => {
          setName(e.target.value)
          setNameEdited(true)
        }}
        placeholder="e.g. Movies"
        error={touched && !name.trim() ? 'Give the library a name' : null}
      />

      <div>
        <label htmlFor="lib-path" className="mb-1.5 block text-sm font-medium text-ink-200">
          Folder on the server
        </label>
        <div className="flex gap-2">
          <input
            id="lib-path"
            value={path}
            onChange={(e) => setPath(e.target.value)}
            placeholder="/mnt/media/movies"
            spellCheck={false}
            autoCapitalize="none"
            className={cx(inputClass, 'font-mono', touched && !path.trim() && 'border-red-400/70')}
          />
          <button type="button" className={buttonClass('outline', 'md', 'shrink-0')} onClick={() => setPicking((p) => !p)} aria-expanded={picking}>
            <FolderOpen className="size-4" /> <span className="hidden sm:inline">Browse</span>
          </button>
        </div>
        {touched && !path.trim() && <p className="mt-1.5 text-xs text-red-300">Choose a folder</p>}
        {picking && (
          <div className="mt-3">
            <FolderPicker
              initial={path}
              onPick={(p) => {
                setPath(p)
                setPicking(false)
              }}
            />
          </div>
        )}
      </div>

      {err && (
        <p role="alert" className="rounded-xl border border-red-400/25 bg-red-500/10 px-3 py-2.5 text-sm text-red-200">
          {errorMessage(err)}
        </p>
      )}

      <div className="flex justify-end gap-2 border-t border-white/[0.06] pt-5">
        <button type="button" className={buttonClass('ghost')} onClick={onDone} disabled={pending}>
          Cancel
        </button>
        <button type="submit" className={buttonClass('primary')} disabled={pending}>
          {pending && <Spinner className="size-4" />}
          {lib ? 'Save changes' : 'Add & scan'}
        </button>
      </div>
    </form>
  )
}

function DeleteLibrary({ lib, onClose }: { lib: Library | null; onClose: () => void }) {
  const del = useDeleteLibrary()
  const toast = useToast()
  return (
    <ConfirmDialog
      open={!!lib}
      onClose={onClose}
      danger
      pending={del.isPending}
      title={lib ? `Remove “${lib.name}”?` : ''}
      confirmLabel="Remove library"
      icon={<Trash className="size-4" />}
      onConfirm={() =>
        lib &&
        del.mutate(lib.id, {
          onSuccess: () => {
            toast.success(`Removed ${lib.name}`, 'Files on disk were not touched.')
            onClose()
          },
          onError: (e) => toast.error('Couldn’t remove the library', errorMessage(e)),
        })
      }
    >
      <p>
        Trove will forget this library and its {lib ? pluralize(lib.itemCount, 'indexed title') : 'titles'}: they disappear from
        browsing and search for everyone.
      </p>
      <p className="mt-3 rounded-xl border border-emerald-400/20 bg-emerald-500/[0.07] px-3 py-2.5 text-emerald-100">
        <strong className="font-semibold">No files are deleted.</strong> Everything in{' '}
        <span className="font-mono text-xs break-all">{lib?.path}</span> stays exactly where it is on disk. You can add the
        folder again any time.
      </p>
    </ConfirmDialog>
  )
}
