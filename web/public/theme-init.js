// Set data-theme before first paint to avoid a light-theme flash on
// dark-mode reloads. Mirrors the resolve() logic in core/theme.tsx.
// Loaded as a classic script from <head> so it runs synchronously, before the
// SPA module boots and before the body paints. Kept external (not inline) so
// the document CSP can drop `unsafe-inline` from script-src.
(function () {
  var m = localStorage.getItem('echo.theme') || 'system'
  var t = m === 'system'
    ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
    : m
  document.documentElement.setAttribute('data-theme', t)
})()
