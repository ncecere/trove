import '@fontsource-variable/inter'
import './index.css'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'

async function start() {
  // Dev-only in-browser API mock. `import.meta.env.DEV` is statically false in production
  // builds, so this branch — and the dynamic import of ./mock — is dropped from the bundle.
  if (import.meta.env.DEV && import.meta.env.VITE_MOCK === '1') {
    const { installMock } = await import('./mock/install')
    installMock()
  }
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}

void start()
