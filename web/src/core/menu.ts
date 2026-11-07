import type { MenuItem } from './types'

// The sidebar used to be this array in AdminLayout.tsx. It is now only a
// fallback for the first paint and for a failed /api/admin/menus request —
// the real order and visibility live in the menu_items table. Paths are
// site-relative, matching the server's validation.
export const FALLBACK_MENU: MenuItem[] = [
  { id: -1, key: 'dashboard', label_key: 'nav.dashboard', label: '', path: '/admin', group_key: '', root_only: false, visible: true, sort: 0 },
  { id: -2, key: 'posts', label_key: 'admin.postsTitle', label: '', path: '/admin/posts', group_key: 'content', root_only: false, visible: true, sort: 10 },
  { id: -3, key: 'review', label_key: 'admin.reviewTitle', label: '', path: '/admin/review', group_key: 'content', root_only: false, visible: true, sort: 20 },
  { id: -4, key: 'categories', label_key: 'admin.categoriesTitle', label: '', path: '/admin/categories', group_key: 'content', root_only: false, visible: true, sort: 30 },
  { id: -5, key: 'tags', label_key: 'admin.tagsTitle', label: '', path: '/admin/tags', group_key: 'content', root_only: false, visible: true, sort: 40 },
  { id: -6, key: 'trash', label_key: 'admin.trashTitle', label: '', path: '/admin/trash', group_key: 'content', root_only: false, visible: true, sort: 50 },
  { id: -7, key: 'banned', label_key: 'admin.banVaultTitle', label: '', path: '/admin/banned', group_key: 'content', root_only: true, visible: true, sort: 60 },
  { id: -8, key: 'media', label_key: 'media.title', label: '', path: '/admin/media', group_key: 'content', root_only: false, visible: true, sort: 70 },
  { id: -9, key: 'users', label_key: 'admin.usersTitle', label: '', path: '/admin/users', group_key: 'users', root_only: false, visible: true, sort: 80 },
  { id: -10, key: 'roles', label_key: 'admin.rolesTitle', label: '', path: '/admin/roles', group_key: 'users', root_only: false, visible: true, sort: 90 },
  { id: -11, key: 'site', label_key: 'site.title', label: '', path: '/admin/site', group_key: 'system', root_only: false, visible: true, sort: 100 },
  { id: -12, key: 'menus', label_key: 'admin.menuAdmin.title', label: '', path: '/admin/menus', group_key: 'system', root_only: true, visible: true, sort: 110 },
  { id: -13, key: 'backup', label_key: 'admin.backupTitle', label: '', path: '/admin/backup', group_key: 'system', root_only: false, visible: true, sort: 120 },
  { id: -14, key: 'audit', label_key: 'audit.title', label: '', path: '/admin/audit', group_key: 'system', root_only: false, visible: true, sort: 130 },
]

// menuLabel prefers the operator's override, then the built-in i18n value.
// A custom row has no label_key, so its literal label is the only source.
export function menuLabel(item: MenuItem, t: (key: string) => string): string {
  return item.label || (item.label_key ? t(item.label_key) : item.path)
}

// groupLabel maps the stored group key to the existing nav translations.
export function groupLabel(key: string, t: (key: string) => string): string {
  switch (key) {
    case 'content':
      return t('nav.groupContent')
    case 'users':
      return t('nav.groupUsers')
    case 'system':
      return t('nav.groupSystem')
    default:
      return ''
  }
}

// MENU_GROUPS feeds the group select on the management page.
export const MENU_GROUPS = ['content', 'users', 'system', ''] as const
