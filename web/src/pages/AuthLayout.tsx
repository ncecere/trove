import type { ReactNode } from 'react'
import type { Kind } from '../api/types'
import { ArtPlaceholder } from '../components/Artwork'
import { Logo } from '../components/Logo'

const WALL: [string, Kind][] = [
  ['The Long Quiet', 'movie'],
  ['Harbor Lights', 'show'],
  ['A Field Guide to Rain', 'book'],
  ['Northbound', 'movie'],
  ['Saltwater Kings', 'show'],
  ['Paper Moons', 'audiobook'],
  ['Echo Valley', 'movie'],
  ['The Cartographer', 'book'],
  ['Starfall Academy', 'show'],
  ['Velvet Hours', 'movie'],
  ['Iron & Ivy', 'book'],
  ['Lanterns', 'show'],
  ['Midnight Ferry', 'movie'],
  ['Glass Garden', 'audiobook'],
  ['Copper Sky', 'movie'],
  ['The Ninth Wave', 'show'],
  ['Small Gods of Autumn', 'book'],
  ['Neon Tide', 'movie'],
]

/** Full-bleed auth screen: a dimmed, tilted wall of generated posters behind a centred card. */
export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="relative isolate flex min-h-dvh items-center justify-center overflow-hidden px-4 py-10">
      <div className="absolute inset-0 -z-10 overflow-hidden" aria-hidden="true">
        <div className="absolute -inset-[20%] grid rotate-[-8deg] grid-cols-4 gap-4 opacity-40 sm:grid-cols-6">
          {[...WALL, ...WALL].map(([t, k], i) => (
            <div key={i} className="aspect-[2/3] overflow-hidden rounded-xl shadow-card">
              <ArtPlaceholder title={t} kind={k} />
            </div>
          ))}
        </div>
        <div className="absolute inset-0 bg-[radial-gradient(60%_60%_at_50%_45%,rgb(10_12_17/0.6),rgb(10_12_17/0.93))]" />
      </div>
      <div className="w-full max-w-sm animate-slide-up">
        <div className="mb-8 flex justify-center">
          <Logo className="scale-125" />
        </div>
        <div className="rounded-2xl border border-white/[0.08] bg-ink-900/85 p-6 shadow-lift backdrop-blur-xl sm:p-7">
          {children}
        </div>
      </div>
    </div>
  )
}
