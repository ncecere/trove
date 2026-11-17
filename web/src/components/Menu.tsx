import { useEffect, useId, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react'
import { cx } from '../lib/ui'

export interface MenuItem {
  label: string
  icon?: ReactNode
  onSelect: () => void
  danger?: boolean
}

interface Props {
  trigger: (props: { open: boolean; toggle: () => void; id: string; menuId: string }) => ReactNode
  header?: ReactNode
  items: MenuItem[]
  align?: 'left' | 'right'
  placement?: 'bottom' | 'top'
  className?: string
}

/** Small accessible popover menu: Esc / outside click close, arrow keys move between items. */
export function Menu({ trigger, header, items, align = 'right', placement = 'bottom', className }: Props) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const panel = useRef<HTMLDivElement>(null)
  const id = useId()
  const menuId = `${id}-menu`

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false)
        root.current?.querySelector<HTMLElement>(`[aria-controls="${menuId}"]`)?.focus()
      }
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    panel.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open, menuId])

  const onPanelKey = (e: ReactKeyboardEvent) => {
    const els = Array.from(panel.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])
    const i = els.indexOf(document.activeElement as HTMLElement)
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      els[(i + 1) % els.length]?.focus()
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      els[(i - 1 + els.length) % els.length]?.focus()
    } else if (e.key === 'Tab') {
      setOpen(false)
    }
  }

  return (
    <div ref={root} className={cx('relative', className)}>
      {trigger({ open, toggle: () => setOpen((o) => !o), id, menuId })}
      {open && (
        <div
          ref={panel}
          id={menuId}
          role="menu"
          aria-labelledby={id}
          onKeyDown={onPanelKey}
          className={cx(
            'absolute z-50 min-w-56 animate-pop-in overflow-hidden rounded-xl border border-white/[0.08] bg-ink-800 p-1.5 shadow-lift',
            align === 'right' ? 'right-0' : 'left-0',
            placement === 'bottom' ? 'top-full mt-2' : 'bottom-full mb-2',
          )}
        >
          {header && <div className="mb-1 border-b border-white/[0.06] px-3 pt-2 pb-3">{header}</div>}
          {items.map((it) => (
            <button
              key={it.label}
              type="button"
              role="menuitem"
              onClick={() => {
                setOpen(false)
                it.onSelect()
              }}
              className={cx(
                'flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm transition focus:outline-none',
                it.danger
                  ? 'text-red-300 hover:bg-red-500/10 focus:bg-red-500/10'
                  : 'text-ink-100 hover:bg-ink-700 focus:bg-ink-700',
              )}
            >
              {it.icon && <span className="text-ink-400">{it.icon}</span>}
              {it.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
