import { Download, Lock, Package } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useUser } from '../api/hooks'
import { formatBytes, LARGE_ZIP_BYTES } from '../lib/format'
import { buttonClass, cx, type ButtonSize, type ButtonVariant } from '../lib/ui'
import { ConfirmDialog } from './Dialog'

export const NO_DOWNLOAD_HINT = 'Your account is browse-only. Ask an admin for download access.'

function triggerDownload(href: string) {
  const a = document.createElement('a')
  a.href = href
  a.download = ''
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  a.remove()
}

interface Props {
  href: string
  children?: ReactNode
  /** Accessible label; required when the button is icon-only. */
  label: string
  size?: number | null
  variant?: ButtonVariant
  btnSize?: ButtonSize
  iconOnly?: boolean
  /** Zip downloads: confirm before starting when bigger than ~20 GB. */
  zip?: boolean
  className?: string
  icon?: ReactNode
}

/**
 * A download is a plain navigation to the API URL (`<a href download>`): the browser's
 * download manager handles multi-GB files and resume. Bytes never go through fetch().
 */
export function DownloadButton({
  href,
  children,
  label,
  size,
  variant = 'secondary',
  btnSize = 'md',
  iconOnly,
  zip,
  className,
  icon,
}: Props) {
  const user = useUser()
  const [confirm, setConfirm] = useState(false)
  const ico = icon ?? (zip ? <Package className="size-4" /> : <Download className="size-4" />)
  const cls = cx(buttonClass(variant, btnSize), iconOnly && (btnSize === 'sm' ? 'w-9 px-0' : 'w-10 px-0'), className)

  if (!user.canDownload) {
    return (
      <button type="button" className={cls} aria-disabled="true" title={NO_DOWNLOAD_HINT} aria-label={`${label} (unavailable: browse-only account)`}>
        {iconOnly ? <Lock className="size-4" /> : <>{<Lock className="size-4" />}{children}</>}
      </button>
    )
  }

  const large = zip && size !== null && size !== undefined && size > LARGE_ZIP_BYTES

  return (
    <>
      <a
        href={href}
        download
        className={cls}
        aria-label={iconOnly ? label : undefined}
        title={iconOnly ? label : undefined}
        onClick={(e) => {
          if (large) {
            e.preventDefault()
            setConfirm(true)
          }
        }}
      >
        {ico}
        {!iconOnly && children}
      </a>
      {large && (
        <ConfirmDialog
          open={confirm}
          onClose={() => setConfirm(false)}
          onConfirm={() => {
            setConfirm(false)
            triggerDownload(href)
          }}
          title="Start a large download?"
          confirmLabel={`Download ${formatBytes(size)}`}
          icon={<Download className="size-4" />}
        >
          <p>
            This zip is <strong className="text-ink-50">{formatBytes(size)}</strong>. It streams straight from the
            server into your browser’s downloads, so make sure you have enough free disk space. On slower connections
            this can take hours — downloading a single season or episode may be quicker.
          </p>
        </ConfirmDialog>
      )}
    </>
  )
}
