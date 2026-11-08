import { useEffect, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { useTheme } from '../core/theme'
import { AppLink, useNav } from '../core/router'
import { useSite } from '../core/site'
import type { Locale } from '../core/i18n'
import { isAdmin } from '../core/roles'
import { Footer } from './Footer'
import { UserMenu, LoginRegisterEntry } from './UserMenu'

// Content-area shell: top bar (brand + nav + utilities) + main + footer.
// Admin pages use the separate AdminLayout; this one is for public/content
// routes only.
export function Layout({ children }: { children: ComponentChildren }) {
  const { user } = useAuth()
  const { t, locale, setLocale } = useI18n()
  const { cycle, mode } = useTheme()
  const [path, navigate] = useNav()
  const { config } = useSite()
  // ≤639: the content nav moves into a dropdown behind a menu button. A
  // horizontally scrolling nav cut "管理" in half and had no affordance.
  const [navOpen, setNavOpen] = useState(false)

  // Reflect the configured browser title across content pages.
  useEffect(() => {
    if (config.site_title) document.title = config.site_title
  }, [config.site_title])

  const isDark = mode === 'dark'
  const themeLabel = isDark ? 'D' : 'L'
  const langLabel = locale === 'zh' ? '中' : 'EN'
  const toggleLang = () => setLocale((locale === 'zh' ? 'en' : 'zh') as Locale)

  useEffect(() => setNavOpen(false), [path])

  // A dropdown without an outside-click close would stay open over the page.
  useEffect(() => {
    if (!navOpen) return
    const onDoc = (e: MouseEvent) => {
      const el = e.target as Element | null
      if (!el?.closest('.topbar')) setNavOpen(false)
    }
    document.addEventListener('click', onDoc)
    return () => document.removeEventListener('click', onDoc)
  }, [navOpen])

  const go = (p: string) => {
    setNavOpen(false)
    navigate(p)
  }

  const isAdminUser = isAdmin(user?.role)
  const isEditorRoute = path === '/create' || path.startsWith('/edit/')
  const isActive = (p: string) =>
    p === '/' ? path === '/' || path === '' : path === p

  const renderUtil = () => (
    <>
      <button class="btn btn-ghost btn-sm icon-btn" title="Theme" onClick={cycle}>
        {themeLabel}
      </button>
      <button class="btn btn-ghost btn-sm icon-btn" title="Language" onClick={toggleLang}>
        {langLabel}
      </button>
      {user ? <UserMenu /> : <LoginRegisterEntry />}
    </>
  )

  return (
    <div class={`layout ${isEditorRoute ? 'layout--editor' : ''}`}>
      <header class="topbar">
        <div class="topbar-brand">
          <AppLink to="/" class="topbar-brand">
            {config.site_logo ? (
              <img class="brand-logo" src={config.site_logo} alt={config.site_name} />
            ) : (
              <span class="brand-dot" />
            )}
            <span class="brand-name">{config.site_name || 'Echo Content'}</span>
          </AppLink>
        </div>
        <button
          class="btn btn-ghost btn-sm icon-btn topbar-menu-btn"
          aria-label={t('nav.menu')}
          aria-expanded={navOpen}
          aria-controls="topbar-nav"
          onClick={() => setNavOpen((o) => !o)}
        >
          ☰
        </button>
        <nav id="topbar-nav" class={`topbar-nav ${navOpen ? 'is-open' : ''}`}>
          <button class={`nav-link ${isActive('/') ? 'nav-link--active' : ''}`} onClick={() => go('/')}>
            {t('nav.list')}
          </button>
          <button class={`nav-link ${isActive('/create') ? 'nav-link--active' : ''}`} onClick={() => go('/create')}>
            {t('nav.create')}
          </button>
          <button class={`nav-link ${isActive('/archive') ? 'nav-link--active' : ''}`} onClick={() => go('/archive')}>
            {t('nav.archive')}
          </button>
          {isAdminUser && (
            <button class="nav-link nav-link--admin" onClick={() => go('/admin')}>
              {t('nav.admin')}
            </button>
          )}
        </nav>
        <div class="topbar-util">{renderUtil()}</div>
      </header>
      <main class={`main ${path === '/' ? 'main--wide' : ''} ${isEditorRoute ? 'main--editor' : ''}`}>{children}</main>
      {isEditorRoute ? null : <Footer />}
    </div>
  )
}
