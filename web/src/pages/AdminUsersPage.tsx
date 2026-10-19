import { useCallback, useEffect, useMemo, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import {
  listUsers,
  setUserDisabled,
  updateUser,
  createUser,
  resetPassword,
  deleteUser,
  listUserLogins,
  batchSetUserDisabled,
  ApiError,
} from '../core/api'
import type { LoginLog, UserWithDisabled } from '../core/types'
import { fmtDate } from '../core/format'
import { Modal } from '../ui/Modal'
import { TableState } from '../ui/TableState'

export function AdminUsersPage() {
  const { user, hasToken } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const [users, setUsers] = useState<UserWithDisabled[]>([])
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [batchBusy, setBatchBusy] = useState(false)

  // Modals / drawer state.
  const [createOpen, setCreateOpen] = useState(false)
  const [resetTarget, setResetTarget] = useState<UserWithDisabled | null>(null)
  const [logsTarget, setLogsTarget] = useState<UserWithDisabled | null>(null)
  const [logs, setLogs] = useState<LoginLog[]>([])
  const [logsLoading, setLogsLoading] = useState(false)

  const load = useCallback(() => {
    setLoading(true)
    listUsers()
      .then(setUsers)
      .catch(() => pushToast('error', t('admin.failed')))
      .finally(() => setLoading(false))
  }, [pushToast, t])

  useEffect(() => {
    if (user?.role === 'admin' || user?.role === 'root') load()
  }, [user, load])

  // Prune selection whenever the user list changes so we never act on a
  // removed row.
  useEffect(() => {
    setSelected((prev) => {
      const ids = new Set(users.map((u) => u.id))
      const next = new Set(Array.from(prev).filter((id) => ids.has(id)))
      return next.size === prev.size ? prev : next
    })
  }, [users])

  const allSelected = useMemo(
    () => users.length > 0 && users.every((u) => selected.has(u.id)),
    [users, selected],
  )

  const toggleSelect = (id: string, on: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (on) next.add(id)
      else next.delete(id)
      return next
    })
  }

  const toggleSelectAll = (on: boolean) => {
    setSelected(on ? new Set(users.map((u) => u.id)) : new Set())
  }

  if (!hasToken || (user?.role !== 'admin' && user?.role !== 'root')) {
    return <div class="admin-page"><p class="muted">{t('admin.notAdmin')}</p></div>
  }

  const me = user!
  const isRoot = me.role === 'root'

  // Role-gated management: root can manage admin (and sees the root badge),
  // admin can manage plain users. Each row's actions are locked down by the
  // caller's tier. The caller can never manage their own account (self-
  // protection mirrors the backend, which 403s on self-modification).
  const canManage = (u: UserWithDisabled) => {
    if (u.id === me.id) return false
    if (u.role === 'root') return isRoot
    if (u.role === 'admin') return isRoot // only root may manage admins
    return true // plain users: any admin-or-root
  }

  const toggle = async (u: UserWithDisabled, next: boolean) => {
    if (!canManage(u)) return
    const confirmMsg = next
      ? t('admin.confirmDisable', { t: u.username })
      : t('admin.confirmEnable', { t: u.username })
    if (!(await confirmDialog(confirmMsg))) return
    setBusyId(u.id)
    try {
      await setUserDisabled(u.id, next)
      pushToast('success', t('admin.saved'))
      load()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    } finally {
      setBusyId(null)
    }
  }

  const changeRole = async (u: UserWithDisabled, role: string) => {
    if (role === u.role) return
    if (!canManage(u)) {
      pushToast('error', t('admin.roleLocked'))
      return
    }
    try {
      await updateUser(u.id, { role })
      pushToast('success', t('admin.saved'))
      load()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    }
  }

  const doDelete = async (u: UserWithDisabled) => {
    if (!canManage(u)) return
    if (!(await confirmDialog(t('admin.deleteConfirmUser', { t: u.username })))) return
    setBusyId(u.id)
    try {
      await deleteUser(u.id)
      pushToast('success', t('admin.deleted'))
      load()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    } finally {
      setBusyId(null)
    }
  }

  const batchRun = async (disable: boolean) => {
    if (selected.size === 0 || batchBusy) return
    const count = selected.size
    const msg = disable
      ? t('admin.batchConfirmDisable', { n: String(count) })
      : t('admin.batchConfirmEnable', { n: String(count) })
    if (!(await confirmDialog(msg))) return
    setBatchBusy(true)
    try {
      const ids = Array.from(selected)
      const { ok, fail } = await batchSetUserDisabled(ids, disable)
      if (fail === 0) {
        pushToast('success', t('admin.batchDone', { n: String(ok) }))
      } else {
        pushToast(
          fail === ok ? 'success' : 'error',
          t('admin.batchFailed', { ok: String(ok), fail: String(fail) }),
        )
      }
      setSelected(new Set())
      load()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    } finally {
      setBatchBusy(false)
    }
  }

  const openLogs = (u: UserWithDisabled) => {
    setLogsTarget(u)
    setLogsLoading(true)
    setLogs([])
    listUserLogins(u.id, 100)
      .then(setLogs)
      .catch(() => pushToast('error', t('admin.failed')))
      .finally(() => setLogsLoading(false))
  }


  const selectedCount = selected.size

  return (
    <div class="admin-page">
      <div class="admin-head">
        <div>
          <h1>{t('admin.usersTitle')}</h1>
          <p class="muted">{t('admin.usersSub')}</p>
        </div>
        <button class="btn btn-primary btn-sm" onClick={() => setCreateOpen(true)}>
          + {t('admin.createUser')}
        </button>
      </div>

      {selectedCount > 0 && (
        <div class="admin-batchbar">
          <span class="admin-batchbar-count">
            {t('admin.selectedCount', { n: String(selectedCount) })}
          </span>
          <span class="admin-batchbar-spacer" />
          <button
            class="btn btn-sm btn-primary"
            disabled={batchBusy}
            onClick={() => batchRun(false)}
          >
            {t('admin.batchEnable')}
          </button>
          <button
            class="btn btn-sm btn-danger"
            disabled={batchBusy}
            onClick={() => batchRun(true)}
          >
            {t('admin.batchDisable')}
          </button>
        </div>
      )}

      <TableState loading={loading} isEmpty={users.length === 0} loadingText={t('common.loading')} emptyText={t('admin.emptyUsers')}>
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
                  onChange={(e) => toggleSelectAll((e.target as HTMLInputElement).checked)}
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
            {users.map((u) => (
              <tr key={u.id} class={selected.has(u.id) ? 'row-selected' : undefined}>
                <td class="col-select">
                  <input
                    type="checkbox"
                    aria-label={u.username}
                    checked={selected.has(u.id)}
                    onChange={(e) => toggleSelect(u.id, (e.target as HTMLInputElement).checked)}
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
                  {u.role === 'root' ? (
                    <span class="badge-root">root</span>
                  ) : (
                    <select
                      class="role-select"
                      value={u.role}
                      disabled={!canManage(u) || busyId === u.id}
                      onChange={(e) => changeRole(u, (e.target as HTMLSelectElement).value)}
                    >
                      <option value="user">{t('admin.roleUser')}</option>
                      {isRoot && <option value="admin">{t('admin.roleAdmin')}</option>}
                      {u.role === 'admin' && !isRoot && <option value="admin">{t('admin.roleAdmin')}</option>}
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
                  <button class="btn btn-sm btn-ghost" disabled={busyId === u.id} onClick={() => openLogs(u)}>
                    {t('admin.loginHistory')}
                  </button>
                  <button class="btn btn-sm btn-ghost" disabled={busyId === u.id} onClick={() => setResetTarget(u)}>
                    {t('admin.resetPwd')}
                  </button>
                  <button
                    class={`btn btn-sm ${u.disabled ? 'btn-primary' : 'btn-danger'}`}
                    disabled={!canManage(u) || busyId === u.id}
                    onClick={() => toggle(u, !u.disabled)}
                  >
                    {u.disabled ? t('admin.enable') : t('admin.disable')}
                  </button>
                  <button
                    class="btn btn-sm btn-danger"
                    disabled={!canManage(u) || busyId === u.id}
                    onClick={() => doDelete(u)}
                  >
                    {t('admin.deleteUser')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      </TableState>

      {createOpen && (
        <CreateUserModal
          onClose={() => setCreateOpen(false)}
          onCreated={() => {
            setCreateOpen(false)
            load()
          }}
        />
      )}

      {resetTarget && (
        <ResetPasswordModal
          user={resetTarget}
          onClose={() => setResetTarget(null)}
          onDone={() => setResetTarget(null)}
        />
      )}

      {logsTarget && (
        <LoginHistoryModal
          user={logsTarget}
          logs={logs}
          loading={logsLoading}
          onClose={() => setLogsTarget(null)}
        />
      )}
    </div>
  )
}

function CreateUserModal({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: () => void
}) {
  const { pushToast } = useApp()
  const { t } = useI18n()
  const { user } = useAuth()
  const isRoot = user?.role === 'root'
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('user')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!username.trim() || !email.trim() || !password) {
      pushToast('error', t('auth.empty'))
      return
    }
    if (password.length < 8) {
      pushToast('error', t('auth.passwordTooShort'))
      return
    }
    setBusy(true)
    try {
      await createUser({ username: username.trim(), email: email.trim(), password, role })
      pushToast('success', t('admin.saved'))
      onCreated()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={t('admin.newUser')}
      onClose={onClose}
      footer={
        <>
          <button class="btn btn-ghost" onClick={onClose}>{t('editor.cancel')}</button>
          <button class="btn btn-primary" disabled={busy} onClick={submit}>
            {busy ? '…' : t('editor.save')}
          </button>
        </>
      }
    >
      <div class="settings-field">
        <label>{t('admin.newUsername')}</label>
        <input value={username} onInput={(e) => setUsername((e.target as HTMLInputElement).value)} />
      </div>
      <div class="settings-field">
        <label>{t('admin.newEmail')}</label>
        <input type="email" value={email} onInput={(e) => setEmail((e.target as HTMLInputElement).value)} />
      </div>
      <div class="settings-field">
        <label>{t('admin.newPassword')}</label>
        <input
          type="text"
          value={password}
          onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
        />
      </div>
      <div class="settings-field">
        <label>{t('admin.newRole')}</label>
        <select value={role} onChange={(e) => setRole((e.target as HTMLSelectElement).value)}>
          <option value="user">{t('admin.roleUser')}</option>
          {/* Only root may provision admins; admins create plain users. */}
          {isRoot && <option value="admin">{t('admin.roleAdmin')}</option>}
        </select>
      </div>
    </Modal>
  )
}

function ResetPasswordModal({
  user,
  onClose,
  onDone,
}: {
  user: UserWithDisabled
  onClose: () => void
  onDone: () => void
}) {
  const { pushToast } = useApp()
  const { t } = useI18n()
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (password.length < 8) {
      pushToast('error', t('auth.passwordTooShort'))
      return
    }
    if (!(await window.confirm(t('admin.resetPwdConfirm', { t: user.username })))) return
    setBusy(true)
    try {
      await resetPassword(user.id, password)
      pushToast('success', t('admin.saved'))
      onDone()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={`${t('admin.resetPwdTitle')} — ${user.username}`}
      onClose={onClose}
      footer={
        <>
          <button class="btn btn-ghost" onClick={onClose}>{t('editor.cancel')}</button>
          <button class="btn btn-primary" disabled={busy} onClick={submit}>
            {busy ? '…' : t('editor.save')}
          </button>
        </>
      }
    >
      <div class="settings-field">
        <label>{t('admin.newPassword')}</label>
        <input
          type="text"
          placeholder={t('admin.newPwdPlaceholder')}
          value={password}
          onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
          onKeyDown={(e) => e.key === 'Enter' && submit()}
        />
      </div>
    </Modal>
  )
}

function LoginHistoryModal({
  user,
  logs,
  loading,
  onClose,
}: {
  user: UserWithDisabled
  logs: LoginLog[]
  loading: boolean
  onClose: () => void
}) {
  const { t, locale } = useI18n()

  return (
    <Modal
      wide
      title={`${t('admin.loginHistory')} — ${user.username}`}
      onClose={onClose}
      footer={<button class="btn btn-ghost" onClick={onClose}>{t('editor.cancel')}</button>}
    >
      {loading ? (
        <p class="muted">{t('common.loading')}</p>
      ) : logs.length === 0 ? (
        <p class="muted">{t('admin.noLogs')}</p>
      ) : (
        <table class="admin-table login-log-table">
          <thead>
            <tr>
              <th>{t('admin.loginTime')}</th>
              <th>{t('admin.loginIp')}</th>
              <th>{t('admin.loginOk')}</th>
              <th>UA</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((l) => (
              <tr key={l.id}>
                <td class="muted">{fmtDate(l.created_at, locale)}</td>
                <td class="mono-cell">{l.ip}</td>
                <td>
                  <span class={`post-status post-status-${l.success ? 'published' : 'draft'}`}>
                    {l.success ? t('admin.loginOk') : t('admin.loginFail')}
                  </span>
                </td>
                <td class="muted ua-cell" title={l.ua}>{l.ua}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Modal>
  )
}
