import { X } from 'lucide-react'
import { useEffect, useId, useRef, type ReactNode } from 'react'
import { buttonClass, cx } from '../lib/ui'
import { Spinner } from './States'

interface DialogProps {
  open: boolean
  onClose: () => void
  title: ReactNode
  description?: ReactNode
  children?: ReactNode
  footer?: ReactNode
  size?: 'sm' | 'md' | 'lg' | 'xl'
  /** Disable closing via Esc/backdrop (e.g. while saving). */
  locked?: boolean
  className?: string
  bodyClassName?: string
}

const SIZES = { sm: 'max-w-md', md: 'max-w-lg', lg: 'max-w-2xl', xl: 'max-w-4xl' }

/**
 * Modal built on the native <dialog> element: showModal() makes the rest of the page inert
 * (focus is trapped inside), Esc closes, focus returns to the opener on close.
 */
export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = 'md',
  locked,
  className,
  bodyClassName,
}: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null)
  const titleId = useId()
  const descId = useId()
  const lockedRef = useRef(locked)
  const closeRef = useRef(onClose)
  useEffect(() => {
    lockedRef.current = locked
    closeRef.current = onClose
  })

  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (open && !el.open) {
      const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
      el.showModal()
      document.documentElement.style.overflow = 'hidden'
      return () => {
        document.documentElement.style.overflow = ''
        if (el.open) el.close()
        opener?.focus()
      }
    }
  }, [open])

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const onCancel = (e: Event) => {
      e.preventDefault()
      if (!lockedRef.current) closeRef.current()
    }
    el.addEventListener('cancel', onCancel)
    return () => el.removeEventListener('cancel', onCancel)
  }, [])

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      aria-describedby={description ? descId : undefined}
      onMouseDown={(e) => {
        // click on the backdrop (the dialog element itself, outside the panel)
        if (e.target === e.currentTarget && !locked) onClose()
      }}
      className={cx(
        'm-auto max-h-[min(90dvh,56rem)] w-[calc(100vw-1.5rem)] overflow-visible bg-transparent p-0 text-ink-100 backdrop:animate-fade-in',
        SIZES[size],
      )}
    >
      {open && (
        <div
          className={cx(
            'flex max-h-[min(90dvh,56rem)] animate-pop-in flex-col overflow-hidden rounded-2xl border border-white/[0.08] bg-ink-850 shadow-[0_30px_80px_-20px_rgb(0_0_0/0.8)]',
            className,
          )}
        >
          <header className="flex items-start gap-4 border-b border-white/[0.06] px-5 pt-5 pb-4 sm:px-6">
            <div className="min-w-0 flex-1">
              <h2 id={titleId} className="text-lg font-semibold tracking-tight text-ink-50">
                {title}
              </h2>
              {description && (
                <div id={descId} className="mt-1 text-sm text-ink-400">
                  {description}
                </div>
              )}
            </div>
            <button
              type="button"
              onClick={onClose}
              disabled={locked}
              className="-mt-1 -mr-2 inline-flex size-9 shrink-0 items-center justify-center rounded-lg text-ink-400 transition hover:bg-ink-700 hover:text-ink-50"
              aria-label="Close dialog"
            >
              <X className="size-5" />
            </button>
          </header>
          {children !== undefined && (
            <div className={cx('scrollbar-thin min-h-0 flex-1 overflow-y-auto px-5 py-5 sm:px-6', bodyClassName)}>
              {children}
            </div>
          )}
          {footer && (
            <footer className="flex flex-wrap items-center justify-end gap-2 border-t border-white/[0.06] bg-ink-900/60 px-5 py-4 sm:px-6">
              {footer}
            </footer>
          )}
        </div>
      )}
    </dialog>
  )
}

interface ConfirmProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: ReactNode
  children?: ReactNode
  confirmLabel?: string
  danger?: boolean
  pending?: boolean
  icon?: ReactNode
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  children,
  confirmLabel = 'Confirm',
  danger,
  pending,
  icon,
}: ConfirmProps) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      size="sm"
      locked={pending}
      footer={
        <>
          <button type="button" className={buttonClass('ghost')} onClick={onClose} disabled={pending}>
            Cancel
          </button>
          <button
            type="button"
            className={buttonClass(danger ? 'danger' : 'primary')}
            onClick={onConfirm}
            disabled={pending}
            autoFocus
          >
            {pending ? <Spinner className="size-4" /> : icon}
            {confirmLabel}
          </button>
        </>
      }
    >
      <div className="text-sm leading-relaxed text-ink-300">{children}</div>
    </Dialog>
  )
}
