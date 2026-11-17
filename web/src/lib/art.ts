import { hashString } from './format'

/** Deterministic palette for a title: two hues a little apart, rich but dark. */
export function titleHues(title: string): { h1: number; h2: number; seed: number } {
  const seed = hashString(title.toLowerCase())
  const h1 = seed % 360
  const h2 = (h1 + 28 + ((seed >>> 9) % 50)) % 360
  return { h1, h2, seed }
}

export function placeholderBackground(title: string): string {
  const { h1, h2, seed } = titleHues(title)
  const angle = 140 + ((seed >>> 3) % 60)
  const gx = 20 + ((seed >>> 5) % 60)
  return [
    `radial-gradient(120% 70% at ${gx}% 0%, hsl(${h1} 70% 62% / 0.35), transparent 60%)`,
    `radial-gradient(90% 60% at 100% 100%, hsl(${h2} 70% 40% / 0.45), transparent 70%)`,
    `linear-gradient(${angle}deg, hsl(${h1} 42% 24%), hsl(${h2} 48% 11%))`,
  ].join(', ')
}
