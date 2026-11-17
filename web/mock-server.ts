// Dev-only Vite middleware used together with the in-browser mock (VITE_MOCK=1).
// The in-browser mock intercepts fetch(), but downloads and media streams are
// plain browser navigations / <video src>, which fetch interception can't see.
// This middleware answers those URLs with small placeholder payloads so the
// whole UI can be exercised without the Go backend. It never ships: it only
// exists in vite.config.ts's dev server.
import type { Plugin } from 'vite'

function toneWav(seconds: number): Buffer {
  const rate = 22050
  const n = rate * seconds
  const buf = Buffer.alloc(44 + n * 2)
  buf.write('RIFF', 0)
  buf.writeUInt32LE(36 + n * 2, 4)
  buf.write('WAVE', 8)
  buf.write('fmt ', 12)
  buf.writeUInt32LE(16, 16)
  buf.writeUInt16LE(1, 20)
  buf.writeUInt16LE(1, 22)
  buf.writeUInt32LE(rate, 24)
  buf.writeUInt32LE(rate * 2, 28)
  buf.writeUInt16LE(2, 32)
  buf.writeUInt16LE(16, 34)
  buf.write('data', 36)
  buf.writeUInt32LE(n * 2, 40)
  const notes = [261.63, 329.63, 392.0, 523.25]
  for (let i = 0; i < n; i++) {
    const t = i / rate
    const f = notes[Math.floor(t * 2) % notes.length]
    const env = Math.min(1, (t * 2) % 1 * 20) * Math.exp(-((t * 2) % 1) * 3)
    buf.writeInt16LE(Math.round(Math.sin(2 * Math.PI * f * t) * env * 6000), 44 + i * 2)
  }
  return buf
}

export function mockMediaServer(): Plugin {
  let wav: Buffer | null = null
  return {
    name: 'trove-mock-media',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = req.url ?? ''
        let m = url.match(/^\/api\/v1\/files\/(\d+)\/download/)
        if (m) {
          res.setHeader('Content-Type', 'text/plain')
          res.setHeader('Content-Disposition', `attachment; filename="trove-mock-file-${m[1]}.txt"`)
          res.end(`Trove mock download for file ${m[1]}.\nThe real server streams the file here.\n`)
          return
        }
        m = url.match(/^\/api\/v1\/items\/(\d+)\/download(?:\?season=(\d+))?/)
        if (m) {
          const name = `trove-mock-item-${m[1]}${m[2] ? `-season-${m[2]}` : ''}.txt`
          res.setHeader('Content-Type', 'text/plain')
          res.setHeader('Content-Disposition', `attachment; filename="${name}"`)
          res.end(`Trove mock zip for item ${m[1]}${m[2] ? ` season ${m[2]}` : ''}.\n`)
          return
        }
        m = url.match(/^\/api\/v1\/files\/(\d+)\/stream/)
        if (m) {
          wav ??= toneWav(16)
          res.setHeader('Content-Type', 'audio/wav')
          res.setHeader('Content-Length', String(wav.length))
          res.end(wav)
          return
        }
        next()
      })
    },
  }
}
