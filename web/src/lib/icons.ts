import { BookOpen, Film, Headphones, Sparkles, Tv, type LucideIcon } from 'lucide-react'
import type { Kind, LibraryType } from '../api/types'

export const LIBRARY_ICON: Record<LibraryType, LucideIcon> = {
  movies: Film,
  tv: Tv,
  anime: Sparkles,
  books: BookOpen,
  audiobooks: Headphones,
}

export const KIND_ICON: Record<Kind, LucideIcon> = {
  movie: Film,
  show: Tv,
  book: BookOpen,
  audiobook: Headphones,
}
