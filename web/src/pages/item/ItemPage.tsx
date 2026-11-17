import { useState } from 'react'
import { useParams } from 'react-router'
import { useItem } from '../../api/hooks'
import type { ItemDetail, MediaFile } from '../../api/types'
import { PlayerDialog } from '../../components/Player'
import { ErrorState, Skeleton } from '../../components/States'
import { useDocumentTitle } from '../../lib/hooks'
import { AudiobookDetail, BookDetail } from './BookDetail'
import { MovieDetail } from './MovieDetail'
import { ShowDetail } from './ShowDetail'

export function ItemPage() {
  const id = Number(useParams().id)
  const item = useItem(Number.isFinite(id) ? id : 0)
  useDocumentTitle(item.data?.title)

  if (!Number.isFinite(id) || id <= 0) return <ErrorState error={new Error('Invalid link')} title="Not found" className="py-28" />
  if (item.isPending) return <DetailSkeleton />
  if (item.isError) return <ErrorState error={item.error} onRetry={() => void item.refetch()} className="py-28" />
  return <ItemView key={item.data.id} item={item.data} />
}

function ItemView({ item }: { item: ItemDetail }) {
  const [playing, setPlaying] = useState<MediaFile | null>(null)
  const props = { item, onPlay: setPlaying }

  return (
    <div className="pb-8">
      {item.kind === 'movie' && <MovieDetail {...props} />}
      {item.kind === 'show' && <ShowDetail {...props} />}
      {item.kind === 'book' && <BookDetail {...props} />}
      {item.kind === 'audiobook' && <AudiobookDetail {...props} />}

      <PlayerDialog file={playing} title={item.title} onClose={() => setPlaying(null)} />
    </div>
  )
}

function DetailSkeleton() {
  return (
    <div className="px-4 pt-6 sm:px-6 sm:pt-10 lg:px-8 lg:pt-16 xl:pt-24" aria-busy="true" aria-label="Loading">
      <div className="flex flex-col items-center gap-6 md:flex-row md:items-start md:gap-8 lg:gap-12">
        <Skeleton className="aspect-[2/3] w-40 shrink-0 rounded-2xl sm:w-48 md:w-56 lg:w-64 xl:w-72" />
        <div className="w-full flex-1 md:pt-6">
          <Skeleton className="mx-auto h-6 w-28 rounded-full md:mx-0" />
          <Skeleton className="mx-auto mt-4 h-11 w-3/4 md:mx-0" />
          <Skeleton className="mx-auto mt-3 h-4 w-1/3 md:mx-0" />
          <div className="mt-5 flex justify-center gap-2 md:justify-start">
            <Skeleton className="h-7 w-16 rounded-full" />
            <Skeleton className="h-7 w-20 rounded-full" />
            <Skeleton className="h-7 w-14 rounded-full" />
          </div>
          <Skeleton className="mt-7 h-12 w-full rounded-xl sm:w-52" />
          <Skeleton className="mt-7 h-4 w-full max-w-3xl" />
          <Skeleton className="mt-2 h-4 w-full max-w-3xl" />
          <Skeleton className="mt-2 h-4 w-2/3 max-w-2xl" />
        </div>
      </div>
    </div>
  )
}
