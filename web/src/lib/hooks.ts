import { useEffect, useRef, useState } from 'react'

export function useDebouncedValue<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = window.setTimeout(() => setV(value), ms)
    return () => window.clearTimeout(t)
  }, [value, ms])
  return v
}

/** Calls `onVisible` when the returned ref's element scrolls near the viewport. */
export function useInView<T extends Element>(onVisible: () => void, enabled: boolean, rootMargin = '800px') {
  const ref = useRef<T>(null)
  const cb = useRef(onVisible)
  useEffect(() => {
    cb.current = onVisible
  })
  useEffect(() => {
    const el = ref.current
    if (!el || !enabled) return
    const io = new IntersectionObserver((entries) => entries.some((e) => e.isIntersecting) && cb.current(), {
      rootMargin,
    })
    io.observe(el)
    return () => io.disconnect()
  }, [enabled, rootMargin])
  return ref
}

export function useDocumentTitle(title: string | null | undefined) {
  useEffect(() => {
    document.title = title ? `${title} · Trove` : 'Trove'
  }, [title])
}
