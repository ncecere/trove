import { useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, FolderSearch, Search, Tag, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useParams, useSearchParams } from 'react-router'
import { qk, useGenres, useItemsInfinite, useLibraries } from '../api/hooks'
import type { ItemSort, Library, SortOrder } from '../api/types'
import { Select } from '../components/Field'
import { PosterCard } from '../components/PosterCard'
import { ScanBanner } from '../components/ScanBanner'
import { CardSkeleton, EmptyState, ErrorState, Skeleton, Spinner } from '../components/States'
import { formatBytes, pluralize } from '../lib/format'
import { useDebouncedValue, useDocumentTitle, useInView } from '../lib/hooks'
import { LIBRARY_ICON } from '../lib/icons'
import { buttonClass, cx, inputClass } from '../lib/ui'

const SORTS: { value: ItemSort; label: string }[] = [
  { value: 'title', label: 'Title' },
  { value: 'added', label: 'Date added' },
  { value: 'year', label: 'Year' },
  { value: 'rating', label: 'Rating' },
  { value: 'size', label: 'Size' },
]

const defaultOrder = (s: ItemSort): SortOrder => (s === 'title' ? 'asc' : 'desc')

function parseSort(v: string | null): ItemSort {
  return SORTS.some((s) => s.value === v) ? (v as ItemSort) : 'title'
}

export const GRID_CLASS =
  'grid grid-cols-[repeat(auto-fill,minmax(8.25rem,1fr))] gap-x-3 gap-y-6 sm:grid-cols-[repeat(auto-fill,minmax(9.5rem,1fr))] sm:gap-x-4 xl:grid-cols-[repeat(auto-fill,minmax(10.5rem,1fr))] 3xl:grid-cols-[repeat(auto-fill,minmax(11.5rem,1fr))]'

export function LibraryPage() {
  const id = Number(useParams().id)
  const libs = useLibraries()
  const lib = libs.data?.find((l) => l.id === id)
  useDocumentTitle(lib?.name)

  if (libs.isPending) return <LibrarySkeleton />
  if (libs.isError) return <ErrorState error={libs.error} onRetry={() => void libs.refetch()} className="py-28" />
  if (!lib)
    return (
      <EmptyState className="py-28" icon={<FolderSearch className="size-7" />} title="Library not found">
        It may have been removed, or you don’t have access to it.
      </EmptyState>
    )
  return <LibraryView key={lib.id} lib={lib} />
}

function LibraryView({ lib }: { lib: Library }) {
  const [params, setParams] = useSearchParams()
  const sort = parseSort(params.get('sort'))
  const orderParam = params.get('order')
  const order: SortOrder = orderParam === 'asc' || orderParam === 'desc' ? orderParam : defaultOrder(sort)
  const genre = params.get('genre') ?? ''
  const urlQ = params.get('q') ?? ''
  const [qInput, setQInput] = useState(urlQ)
  const q = useDebouncedValue(qInput.trim(), 300)
  const qc = useQueryClient()

  const update = (patch: Record<string, string | null>) => {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        for (const [k, v] of Object.entries(patch)) {
          if (v === null || v === '') next.delete(k)
          else next.set(k, v)
        }
        return next
      },
      { replace: true },
    )
  }

  // push the debounced search term into the URL
  useEffect(() => {
    if (q !== (params.get('q') ?? '')) update({ q: q || null })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q])

  const query = { library: lib.id, sort, order, genre: genre || undefined, q: urlQ || undefined }
  const items = useItemsInfinite(query)
  const genres = useGenres(lib.id)

  // when a scan finishes, refresh what's on screen
  const wasScanning = useRef(lib.scan.state === 'scanning')
  useEffect(() => {
    const scanning = lib.scan.state === 'scanning'
    if (wasScanning.current && !scanning) {
      void qc.invalidateQueries({ queryKey: qk.itemsAll })
      void qc.invalidateQueries({ queryKey: qk.genres(lib.id) })
      void qc.invalidateQueries({ queryKey: qk.home })
    }
    wasScanning.current = scanning
  }, [lib.scan.state, lib.id, qc])

  const sentinel = useInView<HTMLDivElement>(
    () => {
      if (items.hasNextPage && !items.isFetchingNextPage) void items.fetchNextPage()
    },
    !!items.hasNextPage,
  )

  const Icon = LIBRARY_ICON[lib.type]
  const all = items.data?.pages.flatMap((p) => p.items) ?? []
  const total = items.data?.pages[0]?.total ?? 0
  const filtered = !!(genre || urlQ)
  const square = lib.type === 'audiobooks'

  return (
    <div className="px-4 pt-6 sm:px-6 sm:pt-8 lg:px-8">
      <header className="flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
        <div className="flex min-w-0 items-center gap-3.5">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-2xl border border-white/[0.06] bg-gradient-to-b from-ink-750 to-ink-850 text-ember-300 shadow-card">
            <Icon className="size-6" />
          </div>
          <div className="min-w-0">
            <h1 className="truncate text-2xl font-semibold tracking-tight text-ink-50 sm:text-3xl">{lib.name}</h1>
            <p className="mt-0.5 text-sm text-ink-400">
              {pluralize(lib.itemCount, 'title')} · {formatBytes(lib.totalSize)}
            </p>
          </div>
        </div>
      </header>

      <ScanBanner scan={lib.scan} className="mt-5" />

      {/* toolbar */}
      <div className="sticky top-[calc(4rem+env(safe-area-inset-top))] z-10 -mx-4 mt-5 border-b border-white/[0.04] bg-ink-950/85 px-4 py-3 backdrop-blur-xl sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative min-w-0 flex-[1_1_14rem]">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-400" />
            <input
              type="search"
              value={qInput}
              onChange={(e) => setQInput(e.target.value)}
              placeholder={`Filter ${lib.name}…`}
              aria-label={`Filter ${lib.name}`}
              className={cx(inputClass, 'pr-9 pl-9')}
            />
            {qInput && (
              <button
                type="button"
                onClick={() => setQInput('')}
                className="absolute top-1/2 right-1.5 inline-flex size-7 -translate-y-1/2 items-center justify-center rounded-lg text-ink-400 hover:bg-ink-700 hover:text-ink-50"
                aria-label="Clear filter"
              >
                <X className="size-4" />
              </button>
            )}
          </div>
          <Select
            label="Genre"
            hideLabel
            icon={<Tag className="size-4" />}
            value={genre}
            onChange={(e) => update({ genre: e.target.value || null })}
            className="min-w-0 flex-[1_1_9rem] sm:flex-[0_1_12rem]"
          >
            <option value="">All genres</option>
            {genre && !genres.data?.includes(genre) && <option value={genre}>{genre}</option>}
            {genres.data?.map((g) => (
              <option key={g} value={g}>
                {g}
              </option>
            ))}
          </Select>
          <div className="flex flex-[1_1_11rem] gap-2 sm:flex-[0_1_14rem]">
            <Select
              label="Sort by"
              hideLabel
              value={sort}
              onChange={(e) => {
                const s = parseSort(e.target.value)
                update({ sort: s === 'title' ? null : s, order: null })
              }}
              className="min-w-0 flex-1"
            >
              {SORTS.map((s) => (
                <option key={s.value} value={s.value}>
                  {s.label}
                </option>
              ))}
            </Select>
            <button
              type="button"
              className={buttonClass('outline', 'md', 'w-10 shrink-0 px-0')}
              onClick={() => {
                const next = order === 'asc' ? 'desc' : 'asc'
                update({ order: next === defaultOrder(sort) ? null : next })
              }}
              aria-label={order === 'asc' ? 'Ascending — switch to descending' : 'Descending — switch to ascending'}
              title={order === 'asc' ? 'Ascending' : 'Descending'}
            >
              {order === 'asc' ? <ArrowUp className="size-4" /> : <ArrowDown className="size-4" />}
            </button>
          </div>
        </div>
        {filtered && items.data && (
          <div className="mt-2.5 flex flex-wrap items-center gap-2 text-sm text-ink-400">
            <span>{pluralize(total, 'match', 'matches')}</span>
            <button
              type="button"
              onClick={() => {
                setQInput('')
                update({ q: null, genre: null })
              }}
              className="rounded-md px-1.5 py-0.5 text-ember-300 hover:bg-ink-800"
            >
              Clear filters
            </button>
          </div>
        )}
      </div>

      <div className="mt-6">
        {items.isPending ? (
          <div className={GRID_CLASS} aria-busy="true" aria-label="Loading titles">
            {Array.from({ length: 18 }, (_, i) => (
              <CardSkeleton key={i} square={square} />
            ))}
          </div>
        ) : items.isError ? (
          <ErrorState error={items.error} onRetry={() => void items.refetch()} />
        ) : all.length === 0 ? (
          filtered ? (
            <EmptyState icon={<Search className="size-7" />} title="No matches">
              Nothing in {lib.name} matches {urlQ ? <>“{urlQ}”</> : 'these filters'}
              {genre && urlQ ? ` in ${genre}` : genre ? ` (${genre})` : ''}.
            </EmptyState>
          ) : lib.scan.state === 'scanning' ? (
            <EmptyState icon={<Spinner className="size-7" />} title="Indexing your files">
              Titles will appear here as the scan finds them.
            </EmptyState>
          ) : (
            <EmptyState icon={<Icon className="size-7" />} title="This library is empty">
              No titles were found in this library’s folder. Check the folder layout, then rescan from Admin › Libraries.
            </EmptyState>
          )
        ) : (
          <>
            <div className={cx(GRID_CLASS, items.isPlaceholderData && 'opacity-60 transition-opacity')}>
              {all.map((item, i) => (
                <PosterCard key={item.id} item={item} eager={i < 12} />
              ))}
              {items.isFetchingNextPage &&
                Array.from({ length: 6 }, (_, i) => <CardSkeleton key={`s${i}`} square={square} />)}
            </div>
            <div ref={sentinel} className="h-px" aria-hidden="true" />
            {items.hasNextPage && !items.isFetchingNextPage && (
              <div className="mt-8 flex justify-center">
                <button type="button" className={buttonClass('outline')} onClick={() => void items.fetchNextPage()}>
                  Load more
                </button>
              </div>
            )}
            {!items.hasNextPage && all.length > 24 && (
              <p className="mt-10 text-center text-xs text-ink-500">
                That’s all {pluralize(total, 'title')}
              </p>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function LibrarySkeleton() {
  return (
    <div className="px-4 pt-6 sm:px-6 sm:pt-8 lg:px-8" aria-busy="true">
      <div className="flex items-center gap-3.5">
        <Skeleton className="size-12 rounded-2xl" />
        <div>
          <Skeleton className="h-7 w-48" />
          <Skeleton className="mt-2 h-4 w-32" />
        </div>
      </div>
      <Skeleton className="mt-8 h-10 w-full rounded-xl" />
      <div className={cx(GRID_CLASS, 'mt-6')}>
        {Array.from({ length: 18 }, (_, i) => (
          <CardSkeleton key={i} />
        ))}
      </div>
    </div>
  )
}
