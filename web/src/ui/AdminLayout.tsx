import { useState, useEffect } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { useTheme } from '../core/theme'
import { useNav } from '../core/router'
import { useSite } from '../core/site'
import type { Locale } from '../core/i18n'
import { isRoot, isAdminOnly } from '../core/roles'
import { listAdminMenus } from '../core/api'
import { FALLBACK_MENU, menuLabel, groupLabel } from '../core/menu'
import type { MenuItem } from '../core/types'
import { Footer } from './Footer'

// The sidebar order/visibility is editable (B5), so the rows come from
// /api/admin/menus. FALLBACK_MENU is the previous hardcoded array: it paints
// the first frame and keeps the console navigable if that request fails.
export function AdminLayout({ children }: { children: ComponentChildren }) {
  const { user, logout } = useAuth()
  const { t, locale, setLocale } = useI18n()
  const { cycle, mode } = useTheme()
  const [path, navigate] = useNav()
  const { config } = useSite()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [menu, setMenu] = useState<MenuItem[]>(FALLBACK_MENU)

  // The management page dispatches this event after a save so the sidebar
  // reflects a reorder/rename without a full reload.
  useEffect(() => {
    const load = () => listAdminMenus().then(setMenu).catch(() => {})
    load()
    window.addEventListener('menus-updated', load)
    return () => window.removeEventListener('menus-updated', load)
  }, [])

  const isDark = mode === 'dark'
  const themeLabel = isDark ? 'D' : 'L'
  const langLabel = locale === 'zh' ? '中' : 'EN'
  const toggleLang = () => setLocale((locale === 'zh' ? 'en' : 'zh') as Locale)

  const isActive = (to: string) =>
    to === '/admin' ? path === '/admin' : path === to || path.startsWith(to + '/')

  const go = (to: string) => {
    navigate(to)
    setDrawerOpen(false)
  }

  const isRootUser = isRoot(user?.role)

  // Group nav items by their group key, preserving row order. Root-only items
  // (the ban vault and the menu manager itself) stay hidden from plain admins.
  const visibleNav = menu.filter((n) => n.visible && (n.root_only ? isRootUser : true))
  const groups: { key: string; label: string; items: MenuItem[] }[] = []
  for (const item of visibleNav) {
    let g = groups.find((x) => x.key === item.group_key)
    if (!g) {
      g = { key: item.group_key, label: groupLabel(item.group_key, t), items: [] }
      groups.push(g)
    }
    g.items.push(item)
  }

  const accountName = user?.display_name || user?.username || ''

  return (
    <div class={`admin-layout ${drawerOpen ? 'admin-layout--open' : ''}`}>
      <aside class="admin-sidebar">
        <div class="admin-sidebar-brand">
          {config.site_logo ? (
            <img class="brand-logo" src={config.site_logo} alt={config.site_name} />
          ) : (
            <span class="brand-dot" />
          )}
          <span class="brand-name">{config.site_name || 'Echo'}</span>
        </div>

        <nav class="admin-sidebar-nav">
          {groups.map((g) => (
            <div class="admin-nav-group" key={g.key || 'root'}>
              {g.label && <div class="admin-nav-group-title">{g.label}</div>}
              {g.items.map((item) => (
                <button
                  key={item.key || item.id}
                  class={`admin-nav-link ${isActive(item.path) ? 'admin-nav-link--active' : ''}`}
                  onClick={() => go(item.path)}
                >
                  {menuLabel(item, t)}
                </button>
              ))}
            </div>
          ))}
        </nav>

        <div class="admin-sidebar-account">
          <div class="admin-account-id">
            {user?.avatar ? (
              <img class="admin-account-avatar" src={user.avatar} alt={accountName} />
            ) : (
              <span class="admin-account-avatar admin-account-avatar--default">
                {accountName.slice(0, 1).toUpperCase()}
              </span>
            )}
            <div class="admin-account-meta">
              <span class="admin-account-name">{accountName}</span>
              {isAdminOnly(user?.role) && <span class="badge-admin">admin</span>}
              {isRoot(user?.role) && <span class="badge-root">root</span>}
            </div>
          </div>
          <div class="admin-account-actions">
            <button class="btn btn-ghost btn-sm" onClick={() => go('/')}>
              {t('nav.backToContent')}
            </button>
            <button class="btn btn-ghost btn-sm" onClick={logout}>
              {t('auth.logout')}
            </button>
          </div>
        </div>
      </aside>

      <div class="admin-content">
        <header class="admin-topbar">
          <button
            class="btn btn-ghost btn-sm admin-hamburger"
            onClick={() => setDrawerOpen((v) => !v)}
            title="Menu"
          >
            ☰
          </button>
          <div class="admin-topbar-crumb">{currentCrumb(path, t, isRootUser, menu)}</div>
          <div class="admin-topbar-util">
            <button class="btn btn-ghost btn-sm icon-btn" title="Theme" onClick={cycle}>
              {themeLabel}
            </button>
            <button class="btn btn-ghost btn-sm icon-btn" title="Language" onClick={toggleLang}>
              {langLabel}
            </button>
          </div>
        </header>
        <main class="admin-main">
          {children}
          <Footer />
        </main>
      </div>
    </div>
  )
}

function currentCrumb(
  path: string,
  t: (k: string) => string,
  isRootUser: boolean,
  menu: MenuItem[],
): string {
  const match = menu.find((n) => {
    if (!n.visible || (n.root_only && !isRootUser)) return false
    return n.path === '/admin' ? path === '/admin' : path === n.path || path.startsWith(n.path + '/')
  })
  if (match) return menuLabel(match, t)
  return t('nav.dashboard')
}