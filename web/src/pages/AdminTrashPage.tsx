import { useCallback, useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listTrash, restoreTrash, emptyTrash, errMessage } from '../core/api'
import type { TrashPost } from '../core/types'
import { fmtDate } from '../core/format'
import { TableState } from '../ui/TableState'

export function AdminTrashPage() {
  const { user, hasToken } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const [items, setItems] = useState<TrashPost[]>([])
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [emptying, setEmptying] = useState(false)

  const load = useCallback(() => {
    setLoading(true)
    listTrash()
      .then(setItems)
      .catch(() => pushToast('error', t('admin.failed')))
      .finally(() => setLoading(false))
  }, [pushToast, t])

  useEffect(() => {
    if (user?.role === 'admin' || user?.role === 'root') load()
  }, [user, load])

  if (!hasToken || (user?.role !== 'admin' && user?.role !== 'root')) {
    return <div class="admin-page"><p class="muted">{t('admin.notAdmin')}</p></div>
  }

  const restore = async (p: TrashPost) => {
    if (!(await confirmDialog(t('admin.restoreConfirm', { title: p.title })))) return
    setBusyId(p.id)
    try {
      await restoreTrash(p.id)
      pushToast('success', t('admin.restored'))
      load()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusyId(null)
    }
  }

  const empty = async () => {
    if (!(await confirmDialog(t('admin.emptyTrashConfirm')))) return
    setEmptying(true)
    try {
      const r = await emptyTrash()
      pushToast('success', t('admin.emptyTrashDone', { n: String(r.deleted) }))
      load()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setEmptying(false)
    }
  }


  return (
    <div class="admin-page">
      <div class="admin-head">
        <h1>{t('admin.trashTitle')}</h1>
        {items.length > 0 && (
          <button class="btn btn-danger btn-sm" disabled={emptying} onClick={empty}>
            {t('admin.emptyTrashBtn')}
          </button>
        )}
      </div>
      <p class="muted">{t('admin.trashSub')}</p>

      <TableState loading={loading} isEmpty={items.length === 0} loadingText={t('common.loading')} emptyText={t('admin.emptyTrash')}>
        <div class="table-scroll">
        <table class="admin-table">
          <thead>
            <tr>
              <th>{t('admin.name')}</th>
              <th>{t('list.author')}</th>
              <th>{t('admin.deletedAt')}</th>
              <th class="actions"></th>
            </tr>
          </thead>
          <tbody>
            {items.map((p) => (
              <tr key={p.id}>
                <td>
                  <div class="admin-user-cell">
                    <span>{p.title}</span>
                    <span class="muted">{p.view_count} {t('list.views')}</span>
                  </div>
                </td>
                <td>{p.author?.username || p.author_id}</td>
                <td class="muted">{fmtDate(p.deleted_at, locale)}</td>
                <td class="actions">
                  <button
                    class="btn btn-primary btn-sm"
                    disabled={busyId === p.id}
                    onClick={() => restore(p)}
                  >
                    {t('admin.restore')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      </TableState>

      <p class="muted admin-trash-hint">{t('admin.expiredHint')}</p>
    </div>
  )
}
