import { Search, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate, useSearchParams } from 'react-router'
import { cx } from '../lib/ui'

function isTypingTarget(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable
}

/** Global search: debounced, navigates to /search?q=, `/` focuses it from anywhere. */
export function SearchField({ className }: { className?: string }) {
  const loc = useLocation()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const onSearch = loc.pathname === '/search'
  const urlQ = onSearch ? (params.get('q') ?? '') : ''
  const [value, setValue] = useState(urlQ)
  const [lastUrlQ, setLastUrlQ] = useState(urlQ)
  const input = useRef<HTMLInputElement>(null)
  const pending = useRef<number | null>(null)

  // keep the field in sync when the URL changes (back/forward, leaving search)
  if (urlQ !== lastUrlQ) {
    setLastUrlQ(urlQ)
    if (value.trim() !== urlQ) setValue(urlQ)
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === '/' && !e.metaKey && !e.ctrlKey && !e.altKey && !isTypingTarget(e.target)) {
        if (document.querySelector('dialog[open]')) return
        e.preventDefault()
        input.current?.focus()
        input.current?.select()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // arriving on an empty search page (e.g. from the mobile Search tab): put the cursor in the field
  useEffect(() => {
    if (onSearch && !urlQ) input.current?.focus()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onSearch])

  useEffect(() => () => {
    if (pending.current) window.clearTimeout(pending.current)
  }, [])

  const go = (q: string, immediate = false) => {
    if (pending.current) window.clearTimeout(pending.current)
    const run = () => {
      const trimmed = q.trim()
      if (!trimmed && loc.pathname !== '/search') return
      navigate(`/search${trimmed ? `?q=${encodeURIComponent(trimmed)}` : ''}`, { replace: loc.pathname === '/search' })
    }
    if (immediate) run()
    else pending.current = window.setTimeout(run, 300)
  }

  return (
    <form
      role="search"
      className={cx('relative', className)}
      onSubmit={(e) => {
        e.preventDefault()
        go(value, true)
      }}
    >
      <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-400" />
      <input
        ref={input}
        type="search"
        value={value}
        onChange={(e) => {
          setValue(e.target.value)
          go(e.target.value)
        }}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            if (value) {
              setValue('')
              go('', true)
            } else input.current?.blur()
          }
        }}
        placeholder="Search movies, shows, books…"
        aria-label="Search the library"
        autoComplete="off"
        spellCheck={false}
        enterKeyHint="search"
        className="h-10 w-full rounded-xl border border-white/[0.07] bg-ink-850/80 pr-16 pl-9 text-sm text-ink-50 placeholder:text-ink-500 transition focus:border-ember-400/60 focus:bg-ink-850 focus:ring-2 focus:ring-ember-400/20 focus:outline-none"
      />
      {value ? (
        <button
          type="button"
          onClick={() => {
            setValue('')
            go('', true)
            input.current?.focus()
          }}
          className="absolute top-1/2 right-1.5 inline-flex size-7 -translate-y-1/2 items-center justify-center rounded-lg text-ink-400 hover:bg-ink-700 hover:text-ink-50"
          aria-label="Clear search"
        >
          <X className="size-4" />
        </button>
      ) : (
        <kbd className="pointer-events-none absolute top-1/2 right-2.5 hidden -translate-y-1/2 rounded-md border border-white/10 px-1.5 py-0.5 font-sans text-[0.7rem] text-ink-400 sm:block">
          /
        </kbd>
      )}
    </form>
  )
}
