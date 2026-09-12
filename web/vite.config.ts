import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// During dev, this proxies /api, /auth, /healthz, /readyz calls
// from the dev server (:5173) to the Go backend (:8080).
// In production, the Go binary serves the built dashboard directly.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api':     'http://localhost:8080',
      '/auth':    'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/readyz':  'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
