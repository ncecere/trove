import { Search, SearchX } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { useItems } from '../api/hooks'
import type { ItemSummary, Kind } from '../api/types'
import { PosterCard } from '../components/PosterCard'
import { CardSkeleton, EmptyState, ErrorState, Skeleton } from '../components/States'
import { KIND_LABEL, pluralize } from '../lib/format'
import { useDocumentTitle } from '../lib/hooks'
import { KIND_ICON } from '../lib/icons'
import { cx } from '../lib/ui'
import { GRID_CLASS } from './LibraryPage'

const ORDER: Kind[] = ['movie', 'show', 'book', 'audiobook']
const PREVIEW = 12
const SEARCH_LIMIT = 100

export function SearchPage() {
  const [params] = useSearchParams()
  const q = (params.get('q') ?? '').trim()
  useDocumentTitle(q ? `“${q}”` : 'Search')
  const res = useItems({ q, limit: SEARCH_LIMIT, sort: 'title' }, q.length > 0)

  return (
    <div className="px-4 pt-6 sm:px-6 sm:pt-8 lg:px-8">
      {!q ? (
        <EmptyState className="py-20" icon={<Search className="size-7" />} title="Search everything">
          Find movies, shows, books and audiobooks by title, original title, author or series. Press{' '}
          <kbd className="rounded border border-white/15 px-1.5 py-0.5 text-xs text-ink-200">/</kbd> anywhere to jump
          here.
        </EmptyState>
      ) : res.isPending ? (
        <div aria-busy="true">
          <Skeleton className="h-8 w-64" />
          <Skeleton className="mt-8 h-6 w-32" />
          <div className={cx(GRID_CLASS, 'mt-4')}>
            {Array.from({ length: 12 }, (_, i) => (
              <CardSkeleton key={i} />
            ))}
          </div>
        </div>
      ) : res.isError ? (
        <ErrorState error={res.error} onRetry={() => void res.refetch()} />
      ) : res.data.items.length === 0 ? (
        <EmptyState className="py-20" icon={<SearchX className="size-7" />} title={`No results for “${q}”`}>
          Check the spelling, or try a shorter part of the title or the author’s name.
        </EmptyState>
      ) : (
        <Results q={q} items={res.data.items} total={res.data.total} stale={res.isPlaceholderData} />
      )}
    </div>
  )
}

function Results({ q, items, total, stale }: { q: string; items: ItemSummary[]; total: number; stale: boolean }) {
  const groups = ORDER.map((k) => ({ kind: k, items: items.filter((i) => i.kind === k) })).filter((g) => g.items.length)
  return (
    <div className={cx('transition-opacity', stale && 'opacity-60')}>
      <h1 className="text-2xl font-semibold tracking-tight text-ink-50 sm:text-3xl">
        Results for <span className="text-ember-gradient">“{q}”</span>
      </h1>
      <p className="mt-1 text-sm text-ink-400">
        {pluralize(total, 'title')}
        {total > items.length && ` — showing the first ${items.length}. Refine your search to narrow it down.`}
      </p>
      {groups.length > 1 && (
        <nav aria-label="Jump to" className="mt-4 flex flex-wrap gap-2">
          {groups.map((g) => {
            const Icon = KIND_ICON[g.kind]
            return (
              <a
                key={g.kind}
                href={`#results-${g.kind}`}
                className="inline-flex h-9 items-center gap-2 rounded-full border border-white/[0.08] bg-ink-850 px-3.5 text-sm text-ink-200 transition hover:border-ember-400/40 hover:text-ink-50"
              >
                <Icon className="size-4 text-ink-400" />
                {KIND_LABEL[g.kind].many}
                <span className="text-ink-500">{g.items.length}</span>
              </a>
            )
          })}
        </nav>
      )}
      <div className="mt-8 space-y-12">
        {groups.map((g) => (
          <Group key={g.kind} kind={g.kind} items={g.items} />
        ))}
      </div>
    </div>
  )
}

function Group({ kind, items }: { kind: Kind; items: ItemSummary[] }) {
  const [expanded, setExpanded] = useState(false)
  const Icon = KIND_ICON[kind]
  const shown = expanded ? items : items.slice(0, PREVIEW)
  return (
    <section id={`results-${kind}`} aria-label={KIND_LABEL[kind].many} className="scroll-mt-24">
      <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold text-ink-50">
        <Icon className="size-5 text-ember-300" />
        {KIND_LABEL[kind].many}
        <span className="text-sm font-normal text-ink-500">{items.length}</span>
      </h2>
      <div className={GRID_CLASS}>
        {shown.map((item) => (
          <PosterCard key={item.id} item={item} />
        ))}
      </div>
      {items.length > PREVIEW && (
        <button
          type="button"
          onClick={() => setExpanded((e) => !e)}
          className="mt-5 rounded-lg px-2 py-1.5 text-sm font-medium text-ember-300 hover:bg-ink-850"
        >
          {expanded ? 'Show fewer' : `Show all ${items.length} ${KIND_LABEL[kind].many.toLowerCase()}`}
        </button>
      )}
    </section>
  )
}
