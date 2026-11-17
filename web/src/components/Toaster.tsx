import { CircleCheck, CircleExclamationPoint, Info, X } from 'lucide-react'
import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react'
import { ToastContext, type ToastApi, type ToastInput } from '../lib/toast'
import { cx } from '../lib/ui'

interface ToastItem extends Required<Omit<ToastInput, 'description'>> {
  id: number
  description?: string
}

export function Toaster({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const nextId = useRef(1)

  const dismiss = useCallback((id: number) => setToasts((ts) => ts.filter((t) => t.id !== id)), [])

  const show = useCallback(
    (t: ToastInput) => {
      const id = nextId.current++
      const item: ToastItem = {
        id,
        title: t.title,
        description: t.description,
        tone: t.tone ?? 'info',
        durationMs: t.durationMs ?? (t.tone === 'error' ? 7000 : 4000),
      }
      setToasts((ts) => [...ts.slice(-3), item])
      window.setTimeout(() => dismiss(id), item.durationMs)
    },
    [dismiss],
  )

  const api = useMemo<ToastApi>(
    () => ({
      show,
      success: (title, description) => show({ title, description, tone: 'success' }),
      error: (title, description) => show({ title, description, tone: 'error' }),
      info: (title, description) => show({ title, description, tone: 'info' }),
    }),
    [show],
  )

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div
        className="pointer-events-none fixed inset-x-0 bottom-[calc(4.75rem+env(safe-area-inset-bottom))] z-[100] flex flex-col items-center gap-2 px-3 md:inset-x-auto md:right-5 md:bottom-5 md:items-end"
        aria-live="polite"
        role="region"
        aria-label="Notifications"
      >
        {toasts.map((t) => {
          const Icon = t.tone === 'success' ? CircleCheck : t.tone === 'error' ? CircleExclamationPoint : Info
          return (
            <div
              key={t.id}
              role={t.tone === 'error' ? 'alert' : 'status'}
              className="pointer-events-auto flex w-full max-w-sm animate-slide-up items-start gap-3 rounded-xl border border-white/[0.08] bg-ink-800/95 p-3.5 shadow-lift backdrop-blur"
            >
              <Icon
                className={cx(
                  'mt-0.5 size-5 shrink-0',
                  t.tone === 'success' && 'text-emerald-400',
                  t.tone === 'error' && 'text-red-400',
                  t.tone === 'info' && 'text-ember-300',
                )}
              />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-ink-50">{t.title}</p>
                {t.description && <p className="mt-0.5 text-sm break-words text-ink-400">{t.description}</p>}
              </div>
              <button
                type="button"
                onClick={() => dismiss(t.id)}
                className="-m-1 inline-flex size-7 shrink-0 items-center justify-center rounded-md text-ink-400 hover:bg-ink-700 hover:text-ink-50"
                aria-label="Dismiss notification"
              >
                <X className="size-4" />
              </button>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}
