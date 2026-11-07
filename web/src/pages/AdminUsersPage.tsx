import { useEffect, useMemo, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listUsers, setUserDisabled, updateUser, deleteUser, listUserLogins, batchSetUserDisabled, errMessage } from '../core/api'
import type { LoginLog, UserWithDisabled } from '../core/types'
import { TableState } from '../ui/TableState'
import { Pagination } from '../ui/Pagination'
import { useQuery } from '../core/useQuery'
import { isRoot, isAdminOnly } from '../core/roles'
import { CreateUserModal } from './admin/users/CreateUserModal'
import { ResetPasswordModal } from './admin/users/ResetPasswordModal'
import { LoginHistoryModal } from './admin/users/LoginHistoryModal'
import { UserFilters } from './admin/users/UserFilters'
import { UserTable } from './admin/users/UserTable'

export function AdminUsersPage() {
  const { user } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t } = useI18n()
  const [busyId, setBusyId] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [batchBusy, setBatchBusy] = useState(false)

  // List controls: search + role/status filters + page size. The API returns
  // every account, so filtering and paging happen here — no contract change.
  const [query, setQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(10)

  // Modals / drawer state.
  const [createOpen, setCreateOpen] = useState(false)
  const [resetTarget, setResetTarget] = useState<UserWithDisabled | null>(null)
  const [logsTarget, setLogsTarget] = useState<UserWithDisabled | null>(null)
  const [logs, setLogs] = useState<LoginLog[]>([])
  const [logsLoading, setLogsLoading] = useState(false)

  const { data, loading, reload } = useQuery(
    () => listUsers(),
    [],
    { onError: () => pushToast('error', t('admin.failed')) },
  )
  const users = data ?? []

  // Prune selection whenever the user list changes so we never act on a
  // removed row.
  useEffect(() => {
    setSelected((prev) => {
      const ids = new Set((data ?? []).map((u) => u.id))
      const next = new Set(Array.from(prev).filter((id) => ids.has(id)))
      return next.size === prev.size ? prev : next
    })
  }, [data])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return users.filter((u) => {
      if (roleFilter && u.role !== roleFilter) return false
      if (statusFilter === 'active' && u.disabled) return false
      if (statusFilter === 'disabled' && !u.disabled) return false
      if (!q) return true
      return (
        u.username.toLowerCase().includes(q) ||
        u.email.toLowerCase().includes(q) ||
        (u.display_name || '').toLowerCase().includes(q)
      )
    })
  }, [users, query, roleFilter, statusFilter])

  const totalPages = Math.max(1, Math.ceil(filtered.length / size))
  const currentPage = Math.min(page, totalPages)
  const pageRows = filtered.slice((currentPage - 1) * size, currentPage * size)

  const allSelected = useMemo(
    () => filtered.length > 0 && filtered.every((u) => selected.has(u.id)),
    [filtered, selected],
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
    setSelected(on ? new Set(filtered.map((u) => u.id)) : new Set())
  }

  const me = user!
  const isRootUser = isRoot(me.role)

  // Role-gated management: root can manage admin (and sees the root badge),
  // admin can manage plain users. Each row's actions are locked down by the
  // caller's tier. The caller can never manage their own account (self-
  // protection mirrors the backend, which 403s on self-modification).
  const canManage = (u: UserWithDisabled) => {
    if (u.id === me.id) return false
    if (isRoot(u.role)) return isRootUser // only root may manage roots
    if (isAdminOnly(u.role)) return isRootUser // only root may manage admins
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
      reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
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
      reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    }
  }

  const doDelete = async (u: UserWithDisabled) => {
    if (!canManage(u)) return
    if (!(await confirmDialog(t('admin.deleteConfirmUser', { t: u.username })))) return
    setBusyId(u.id)
    try {
      await deleteUser(u.id)
      pushToast('success', t('admin.deleted'))
      reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
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
      reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
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

      <UserFilters
        query={query}
        roleFilter={roleFilter}
        statusFilter={statusFilter}
        size={size}
        onQuery={(v) => {
          setQuery(v)
          setPage(1)
        }}
        onRole={(v) => {
          setRoleFilter(v)
          setPage(1)
        }}
        onStatus={(v) => {
          setStatusFilter(v)
          setPage(1)
        }}
        onSize={(v) => {
          setSize(v)
          setPage(1)
        }}
      />

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

      <TableState loading={loading} isEmpty={filtered.length === 0} loadingText={t('common.loading')} emptyText={users.length === 0 ? t('admin.emptyUsers') : t('admin.noMatchingUsers')}>
        <UserTable
          rows={pageRows}
          selected={selected}
          allSelected={allSelected}
          busyId={busyId}
          viewerIsRoot={isRootUser}
          canManage={canManage}
          onSelectAll={toggleSelectAll}
          onSelect={toggleSelect}
          onChangeRole={changeRole}
          onToggle={toggle}
          onDelete={doDelete}
          onLogs={openLogs}
          onReset={setResetTarget}
        />
      </TableState>

      <Pagination page={currentPage} totalPages={totalPages} onPage={setPage} />

      {createOpen && (
        <CreateUserModal
          onClose={() => setCreateOpen(false)}
          onCreated={() => {
            setCreateOpen(false)
            reload()
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
