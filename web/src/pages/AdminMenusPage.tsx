import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import {
  listAdminMenus,
  createMenuItem,
  updateMenuItem,
  deleteMenuItem,
  reorderMenuItems,
  errMessage,
} from '../core/api'
import { groupLabel, MENU_GROUPS } from '../core/menu'
import type { MenuItem } from '../core/types'
import { MenuRow } from './admin/menus/MenuRow'

// Sidebar manager (root-only). Built-in rows ship with the console and can be
// renamed, hidden or reordered; only operator-added links can be deleted. The
// server enforces the same rule, this page just does not offer a button that
// would be refused.
export function AdminMenusPage() {
  const { t } = useI18n()
  const { pushToast, confirmDialog } = useApp()
  const [rows, setRows] = useState<MenuItem[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [label, setLabel] = useState('')
  const [path, setPath] = useState('')
  const [group, setGroup] = useState('system')

  const apply = (next: MenuItem[]) => {
    setRows(next)
    // AdminLayout listens for this so the sidebar updates without a reload.
    window.dispatchEvent(new Event('menus-updated'))
  }

  const load = async () => {
    try {
      setRows(await listAdminMenus())
    } catch (e) {
      pushToast('error', errMessage(e, t('menuAdmin.saveFailed')))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const run = async (fn: () => Promise<unknown>, done: string) => {
    setBusy(true)
    try {
      const res = await fn()
      apply(res && Array.isArray(res) ? res : await listAdminMenus())
      pushToast('success', done)
    } catch (e) {
      pushToast('error', errMessage(e, t('menuAdmin.saveFailed')))
    } finally {
      setBusy(false)
    }
  }

  const move = (index: number, dir: -1 | 1) => {
    const target = index + dir
    if (target < 0 || target >= rows.length) return
    const ordered = [...rows]
    ;[ordered[index], ordered[target]] = [ordered[target], ordered[index]]
    run(() => reorderMenuItems(ordered.map((r) => r.id)), t('menuAdmin.saved'))
  }

  const remove = async (row: MenuItem) => {
    if (!(await confirmDialog(t('menuAdmin.deleteConfirm')))) return
    run(() => deleteMenuItem(row.id), t('menuAdmin.saved'))
  }

  const add = () => {
    if (!label.trim() || !path.trim()) return
    run(async () => {
      await createMenuItem({ label: label.trim(), path: path.trim(), group_key: group })
      setLabel('')
      setPath('')
      return listAdminMenus()
    }, t('menuAdmin.saved'))
  }

  return (
    <div class="admin-page">
      <h1>{t('menuAdmin.title')}</h1>
      <p class="muted">{t('menuAdmin.sub')}</p>

      {loading ? (
        <p class="muted">{t('common.loading')}</p>
      ) : rows.length === 0 ? (
        <p class="muted">{t('menuAdmin.empty')}</p>
      ) : (
        <div class="menu-list">
          {rows.map((row, i) => (
            <MenuRow
              key={`${row.id}-${row.sort}-${row.label}-${row.visible}`}
              row={row}
              index={i}
              total={rows.length}
              busy={busy}
              onMove={(dir) => move(i, dir)}
              onSave={(patch) => run(() => updateMenuItem(row.id, patch), t('menuAdmin.saved'))}
              onToggle={() =>
                run(
                  () =>
                    updateMenuItem(row.id, {
                      label: row.label,
                      path: row.path,
                      group_key: row.group_key,
                      visible: !row.visible,
                    }),
                  t('menuAdmin.saved'),
                )
              }
              onDelete={() => remove(row)}
            />
          ))}
        </div>
      )}

      <p class="muted menu-order-hint">{t('menuAdmin.orderHint')}</p>

      <section class="settings-section menu-add">
        <h3>{t('menuAdmin.add')}</h3>
        <div class="menu-add-row">
          <input
            value={label}
            placeholder={t('menuAdmin.labelPlaceholder')}
            maxLength={64}
            onInput={(e) => setLabel((e.target as HTMLInputElement).value)}
          />
          <input
            value={path}
            placeholder={t('menuAdmin.pathPlaceholder')}
            maxLength={256}
            onInput={(e) => setPath((e.target as HTMLInputElement).value)}
          />
          <select value={group} onChange={(e) => setGroup((e.target as HTMLSelectElement).value)}>
            {MENU_GROUPS.map((g) => (
              <option key={g || 'none'} value={g}>
                {g ? groupLabel(g, t) : t('menuAdmin.groupNone')}
              </option>
            ))}
          </select>
          <button class="btn btn-primary btn-sm" disabled={busy} onClick={add}>
            {t('menuAdmin.add')}
          </button>
        </div>
      </section>
    </div>
  )
}
