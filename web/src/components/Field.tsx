import { Check, ChevronDown, Eye, EyeOff } from 'lucide-react'
import { useId, useState, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from 'react'
import { cx, inputClass } from '../lib/ui'

interface FieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> {
  label: ReactNode
  hint?: ReactNode
  error?: string | null
}

export function TextField({ label, hint, error, className, type, ...rest }: FieldProps) {
  const id = useId()
  const [reveal, setReveal] = useState(false)
  const isPassword = type === 'password'
  return (
    <div className={className}>
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-ink-200">
        {label}
      </label>
      <div className="relative">
        <input
          id={id}
          type={isPassword && reveal ? 'text' : type}
          aria-invalid={error ? true : undefined}
          aria-describedby={error || hint ? `${id}-d` : undefined}
          className={cx(
            inputClass,
            isPassword && 'pr-11',
            error && 'border-red-400/70 focus:border-red-400 focus:ring-red-400/25',
          )}
          {...rest}
        />
        {isPassword && (
          <button
            type="button"
            onClick={() => setReveal((r) => !r)}
            className="absolute top-1/2 right-1 inline-flex size-8 -translate-y-1/2 items-center justify-center rounded-lg text-ink-400 hover:text-ink-100"
            aria-label={reveal ? 'Hide password' : 'Show password'}
            aria-pressed={reveal}
          >
            {reveal ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
          </button>
        )}
      </div>
      {(error || hint) && (
        <p id={`${id}-d`} className={cx('mt-1.5 text-xs', error ? 'text-red-300' : 'text-ink-400')}>
          {error || hint}
        </p>
      )}
    </div>
  )
}

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  /** Visually hide the label but keep it for screen readers. */
  hideLabel?: boolean
  icon?: ReactNode
}

export function Select({ label, hideLabel, icon, className, children, ...rest }: SelectProps) {
  const id = useId()
  return (
    <div className={className}>
      {label && (
        <label htmlFor={id} className={hideLabel ? 'sr-only' : 'mb-1.5 block text-sm font-medium text-ink-200'}>
          {label}
        </label>
      )}
      <div className="relative">
        {icon && (
          <span className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-ink-400">{icon}</span>
        )}
        <select
          id={id}
          className={cx(inputClass, 'cursor-pointer appearance-none pr-9', icon ? 'pl-9' : undefined)}
          {...rest}
        >
          {children}
        </select>
        <ChevronDown className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-ink-400" />
      </div>
    </div>
  )
}

interface ToggleProps {
  checked: boolean
  onChange: (v: boolean) => void
  label: ReactNode
  description?: ReactNode
  disabled?: boolean
}

export function Toggle({ checked, onChange, label, description, disabled }: ToggleProps) {
  const id = useId()
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="min-w-0">
        <label htmlFor={id} className="text-sm font-medium text-ink-100">
          {label}
        </label>
        {description && <p className="mt-0.5 text-xs leading-relaxed text-ink-400">{description}</p>}
      </div>
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={cx(
          'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition disabled:opacity-50',
          checked ? 'bg-ember-500' : 'bg-ink-600',
        )}
      >
        <span
          className={cx(
            'inline-block size-5 rounded-full bg-white shadow transition-transform',
            checked ? 'translate-x-[1.375rem]' : 'translate-x-0.5',
          )}
        />
      </button>
    </div>
  )
}

interface CheckboxProps {
  checked: boolean
  onChange: (v: boolean) => void
  label: ReactNode
  disabled?: boolean
  icon?: ReactNode
}

export function Checkbox({ checked, onChange, label, disabled, icon }: CheckboxProps) {
  return (
    <label
      className={cx(
        'flex min-h-10 cursor-pointer items-center gap-3 rounded-lg px-2 py-1.5 text-sm transition hover:bg-ink-800',
        disabled && 'pointer-events-none opacity-50',
      )}
    >
      <input
        type="checkbox"
        className="peer sr-only"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span
        className={cx(
          'flex size-5 shrink-0 items-center justify-center rounded-md border transition peer-focus-visible:ring-2 peer-focus-visible:ring-ember-400',
          checked ? 'border-ember-400 bg-ember-500 text-ink-950' : 'border-ink-500 bg-ink-900',
        )}
        aria-hidden="true"
      >
        {checked && <Check className="size-3.5" strokeWidth={3} />}
      </span>
      {icon && <span className="text-ink-400">{icon}</span>}
      <span className="min-w-0 truncate text-ink-100">{label}</span>
    </label>
  )
}
