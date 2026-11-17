// Generates stand-in artwork (SVG data URIs) for mock fixtures. Dev only.

function rng(seed: number) {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

function hash(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return h >>> 0
}

function ridge(r: () => number, w: number, h: number, base: number, amp: number): string {
  const pts: string[] = [`M0 ${h}`]
  const steps = 8
  for (let i = 0; i <= steps; i++) {
    const x = (w / steps) * i
    const y = base - r() * amp
    pts.push(`L${x.toFixed(0)} ${y.toFixed(0)}`)
  }
  pts.push(`L${w} ${h}Z`)
  return pts.join(' ')
}

function scene(key: string, w: number, h: number): string {
  const seed = hash(key)
  const r = rng(seed)
  r()
  const h1 = seed % 360
  const h2 = (h1 + 30 + Math.floor(r() * 120)) % 360
  const sunX = w * (0.25 + r() * 0.5)
  const sunY = h * (0.2 + r() * 0.25)
  const sunR = Math.min(w, h) * (0.12 + r() * 0.12)
  const layers = [0, 1, 2, 3]
    .map((i) => {
      const base = h * (0.55 + i * 0.12)
      const l = 30 - i * 7
      return `<path d="${ridge(r, w, h, base, h * 0.18)}" fill="hsl(${(h2 + i * 8) % 360} 45% ${l}%)"/>`
    })
    .join('')
  const stars = Array.from({ length: 18 }, () => {
    const x = (r() * w).toFixed(0)
    const y = (r() * h * 0.45).toFixed(0)
    return `<circle cx="${x}" cy="${y}" r="${(r() * 1.6 + 0.4).toFixed(1)}" fill="white" opacity="${(r() * 0.6 + 0.2).toFixed(2)}"/>`
  }).join('')
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}" preserveAspectRatio="xMidYMid slice"><defs><linearGradient id="b" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="hsl(${h1} 60% 22%)"/><stop offset="1" stop-color="hsl(${h2} 55% 45%)"/></linearGradient><radialGradient id="s"><stop offset="0" stop-color="hsl(${(h1 + 40) % 360} 95% 85%)"/><stop offset="1" stop-color="hsl(${(h1 + 20) % 360} 90% 60%)"/></radialGradient></defs><rect width="${w}" height="${h}" fill="url(#b)"/>${stars}<circle cx="${sunX.toFixed(0)}" cy="${sunY.toFixed(0)}" r="${sunR.toFixed(0)}" fill="url(#s)" opacity=".9"/>${layers}</svg>`
}

const uri = (svg: string) => `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`

export const posterArt = (key: string) => uri(scene(key, 400, 600))
export const squareArt = (key: string) => uri(scene(key, 500, 500))
export const backdropArt = (key: string) => uri(scene(`${key}-bd`, 1280, 720))
