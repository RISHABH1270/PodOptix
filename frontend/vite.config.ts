import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),   // Tailwind as a Vite plugin — no separate config file needed
  ],
  server: {
    proxy: {
      // forward API calls to Go backend during development
      '/api':  'http://localhost:8080',
      '/auth': 'http://localhost:8080',
    },
  },
})
