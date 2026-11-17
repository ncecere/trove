export function cx(...parts: (string | false | null | undefined)[]): string {
  return parts.filter(Boolean).join(' ')
}

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline'
export type ButtonSize = 'sm' | 'md' | 'lg'

const base =
  'inline-flex items-center justify-center gap-2 rounded-xl font-medium whitespace-nowrap select-none transition ' +
  'duration-150 disabled:opacity-50 disabled:pointer-events-none aria-disabled:opacity-50 aria-disabled:cursor-not-allowed ' +
  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ember-400'

const variants: Record<ButtonVariant, string> = {
  primary:
    'bg-ember-gradient text-ink-950 shadow-[0_6px_20px_-6px_rgb(245_154_54/0.55)] hover:brightness-110 active:brightness-95',
  secondary: 'bg-ink-700/80 text-ink-50 hover:bg-ink-600 active:bg-ink-700 backdrop-blur',
  outline: 'border border-ink-600 text-ink-100 hover:bg-ink-800 hover:border-ink-500',
  ghost: 'text-ink-200 hover:bg-ink-800 hover:text-ink-50',
  danger: 'bg-red-500/90 text-white hover:bg-red-500 active:bg-red-600',
}

const sizes: Record<ButtonSize, string> = {
  sm: 'h-9 px-3 text-sm',
  md: 'h-10 px-4 text-sm',
  lg: 'h-12 px-5 text-base',
}

export function buttonClass(variant: ButtonVariant = 'secondary', size: ButtonSize = 'md', extra?: string): string {
  return cx(base, variants[variant], sizes[size], extra)
}

export const inputClass =
  'h-10 w-full rounded-xl border border-ink-600/70 bg-ink-900 px-3 text-sm text-ink-50 placeholder:text-ink-500 ' +
  'transition focus:border-ember-400/70 focus:outline-none focus:ring-2 focus:ring-ember-400/25 disabled:opacity-60'

export const cardClass = 'rounded-2xl border border-white/[0.06] bg-ink-900/80 shadow-card'
