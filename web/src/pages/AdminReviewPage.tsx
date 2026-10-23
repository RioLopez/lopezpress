import { useCallback, useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listPending, reviewPost, errMessage } from '../core/api'
import type { Post } from '../core/types'
import { fmtDate } from '../core/format'
import { Modal } from '../ui/Modal'
import { TableState } from '../ui/TableState'

// Content review workbench (spec 24). Every post authored by a plain user is
// reviewed by an admin; a post authored by an admin is reviewed by root. The
// backend already scopes the queue to the caller's tier, so this page simply
// renders whatever comes back and offers the two decisions.
export function AdminReviewPage() {
  const { user, hasToken } = useAuth()
  const { pushToast } = useApp()
  const { t, locale } = useI18n()
  const [items, setItems] = useState<Post[]>([])
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState<string | null>(null)

  // The rejection modal: which post we are rejecting and the note text.
  const [rejectTarget, setRejectTarget] = useState<Post | null>(null)
  const [note, setNote] = useState('')

  const load = useCallback(() => {
    setLoading(true)
    listPending()
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

  const approve = async (p: Post) => {
    setBusyId(p.id)
    try {
      await reviewPost(p.id, { decision: 'approve' })
      pushToast('success', t('admin.reviewApproved'))
      load()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusyId(null)
    }
  }

  const submitReject = async () => {
    if (!rejectTarget) return
    const trimmed = note.trim()
    if (!trimmed) {
      pushToast('error', t('admin.reviewNoteRequired'))
      return
    }
    const id = rejectTarget.id
    setBusyId(id)
    try {
      await reviewPost(id, { decision: 'reject', note: trimmed })
      pushToast('success', t('admin.reviewRejected'))
      setRejectTarget(null)
      setNote('')
      load()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusyId(null)
    }
  }


  return (
    <div class="admin-page">
      <div class="admin-head">
        <div>
          <h1>{t('admin.reviewTitle')}</h1>
          <p class="muted">{t('admin.reviewSub')}</p>
        </div>
      </div>

      <TableState loading={loading} isEmpty={items.length === 0} loadingText={t('common.loading')} emptyText={t('admin.reviewEmpty')}>
        <div class="table-scroll">
          <table class="admin-table">
            <thead>
              <tr>
                <th>{t('admin.name')}</th>
                <th>{t('list.author')}</th>
                <th>{t('admin.submittedAt')}</th>
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
                  <td>
                    <div class="admin-user-cell">
                      <span>{p.author?.display_name || p.author?.username || p.author_id}</span>
                      <span class="muted">{p.author?.email}</span>
                    </div>
                  </td>
                  <td class="muted">{fmtDate(p.created_at, locale)}</td>
                  <td class="actions">
                    <button
                      class="btn btn-primary btn-sm"
                      disabled={busyId === p.id}
                      onClick={() => approve(p)}
                    >
                      {t('admin.approve')}
                    </button>
                    <button
                      class="btn btn-danger btn-sm"
                      disabled={busyId === p.id}
                      onClick={() => {
                        setRejectTarget(p)
                        setNote('')
                      }}
                    >
                      {t('admin.reject')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </TableState>

      {rejectTarget && (
        <Modal
          title={`${t('admin.rejectTitle')} — ${rejectTarget.title}`}
          onClose={() => setRejectTarget(null)}
          footer={
            <>
              <button class="btn btn-ghost" onClick={() => setRejectTarget(null)}>
                {t('editor.cancel')}
              </button>
              <button class="btn btn-primary" disabled={busyId === rejectTarget.id} onClick={submitReject}>
                {busyId === rejectTarget.id ? '…' : t('admin.reject')}
              </button>
            </>
          }
        >
          <div class="settings-field">
            <label>{t('admin.reviewNote')}</label>
            <textarea
              rows={4}
              value={note}
              placeholder={t('admin.reviewNotePlaceholder')}
              onInput={(e) => setNote((e.target as HTMLTextAreaElement).value)}
            />
          </div>
          <p class="muted">{t('admin.rejectHint')}</p>
        </Modal>
      )}
    </div>
  )
}
