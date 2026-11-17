import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { cx } from '../lib/ui'

export const ROW_CARD_WIDTH = 'w-[8.25rem] sm:w-36 md:w-40 xl:w-44 3xl:w-48'

interface Props {
  title: ReactNode
  icon?: ReactNode
  seeAllTo?: string
  subtitle?: ReactNode
  children: ReactNode
  /** aria label for the region */
  label: string
}

export function Slider({ title, icon, subtitle, seeAllTo, children, label }: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)

  const update = useCallback(() => {
    const el = ref.current
    if (!el) return
    setCanLeft(el.scrollLeft > 4)
    setCanRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 4)
  }, [])

  useEffect(() => {
    const el = ref.current
    if (!el) return
    update()
    const ro = new ResizeObserver(update)
    ro.observe(el)
    el.addEventListener('scroll', update, { passive: true })
    return () => {
      ro.disconnect()
      el.removeEventListener('scroll', update)
    }
  }, [update])

  const scrollBy = (dir: 1 | -1) => {
    const el = ref.current
    if (!el) return
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    el.scrollBy({ left: dir * el.clientWidth * 0.85, behavior: reduce ? 'auto' : 'smooth' })
  }

  const arrow =
    'hidden md:inline-flex size-9 items-center justify-center rounded-full border border-white/10 bg-ink-800/80 text-ink-100 transition hover:bg-ink-700 hover:text-white disabled:opacity-30 disabled:pointer-events-none'

  return (
    <section aria-label={label} className="min-w-0">
      <div className="mb-3 flex items-end justify-between gap-4 px-4 sm:px-6 lg:px-8">
        <div className="min-w-0">
          <h2 className="flex items-center gap-2 text-lg font-semibold tracking-tight text-ink-50 sm:text-xl">
            {icon}
            <span className="truncate">{title}</span>
          </h2>
          {subtitle && <p className="mt-0.5 text-sm text-ink-400">{subtitle}</p>}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {seeAllTo && (
            <Link
              to={seeAllTo}
              className="rounded-lg px-2 py-1.5 text-sm font-medium text-ink-300 transition hover:text-ember-300"
            >
              See all
            </Link>
          )}
          <button type="button" className={arrow} onClick={() => scrollBy(-1)} disabled={!canLeft} aria-label="Scroll left">
            <ChevronLeft className="size-5" />
          </button>
          <button type="button" className={arrow} onClick={() => scrollBy(1)} disabled={!canRight} aria-label="Scroll right">
            <ChevronRight className="size-5" />
          </button>
        </div>
      </div>
      <div className="relative">
        <div
          ref={ref}
          className={cx(
            'scrollbar-none flex snap-x snap-mandatory scroll-px-4 gap-3 overflow-x-auto overscroll-x-contain px-4 pt-1 pb-3',
            'sm:scroll-px-6 sm:gap-4 sm:px-6 lg:scroll-px-8 lg:px-8',
          )}
        >
          {children}
        </div>
      </div>
    </section>
  )
}

export function SliderItem({ children }: { children: ReactNode }) {
  return <div className={cx('shrink-0 snap-start', ROW_CARD_WIDTH)}>{children}</div>
}
