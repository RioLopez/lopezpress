import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'

// Dev proxy: forward /api/* and /health to the Go backend on :8080 so the
// Preact app can call the REST API same-origin during `npm run dev`.
// In production the built dist/ is served by Nginx which does the same.
export default defineConfig({
  plugins: [preact()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/health': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    // Small Preact bundle; disable chunking for this tiny app.
    rollupOptions: {
      output: {
        manualChunks: undefined,
      },
    },
  },
})
