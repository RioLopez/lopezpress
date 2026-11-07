import { useI18n } from '../../../core/i18n'
import { fmtDate } from '../../../core/format'
import type { UserWithDisabled } from '../../../core/types'
import { isRoot, isAdminOnly } from '../../../core/roles'

export function UserTable({
  rows,
  selected,
  allSelected,
  busyId,
  viewerIsRoot,
  canManage,
  onSelectAll,
  onSelect,
  onChangeRole,
  onToggle,
  onDelete,
  onLogs,
  onReset,
}: {
  rows: UserWithDisabled[]
  selected: Set<string>
  allSelected: boolean
  busyId: string | null
  viewerIsRoot: boolean
  canManage: (u: UserWithDisabled) => boolean
  onSelectAll: (on: boolean) => void
  onSelect: (id: string, on: boolean) => void
  onChangeRole: (u: UserWithDisabled, role: string) => void
  onToggle: (u: UserWithDisabled, next: boolean) => void
  onDelete: (u: UserWithDisabled) => void
  onLogs: (u: UserWithDisabled) => void
  onReset: (u: UserWithDisabled) => void
}) {
  const { t, locale } = useI18n()

  return (
    <div class="table-scroll">
      <table class="admin-table users-table">
        <thead>
          <tr>
            <th class="col-select">
              <input
                type="checkbox"
                aria-label={t('admin.selectAll')}
                title={t('admin.selectAll')}
                checked={allSelected}
                onChange={(e) => onSelectAll((e.target as HTMLInputElement).checked)}
              />
            </th>
            <th>{t('admin.username')}</th>
            <th>{t('admin.role')}</th>
            <th>{t('admin.status')}</th>
            <th>{t('admin.lastLoginIp')}</th>
            <th>{t('admin.lastLoginAt')}</th>
            <th class="actions"></th>
          </tr>
        </thead>
        <tbody>
          {rows.map((u) => (
            <tr key={u.id} class={selected.has(u.id) ? 'row-selected' : undefined}>
              <td class="col-select">
                <input
                  type="checkbox"
                  aria-label={u.username}
                  checked={selected.has(u.id)}
                  onChange={(e) => onSelect(u.id, (e.target as HTMLInputElement).checked)}
                />
              </td>
              <td>
                <div class="admin-user-cell">
                  <span class="admin-user-name">
                    {u.display_name || u.username}
                    {u.display_name && <span class="muted"> ({u.username})</span>}
                  </span>
                  <span class="muted">{u.email}</span>
                  <span class="muted">{t('list.author')}: {fmtDate(u.created_at, locale)}</span>
                </div>
              </td>
              <td>
                {isRoot(u.role) ? (
                  <span class="badge-root">root</span>
                ) : (
                  <select
                    class="role-select"
                    value={u.role}
                    disabled={!canManage(u) || busyId === u.id}
                    onChange={(e) => onChangeRole(u, (e.target as HTMLSelectElement).value)}
                  >
                    <option value="user">{t('admin.roleUser')}</option>
                    {viewerIsRoot && <option value="admin">{t('admin.roleAdmin')}</option>}
                    {isAdminOnly(u.role) && !viewerIsRoot && <option value="admin">{t('admin.roleAdmin')}</option>}
                  </select>
                )}
              </td>
              <td>
                <span class={`post-status post-status-${u.disabled ? 'draft' : 'published'}`}>
                  {u.disabled ? t('admin.disabled') : t('admin.active')}
                </span>
              </td>
              <td class="mono-cell">{u.last_login_ip || '—'}</td>
              <td class="muted">{fmtDate(u.last_login_at, locale)}</td>
              <td class="actions">
                {/* The root account's login trail is root-only on the server:
                    an admin could otherwise profile where the super-admin
                    signs in from. Hide the entry instead of offering a button
                    that 403s. */}
                {(viewerIsRoot || u.role !== 'root') && (
                  <button class="btn btn-sm btn-ghost" disabled={busyId === u.id} onClick={() => onLogs(u)}>
                    {t('admin.loginHistory')}
                  </button>
                )}
                <button class="btn btn-sm btn-ghost" disabled={busyId === u.id} onClick={() => onReset(u)}>
                  {t('admin.resetPwd')}
                </button>
                <button
                  class={`btn btn-sm ${u.disabled ? 'btn-primary' : 'btn-danger'}`}
                  disabled={!canManage(u) || busyId === u.id}
                  onClick={() => onToggle(u, !u.disabled)}
                >
                  {u.disabled ? t('admin.enable') : t('admin.disable')}
                </button>
                <button
                  class="btn btn-sm btn-danger"
                  disabled={!canManage(u) || busyId === u.id}
                  onClick={() => onDelete(u)}
                >
                  {t('admin.deleteUser')}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
