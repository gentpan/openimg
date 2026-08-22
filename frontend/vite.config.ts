import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    rollupOptions: {
      output: {
        // The inline SVG icon data (src/icons.ts) is needed by the first
        // paint (Nav, Home), so pin it into the entry chunk. Left alone,
        // rolldown splits it into a shared chunk that also drags the i18n
        // dictionaries with it, and the first paint pays for two round
        // trips instead of one slightly larger entry.
        manualChunks(id) {
          if (id.includes('src/icons.ts') || id.includes('src/Icon.tsx')) {
            return 'index'
          }
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/auth': 'http://127.0.0.1:8080',
      // Only /admin/api — bare /admin is a client-side route, and proxying it
      // would hand the admin page itself to the backend.
      '/admin/api': 'http://127.0.0.1:8080',
      '/storage': 'http://127.0.0.1:8080',
      '/healthz': 'http://127.0.0.1:8080',
    },
  },
})
