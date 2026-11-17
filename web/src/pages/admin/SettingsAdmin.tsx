import { BookOpen, CircleCheck, Clock, Film, KeyRound, X } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { errorMessage } from '../../api/client'
import { useSettings, useUpdateSettings } from '../../api/hooks'
import type { Settings } from '../../api/types'
import { ConfirmDialog } from '../../components/Dialog'
import { Select, Toggle } from '../../components/Field'
import { ErrorState, Skeleton, Spinner } from '../../components/States'
import { useToast } from '../../lib/toast'
import { buttonClass, cardClass, cx, inputClass } from '../../lib/ui'
import { AdminPage } from './AdminHeader'

const INTERVALS = [
  { v: 0, label: 'Never (manual scans only)' },
  { v: 15, label: 'Every 15 minutes' },
  { v: 30, label: 'Every 30 minutes' },
  { v: 60, label: 'Every hour' },
  { v: 180, label: 'Every 3 hours' },
  { v: 360, label: 'Every 6 hours' },
  { v: 720, label: 'Every 12 hours' },
  { v: 1440, label: 'Once a day' },
]

function Panel({ icon, title, description, children }: { icon: ReactNode; title: string; description: string; children: ReactNode }) {
  return (
    <section className={cx(cardClass, 'p-5 sm:p-6')}>
      <div className="flex gap-4">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-ink-800 text-ember-300">{icon}</div>
        <div className="min-w-0 flex-1">
          <h2 className="text-base font-semibold text-ink-50">{title}</h2>
          <p className="mt-0.5 text-sm text-ink-400">{description}</p>
          <div className="mt-4">{children}</div>
        </div>
      </div>
    </section>
  )
}

export default function SettingsAdmin() {
  const settings = useSettings()
  return (
    <AdminPage title="Settings" description="Metadata sources and background scanning.">
      {settings.isPending ? (
        <div className="space-y-4">
          <Skeleton className="h-40 rounded-2xl" />
          <Skeleton className="h-28 rounded-2xl" />
          <Skeleton className="h-28 rounded-2xl" />
        </div>
      ) : settings.isError ? (
        <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
      ) : (
        <div className="max-w-3xl space-y-4">
          <TmdbPanel settings={settings.data} />
          <OpenLibraryPanel settings={settings.data} />
          <ScanPanel settings={settings.data} />
        </div>
      )}
    </AdminPage>
  )
}

function TmdbPanel({ settings }: { settings: Settings }) {
  const update = useUpdateSettings()
  const toast = useToast()
  const [key, setKey] = useState('')
  const [editing, setEditing] = useState(!settings.tmdbApiKeySet)
  const [confirmClear, setConfirmClear] = useState(false)
  const showForm = editing || !settings.tmdbApiKeySet

  const save = (e: FormEvent) => {
    e.preventDefault()
    if (!key.trim()) return
    update.mutate(
      { tmdbApiKey: key.trim() },
      {
        onSuccess: () => {
          toast.success('TMDB key saved', 'Rescan or refresh items to pull in posters and details.')
          setKey('')
          setEditing(false)
        },
        onError: (err) => toast.error('Couldn’t save the key', errorMessage(err)),
      },
    )
  }

  return (
    <Panel
      icon={<Film className="size-5" />}
      title="TMDB"
      description="Posters, backdrops & details for movies, TV and anime from The Movie Database."
    >
      {settings.tmdbApiKeySet && (
        <div className="mb-3 flex flex-wrap items-center gap-3">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-300 ring-1 ring-emerald-400/20">
            <CircleCheck className="size-3.5" /> API key configured
          </span>
          {!editing && (
            <button type="button" className={buttonClass('ghost', 'sm')} onClick={() => setEditing(true)}>
              <KeyRound className="size-4" /> Replace
            </button>
          )}
          <button
            type="button"
            className={buttonClass('ghost', 'sm', 'hover:text-red-300')}
            onClick={() => setConfirmClear(true)}
            disabled={update.isPending}
          >
            <X className="size-4" /> Clear
          </button>
        </div>
      )}
      {showForm && (
        <form onSubmit={save} className="flex flex-col gap-2 sm:flex-row">
          <label htmlFor="tmdb-key" className="sr-only">
            TMDB API key
          </label>
          <input
            id="tmdb-key"
            type="password"
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder={settings.tmdbApiKeySet ? 'Paste a new key to replace it' : 'Paste your TMDB API key (v3) or read access token'}
            autoComplete="off"
            spellCheck={false}
            className={cx(inputClass, 'font-mono')}
          />
          <div className="flex gap-2">
            <button type="submit" className={buttonClass('primary', 'md', 'flex-1 sm:flex-none')} disabled={!key.trim() || update.isPending}>
              {update.isPending && <Spinner className="size-4" />}
              Save key
            </button>
            {settings.tmdbApiKeySet && (
              <button type="button" className={buttonClass('ghost')} onClick={() => { setEditing(false); setKey('') }}>
                Cancel
              </button>
            )}
          </div>
        </form>
      )}
      <p className="mt-3 text-xs text-ink-500">
        The key is stored on the server and never shown again. Get one free at themoviedb.org › Settings › API.
      </p>
      <ConfirmDialog
        open={confirmClear}
        onClose={() => setConfirmClear(false)}
        danger
        pending={update.isPending}
        title="Clear the TMDB key?"
        confirmLabel="Clear key"
        onConfirm={() =>
          update.mutate(
            { tmdbApiKey: '' },
            {
              onSuccess: () => {
                toast.success('TMDB key cleared')
                setConfirmClear(false)
                setEditing(true)
              },
              onError: (err) => toast.error('Couldn’t clear the key', errorMessage(err)),
            },
          )
        }
      >
        Existing artwork and details stay, but new movies and shows won’t get posters or descriptions until a key is set
        again.
      </ConfirmDialog>
    </Panel>
  )
}

function OpenLibraryPanel({ settings }: { settings: Settings }) {
  const update = useUpdateSettings()
  const toast = useToast()
  return (
    <Panel
      icon={<BookOpen className="size-5" />}
      title="Open Library"
      description="Covers & details for books and audiobooks from openlibrary.org (no key needed)."
    >
      <Toggle
        checked={settings.openLibraryEnabled}
        disabled={update.isPending}
        onChange={(v) =>
          update.mutate(
            { openLibraryEnabled: v },
            {
              onSuccess: () => toast.success(v ? 'Open Library lookups on' : 'Open Library lookups off'),
              onError: (e) => toast.error('Couldn’t save', errorMessage(e)),
            },
          )
        }
        label="Look up books on Open Library"
        description="Sends book titles and authors to openlibrary.org during scans."
      />
    </Panel>
  )
}

function ScanPanel({ settings }: { settings: Settings }) {
  const update = useUpdateSettings()
  const toast = useToast()
  const known = INTERVALS.some((i) => i.v === settings.scanIntervalMinutes)
  return (
    <Panel
      icon={<Clock className="size-5" />}
      title="Automatic rescans"
      description="How often Trove re-walks every library to pick up new, changed or removed files."
    >
      <Select
        label="Scan interval"
        hideLabel
        value={String(settings.scanIntervalMinutes)}
        disabled={update.isPending}
        className="max-w-xs"
        onChange={(e) =>
          update.mutate(
            { scanIntervalMinutes: Number(e.target.value) },
            {
              onSuccess: () => toast.success('Scan interval saved'),
              onError: (err) => toast.error('Couldn’t save', errorMessage(err)),
            },
          )
        }
      >
        {!known && <option value={settings.scanIntervalMinutes}>Every {settings.scanIntervalMinutes} minutes</option>}
        {INTERVALS.map((i) => (
          <option key={i.v} value={i.v}>
            {i.label}
          </option>
        ))}
      </Select>
    </Panel>
  )
}
