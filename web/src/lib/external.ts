import type { ItemDetail } from '../api/types'

export interface ExternalLink {
  key: string
  label: string
  href: string
}

/** Build outbound links from `externalIds` (tmdb, imdb, tvdb, isbn, asin, openlibrary). */
export function externalLinks(item: ItemDetail): ExternalLink[] {
  const ids = item.externalIds
  const out: ExternalLink[] = []
  const enc = encodeURIComponent
  if (ids.tmdb) {
    const type = item.kind === 'show' ? 'tv' : 'movie'
    out.push({ key: 'tmdb', label: 'TMDB', href: `https://www.themoviedb.org/${type}/${enc(ids.tmdb)}` })
  }
  if (ids.imdb) out.push({ key: 'imdb', label: 'IMDb', href: `https://www.imdb.com/title/${enc(ids.imdb)}/` })
  if (ids.tvdb) out.push({ key: 'tvdb', label: 'TheTVDB', href: `https://thetvdb.com/dereferrer/series/${enc(ids.tvdb)}` })
  if (ids.openlibrary) {
    const id = ids.openlibrary.replace(/^\/?(works|books)\//, '')
    const path = /M$/i.test(id) ? 'books' : 'works'
    out.push({ key: 'openlibrary', label: 'Open Library', href: `https://openlibrary.org/${path}/${enc(id)}` })
  } else if (ids.isbn) {
    out.push({ key: 'isbn', label: 'Open Library', href: `https://openlibrary.org/isbn/${enc(ids.isbn)}` })
  }
  if (ids.asin) out.push({ key: 'asin', label: 'Audible', href: `https://www.audible.com/pd/${enc(ids.asin)}` })
  return out
}
