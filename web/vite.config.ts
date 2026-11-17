import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { mockMediaServer } from './mock-server.ts'

export default defineConfig(({ mode, command }) => {
  const env = { ...loadEnv(mode, process.cwd(), 'VITE_'), ...process.env }
  const mock = command === 'serve' && env.VITE_MOCK === '1'
  return {
    plugins: [react(), tailwindcss(), ...(mock ? [mockMediaServer()] : [])],
    build: {
      outDir: 'dist',
      emptyOutDir: true,
      sourcemap: false,
      chunkSizeWarningLimit: 700,
    },
    server: {
      port: 5173,
      proxy: mock
        ? undefined
        : {
            '/api': { target: 'http://localhost:8080', changeOrigin: false },
          },
    },
  }
})
