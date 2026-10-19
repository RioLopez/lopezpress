import { useEffect } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { useTheme } from '../core/theme'
import { AppLink, useNav } from '../core/router'
import { useSite } from '../core/site'
import type { Locale } from '../core/i18n'
import { Footer } from './Footer'
import { UserMenu, LoginRegisterEntry } from './UserMenu'

// Content-area shell: top bar (brand + nav + utilities) + main + footer.
// Admin pages use the separate AdminLayout; this one is for public/content
// routes only (spec 06).
export function Layout({ children }: { children: ComponentChildren }) {
  const { user } = useAuth()
  const { t, locale, setLocale } = useI18n()
  const { cycle, mode } = useTheme()
  const [path, navigate] = useNav()
  const { config } = useSite()

  // Reflect the configured browser title across content pages.
  useEffect(() => {
    if (config.site_title) document.title = config.site_title
  }, [config.site_title])

  const isDark = mode === 'dark'
  const themeLabel = isDark ? 'D' : 'L'
  const langLabel = locale === 'zh' ? '中' : 'EN'
  const toggleLang = () => setLocale((locale === 'zh' ? 'en' : 'zh') as Locale)

  const isAdminUser = user?.role === 'admin' || user?.role === 'root'
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
        <nav class="topbar-nav">
          <button class={`nav-link ${isActive('/') ? 'nav-link--active' : ''}`} onClick={() => navigate('/')}>
            {t('nav.list')}
          </button>
          <button class={`nav-link ${isActive('/create') ? 'nav-link--active' : ''}`} onClick={() => navigate('/create')}>
            {t('nav.create')}
          </button>
          <button class={`nav-link ${isActive('/archive') ? 'nav-link--active' : ''}`} onClick={() => navigate('/archive')}>
            {t('nav.archive')}
          </button>
          {isAdminUser && (
            <button class="nav-link nav-link--admin" onClick={() => navigate('/admin')}>
              {t('nav.admin')}
            </button>
          )}
          {renderUtil()}
        </nav>
      </header>
      <main class={`main ${path === '/' ? 'main--wide' : ''} ${isEditorRoute ? 'main--editor' : ''}`}>{children}</main>
      {isEditorRoute ? null : <Footer />}
    </div>
  )
}
