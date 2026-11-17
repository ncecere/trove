import { useState } from 'react'
import type { Kind } from '../api/types'
import { KIND_ICON } from '../lib/icons'
import { placeholderBackground } from '../lib/art'
import { cx } from '../lib/ui'

interface PlaceholderProps {
  title: string
  kind: Kind
  subtitle?: string | null
  className?: string
  /** "card" sizes type for a ~150-200px card; "hero" for the detail page poster. */
  size?: 'card' | 'hero'
}

/** The no-artwork poster: title-derived gradient, typeset title and a kind glyph. */
export function ArtPlaceholder({ title, kind, subtitle, className, size = 'card' }: PlaceholderProps) {
  const Icon = KIND_ICON[kind]
  const long = title.length > 28
  return (
    <div
      className={cx('relative isolate flex h-full w-full flex-col overflow-hidden text-left', className)}
      style={{ background: placeholderBackground(title) }}
      aria-hidden="true"
    >
      {/* faint large glyph */}
      <Icon
        className={cx(
          'absolute -right-[12%] -bottom-[8%] -z-10 text-white/[0.07]',
          size === 'hero' ? 'size-[70%]' : 'size-[75%]',
        )}
        strokeWidth={1.25}
      />
      {/* fine grain / sheen */}
      <div className="absolute inset-0 -z-10 bg-[linear-gradient(180deg,rgb(255_255_255/0.06),transparent_35%,rgb(0_0_0/0.35))]" />
      <div className={cx('mt-auto', size === 'hero' ? 'p-5' : 'p-3')}>
        <div className={cx('mb-2 flex items-center gap-1.5 text-white/60', size === 'hero' && 'mb-3')}>
          <Icon className={size === 'hero' ? 'size-4' : 'size-3.5'} strokeWidth={2} />
          <span className={cx('font-semibold tracking-[0.16em] uppercase', size === 'hero' ? 'text-xs' : 'text-[0.6rem]')}>
            {kind === 'show' ? 'Series' : kind}
          </span>
        </div>
        <p
          className={cx(
            'font-bold tracking-[-0.02em] text-balance break-words hyphens-auto text-white drop-shadow-sm',
            size === 'hero'
              ? long
                ? 'line-clamp-6 text-lg leading-tight sm:text-xl lg:text-2xl'
                : 'line-clamp-4 text-2xl leading-[1.05] lg:text-3xl'
              : long
                ? 'line-clamp-4 text-[0.95rem] leading-tight'
                : 'line-clamp-4 text-lg leading-[1.1]',
          )}
        >
          {title}
        </p>
        {subtitle && (
          <p className={cx('mt-1.5 line-clamp-1 text-white/65', size === 'hero' ? 'text-sm' : 'text-[0.7rem]')}>
            {subtitle}
          </p>
        )}
      </div>
    </div>
  )
}

interface PosterImageProps {
  src: string | null
  title: string
  kind: Kind
  subtitle?: string | null
  className?: string
  size?: 'card' | 'hero'
  eager?: boolean
}

/** A poster/cover image that falls back to {@link ArtPlaceholder} when missing or broken. */
export function PosterImage({ src, title, kind, subtitle, className, size, eager }: PosterImageProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  const [loadedSrc, setLoadedSrc] = useState<string | null>(null)
  const show = src && failedSrc !== src
  return (
    <div className={cx('relative h-full w-full overflow-hidden bg-ink-800', className)}>
      {(!show || loadedSrc !== src) && (
        <ArtPlaceholder title={title} kind={kind} subtitle={subtitle} size={size} className="absolute inset-0" />
      )}
      {show && (
        <img
          src={src}
          alt=""
          loading={eager ? 'eager' : 'lazy'}
          decoding="async"
          draggable={false}
          onLoad={() => setLoadedSrc(src)}
          onError={() => setFailedSrc(src)}
          className={cx(
            'absolute inset-0 h-full w-full object-cover transition-opacity duration-300',
            loadedSrc === src ? 'opacity-100' : 'opacity-0',
          )}
        />
      )}
    </div>
  )
}
