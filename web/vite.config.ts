// The asset sync below uses Node APIs, which the app's own compilation target
// (DOM only) does not include. Referenced here rather than added to tsconfig's
// `types`, so src/ still cannot reach for globals that do not exist in the
// browser.
/// <reference types="node" />
import { defineConfig, type Plugin } from 'vite'
import preact from '@preact/preset-vite'
import { cp, mkdir, rm, readdir, stat } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// --- Vditor runtime assets -------------------------------------------------
//
// Vditor loads its Markdown engine, icons, locales and highlighter at runtime
// from `${cdn}/dist/...`, and defaults that cdn to unpkg. For a self-hosted
// blog that is two problems at once: a third-party request on every editor
// load, and a hard failure whenever unpkg is slow or unreachable from wherever
// the author happens to be. So the assets are served from our own origin and
// `cdn` points at /vditor in VditorEditor.tsx.
//
// Only the reachable slice is copied. node_modules/vditor/dist is 24 MB, of
// which 18 MB is renderers no post on this site asks for (mermaid, echarts,
// graphviz, mathjax, katex, markmap, abcjs, smiles-drawer, flowchart,
// wavedrom, plantuml). Copying the whole dist would also mean committing
// 24 MB of generated third-party code, so the sync runs on every dev start and
// build instead, and public/vditor stays out of version control.
//
// The layout below public/ mirrors dist exactly, because the runtime path is
// literal concatenation: `${cdn}/dist/js/lute/lute.min.js`.

const webRoot = path.dirname(fileURLToPath(import.meta.url))
const vditorSrc = path.join(webRoot, 'node_modules', 'vditor', 'dist')
const vditorDest = path.join(webRoot, 'public', 'vditor', 'dist')

const VDITOR_FILES = [
  // The Markdown engine. 3.7 MB raw and not optional: every keystroke is
  // parsed by it, in every edit mode.
  'js/lute/lute.min.js',
  'js/icons/ant.js',
  'js/icons/material.js',
  // Only the two locales the site ships; the other ten are dead weight.
  'js/i18n/zh_CN.js',
  'js/i18n/en_US.js',
  // Syntax highlighting for fenced code blocks.
  'js/highlight.js/highlight.min.js',
  // Vditor appends ".min.css" to the configured theme name.
  'js/highlight.js/styles/github.min.css',
  'js/highlight.js/styles/github-dark.min.css',
]

const VDITOR_DIRS = [
  // ant-design / light / dark / wechat, picked via preview.theme.
  'css/content-theme',
  // Emoji sprite behind the toolbar's emoji picker.
  'images',
]

async function dirSize(dir: string): Promise<number> {
  let total = 0
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    total += entry.isDirectory() ? await dirSize(full) : (await stat(full)).size
  }
  return total
}

async function syncVditorAssets(): Promise<void> {
  if (!existsSync(vditorSrc)) {
    // Failing loudly beats an editor that renders blank with a 404 in the
    // console: the package is a dependency, so this can only mean that
    // npm install never ran.
    throw new Error(
      `[vditor] ${vditorSrc} missing - run "npm install" in web/ before starting Vite`,
    )
  }

  // Wiped first so a downgrade or a narrower list cannot leave orphaned files
  // behind. Failure is tolerated on purpose: a running dev server holds open
  // handles on these files, and some sandboxes redirect recursive deletes
  // through the recycle bin, which refuses directories. The copies below
  // overwrite everything they write, so a failed wipe costs disk space at
  // worst, never correctness.
  try {
    await rm(vditorDest, { recursive: true, force: true })
  } catch {
    /* see above */
  }

  for (const rel of VDITOR_FILES) {
    const from = path.join(vditorSrc, rel)
    const to = path.join(vditorDest, rel)
    if (!existsSync(from)) throw new Error(`[vditor] expected asset missing: ${rel}`)
    await mkdir(path.dirname(to), { recursive: true })
    await cp(from, to)
  }

  for (const rel of VDITOR_DIRS) {
    const from = path.join(vditorSrc, rel)
    if (!existsSync(from)) throw new Error(`[vditor] expected directory missing: ${rel}`)
    await cp(from, path.join(vditorDest, rel), { recursive: true })
  }

  const size = (await dirSize(vditorDest)) / 1024 / 1024
  console.log(`[vditor] assets -> public/vditor (${size.toFixed(2)} MB)`)
}

function vditorAssets(): Plugin {
  return {
    name: 'vditor-local-assets',
    // configResolved runs for `vite` and `vite build` alike, which also covers
    // the launcher: LopezPress.bat calls vite.js directly and therefore never
    // runs npm's "predev" hook.
    async configResolved() {
      await syncVditorAssets()
    },
  }
}

// Dev proxy: forward /api/* and /uploads to the Go backend on :8080 so the
// Preact app can call the REST API same-origin during `npm run dev`. In
// production Nginx does the same. /feed, /sitemap.xml and /robots.txt are
// proxied too, otherwise the SPA history fallback would answer them with
// index.html instead of the server-rendered XML.
//
// /uploads MUST be proxied: uploaded images live in the Go server's upload
// directory. Without this, Vite's SPA history fallback answers /uploads/*.png
// with index.html, so every uploaded image renders broken in dev.
export default defineConfig({
  plugins: [preact(), vditorAssets()],
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
