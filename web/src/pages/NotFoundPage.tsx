import { Compass } from 'lucide-react'
import { Link } from 'react-router'
import { EmptyState } from '../components/States'
import { buttonClass } from '../lib/ui'

export function NotFoundPage() {
  return (
    <EmptyState
      className="py-28"
      icon={<Compass className="size-7" />}
      title="Nothing here"
      action={
        <Link to="/" className={buttonClass('primary')}>
          Back to home
        </Link>
      }
    >
      This page doesn’t exist. It may have moved, or the link is out of date.
    </EmptyState>
  )
}
