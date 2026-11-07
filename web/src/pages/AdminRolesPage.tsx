import { useMemo } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listUsers } from '../core/api'
import type { UserWithDisabled } from '../core/types'
import { useQuery } from '../core/useQuery'

interface RoleCard {
  role: string
  badge: string
  desc: string
}

// The three roles are fixed by the backend (models.RoleRoot / RoleAdmin /
// RoleUser); this page documents their boundaries and shows who holds each.
// Adding a role is a backend change, so nothing here is editable.
const ROLES: RoleCard[] = [
  { role: 'root', badge: 'badge-root', desc: 'admin.roleRootDesc' },
  { role: 'admin', badge: 'badge-admin', desc: 'admin.roleAdminDesc' },
  { role: 'user', badge: 'badge-user', desc: 'admin.roleUserDesc' },
]

const MATRIX: { key: string; root: boolean; admin: boolean; user: boolean }[] = [
  { key: 'admin.capPublish', root: true, admin: false, user: false },
  { key: 'admin.capReview', root: true, admin: true, user: false },
  { key: 'admin.capEditOthers', root: true, admin: true, user: false },
  { key: 'admin.capEditPeers', root: true, admin: false, user: false },
  { key: 'admin.capTrash', root: true, admin: true, user: false },
  { key: 'admin.capManageUsers', root: true, admin: true, user: false },
  { key: 'admin.capManageAdmins', root: true, admin: false, user: false },
  { key: 'admin.capBanVault', root: true, admin: false, user: false },
  { key: 'admin.capRootSite', root: true, admin: false, user: false },
  { key: 'admin.capImportBackup', root: true, admin: false, user: false },
]

export function AdminRolesPage() {
  const { pushToast } = useApp()
  const { t } = useI18n()
  const { data, loading } = useQuery(
    () => listUsers(),
    [],
    { onError: () => pushToast('error', t('admin.failed')) },
  )
  const users = data ?? []

  const counts = useMemo(() => {
    const out: Record<string, number> = { root: 0, admin: 0, user: 0 }
    for (const u of users) out[u.role] = (out[u.role] ?? 0) + 1
    return out
  }, [users])

  const membersOf = (role: string) => users.filter((u) => u.role === role)

  return (
    <div class="admin-page">
      <div class="admin-head">
        <div>
          <h1>{t('admin.rolesTitle')}</h1>
          <p class="muted">{t('admin.rolesSub')}</p>
        </div>
      </div>

      <div class="dash-cards">
        {ROLES.map((r) => (
          <div class="dash-card" key={r.role}>
            <span class={r.badge}>{r.role}</span>
            <div class="dash-card-num">{counts[r.role] ?? 0}</div>
            <div class="dash-card-label">{t(r.desc)}</div>
          </div>
        ))}
      </div>

      <section class="dash-block">
        <h3>{t('admin.roleMatrixTitle')}</h3>
        <p class="muted">{t('admin.roleMatrixNote')}</p>
        <div class="table-scroll">
          <table class="admin-table role-matrix">
            <thead>
              <tr>
                <th>{t('admin.roleMatrixCapability')}</th>
                <th>root</th>
                <th>admin</th>
                <th>user</th>
              </tr>
            </thead>
            <tbody>
              {MATRIX.map((row) => (
                <tr key={row.key}>
                  <td>{t(row.key)}</td>
                  {[row.root, row.admin, row.user].map((allowed, i) => (
                    <td key={i} class={allowed ? 'role-matrix-yes' : 'role-matrix-no'}>
                      {allowed ? '✓' : '—'}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section class="dash-block">
        <h3>{t('admin.roleMembers')}</h3>
        {loading ? (
          <p class="muted">{t('common.loading')}</p>
        ) : (
          ROLES.map((r) => (
            <RoleMembers
              key={r.role}
              role={r}
              members={membersOf(r.role)}
              emptyText={t('admin.noRoleMembers')}
              disabledText={t('admin.disabled')}
            />
          ))
        )}
      </section>
    </div>
  )
}

function RoleMembers({
  role,
  members,
  emptyText,
  disabledText,
}: {
  role: RoleCard
  members: UserWithDisabled[]
  emptyText: string
  disabledText: string
}) {
  return (
    <div class="role-members-block">
      <div class="role-members-head">
        <span class={role.badge}>{role.role}</span>
        <span class="muted">{members.length}</span>
      </div>
      {members.length === 0 ? (
        <p class="muted">{emptyText}</p>
      ) : (
        <div class="role-member-list">
          {members.map((u) => (
            <span class="role-member" key={u.id}>
              {u.display_name || u.username}
              {u.disabled && <span class="muted"> ({disabledText})</span>}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}
