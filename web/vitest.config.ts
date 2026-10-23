import { defineConfig } from 'vitest/config'

// AR-8: pure-function tests only — node environment, no DOM, no preact.
// Keeping a dedicated vitest config (instead of reusing vite.config.ts) means
// the preact plugin and dev proxy are not loaded for tests, and the test
// environment is explicit rather than implicit.
export default defineConfig({
  test: {
    environment: 'node',
  },
})
