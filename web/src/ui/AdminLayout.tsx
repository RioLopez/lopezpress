import { useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { useTheme } from '../core/theme'
import { useNav } from '../core/router'
import { useSite } from '../core/site'
import type { Locale } from '../core/i18n'
import { isRoot, isAdminOnly } from '../core/roles'
import { Footer } from './Footer'

interface NavItem {
  to: string
  label: string
  group?: string
  // Only shown to the super-admin (root). Omitted = every admin-or-root.
  rootOnly?: boolean
}

// Left vertical sidebar nav (spec 06, regrouped in spec 26). Eleven flat items
// were hard to scan; they now follow the Halo-style split of CONTENT / USERS /
// SYSTEM. The group titles come from i18n so the structure survives translation.
const NAV: NavItem[] = [
  { to: '/admin', label: 'nav.dashboard' },
  { to: '/admin/posts', label: 'admin.postsTitle', group: 'nav.groupContent' },
  { to: '/admin/review', label: 'admin.reviewTitle', group: 'nav.groupContent' },
  { to: '/admin/categories', label: 'admin.categoriesTitle', group: 'nav.groupContent' },
  { to: '/admin/tags', label: 'admin.tagsTitle', group: 'nav.groupContent' },
  { to: '/admin/trash', label: 'admin.trashTitle', group: 'nav.groupContent' },
  { to: '/admin/banned', label: 'admin.banVaultTitle', group: 'nav.groupContent', rootOnly: true },
  { to: '/admin/users', label: 'admin.usersTitle', group: 'nav.groupUsers' },
  { to: '/admin/site', label: 'site.title', group: 'nav.groupSystem' },
  { to: '/admin/backup', label: 'admin.backupTitle', group: 'nav.groupSystem' },
  { to: '/admin/audit', label: 'audit.title', group: 'nav.groupSystem' },
]

export function AdminLayout({ children }: { children: ComponentChildren }) {
  const { user, logout } = useAuth()
  const { t, locale, setLocale } = useI18n()
  const { cycle, mode } = useTheme()
  const [path, navigate] = useNav()
  const { config } = useSite()
  const [drawerOpen, setDrawerOpen] = useState(false)

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

  // Group nav items by their `group` key, preserving order. Root-only items
  // (the ban vault) are hidden from plain admins.
  const visibleNav = NAV.filter((n) => (n.rootOnly ? isRootUser : true))
  const groups: { key: string; label: string; items: NavItem[] }[] = []
  for (const item of visibleNav) {
    const key = item.group || ''
    let g = groups.find((x) => x.key === key)
    if (!g) {
      g = { key, label: key ? t(key) : '', items: [] }
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
                  key={item.to}
                  class={`admin-nav-link ${isActive(item.to) ? 'admin-nav-link--active' : ''}`}
                  onClick={() => go(item.to)}
                >
                  {t(item.label)}
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
          <div class="admin-topbar-crumb">{currentCrumb(path, t, isRootUser)}</div>
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

function currentCrumb(path: string, t: (k: string) => string, isRoot: boolean): string {
  const match = NAV.find((n) => {
    if (n.rootOnly && !isRoot) return false
    return n.to === '/admin' ? path === '/admin' : path === n.to || path.startsWith(n.to + '/')
  })
  if (match) return t(match.label)
  return t('nav.dashboard')
}