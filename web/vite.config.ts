import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// During dev, this proxies API calls from the Vite dev server to the Go backend.
// Ports are env-driven so UI tests can start Vite + backend on isolated ports.
//   Normal dev:   Vite :5173 → Go :8080
//   UI tests:     Vite :5174 → Go :9091  (isolated DB + Redis index)
// In production, the Go binary serves the built dashboard directly (single-origin).
const backend = `http://localhost:${process.env.VITE_API_PORT || '8080'}`

export default defineConfig({
  plugins: [react()],
  server: {
    port: parseInt(process.env.VITE_PORT || '5173'),
    proxy: {
      '/api':     backend,
      '/auth':    backend,
      '/healthz': backend,
      '/readyz':  backend,
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
