import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'

// Dev proxy: forward /api/*, /health and /uploads to the Go backend on :8080
// so the Preact app can call the REST API same-origin during `npm run dev`.
// In production the built dist/ is served by Nginx which does the same.
//
// /uploads MUST be proxied: uploaded images live in the Go server's upload
// directory. Without this, Vite's SPA history fallback answers /uploads/*.png
// with index.html, so every uploaded image renders broken in dev.
export default defineConfig({
  plugins: [preact()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/health': { target: 'http://localhost:8080', changeOrigin: true },
      '/uploads': { target: 'http://localhost:8080', changeOrigin: true },
      '/feed': { target: 'http://localhost:8080', changeOrigin: true },
      '/sitemap.xml': { target: 'http://localhost:8080', changeOrigin: true },
      '/robots.txt': { target: 'http://localhost:8080', changeOrigin: true },
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
