import { Clock, FolderPlus, LibraryBig } from 'lucide-react'
import { Link } from 'react-router'
import { useHome, useUser } from '../api/hooks'
import { PosterCard } from '../components/PosterCard'
import { Slider, SliderItem } from '../components/Slider'
import { CardSkeleton, EmptyState, ErrorState, Skeleton } from '../components/States'
import { formatBytes, isSquareArt, pluralize } from '../lib/format'
import { LIBRARY_ICON } from '../lib/icons'
import { buttonClass } from '../lib/ui'

function RowSkeleton() {
  return (
    <div aria-hidden="true">
      <div className="mb-3 px-4 sm:px-6 lg:px-8">
        <Skeleton className="h-6 w-44" />
      </div>
      <div className="flex gap-3 overflow-hidden px-4 sm:gap-4 sm:px-6 lg:px-8">
        {Array.from({ length: 9 }, (_, i) => (
          <div key={i} className="w-[8.25rem] shrink-0 sm:w-36 md:w-40 xl:w-44">
            <CardSkeleton />
          </div>
        ))}
      </div>
    </div>
  )
}

export function HomePage() {
  const user = useUser()
  const home = useHome()

  if (home.isPending) {
    return (
      <div className="space-y-10 pt-6" aria-busy="true" aria-label="Loading home">
        <RowSkeleton />
        <RowSkeleton />
        <RowSkeleton />
      </div>
    )
  }
  if (home.isError) return <ErrorState error={home.error} onRetry={() => void home.refetch()} className="py-28" />

  const { recentlyAdded, libraries } = home.data

  if (libraries.length === 0) {
    return user.role === 'admin' ? (
      <EmptyState
        className="py-24"
        icon={<FolderPlus className="size-7" />}
        title="Add your first library"
        action={
          <Link to="/admin/libraries?new=1" className={buttonClass('primary', 'lg')}>
            <FolderPlus className="size-4" /> Add a library
          </Link>
        }
      >
        Point Trove at a folder of movies, TV shows, anime, books or audiobooks on your NAS. It will index everything
        and build a browsable library — your files are never moved or changed.
      </EmptyState>
    ) : (
      <EmptyState className="py-24" icon={<LibraryBig className="size-7" />} title="Nothing to browse yet">
        You don’t have access to any libraries yet. Ask the admin of this Trove to share one with you.
      </EmptyState>
    )
  }

  const totalItems = libraries.reduce((n, l) => n + l.library.itemCount, 0)

  return (
    <div className="space-y-10 pt-6 sm:space-y-12 sm:pt-8">
      <header className="px-4 sm:px-6 lg:px-8">
        <h1 className="text-2xl font-semibold tracking-tight text-ink-50 sm:text-3xl">
          {greeting()}, <span className="text-ember-gradient">{user.username}</span>
        </h1>
        <p className="mt-1 text-sm text-ink-400">
          {pluralize(totalItems, 'title')} across {pluralize(libraries.length, 'library', 'libraries')}
        </p>
      </header>

      {recentlyAdded.length > 0 && (
        <Slider
          label="Recently added"
          title="Recently Added"
          icon={<Clock className="size-5 text-ember-300" />}
        >
          {recentlyAdded.map((item, i) => (
            <SliderItem key={item.id}>
              <PosterCard item={item} uniform eager={i < 6} />
            </SliderItem>
          ))}
        </Slider>
      )}

      {libraries.map(({ library, recent }) => {
        const Icon = LIBRARY_ICON[library.type]
        const square = recent.length > 0 && recent.every((r) => isSquareArt(r.kind))
        return (
          <Slider
            key={library.id}
            label={library.name}
            title={library.name}
            icon={<Icon className="size-5 text-ink-400" />}
            subtitle={`${pluralize(library.itemCount, 'title')} · ${formatBytes(library.totalSize)}`}
            seeAllTo={`/library/${library.id}`}
          >
            {recent.length === 0 ? (
              <p className="py-6 text-sm text-ink-500">
                {library.scan.state === 'scanning' ? 'Scanning… titles will appear here shortly.' : 'This library is empty.'}
              </p>
            ) : (
              recent.map((item) => (
                <SliderItem key={item.id}>
                  <PosterCard item={item} uniform={!square} />
                </SliderItem>
              ))
            )}
          </Slider>
        )
      })}
    </div>
  )
}

function greeting(): string {
  const h = new Date().getHours()
  if (h < 5) return 'Good evening'
  if (h < 12) return 'Good morning'
  if (h < 18) return 'Good afternoon'
  return 'Good evening'
}
