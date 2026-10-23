import { useCallback, useEffect, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listBanned, sendBackBanned, deleteBanned, errMessage } from '../core/api'
import type { Post } from '../core/types'
import { fmtDate } from '../core/format'
import { Modal } from '../ui/Modal'
import { TableState } from '../ui/TableState'

// Ban vault (spec 24): the root-only view of every banned post. The author
// (and every admin) cannot see these posts anywhere else in the app, so this
// page is the single place they can be managed. Two actions are available:
// send the post back to its author with a note, or delete it permanently.
export function AdminBanVaultPage() {
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const [items, setItems] = useState<Post[]>([])
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState<string | null>(null)

  // The send-back modal: which post we are returning and the note text.
  const [sendBackTarget, setSendBackTarget] = useState<Post | null>(null)
  const [note, setNote] = useState('')

  const load = useCallback(() => {
    setLoading(true)
    listBanned()
      .then(setItems)
      .catch(() => pushToast('error', t('admin.failed')))
      .finally(() => setLoading(false))
  }, [pushToast, t])

  useEffect(() => {
    load()
  }, [load])

  const submitSendBack = async () => {
    if (!sendBackTarget) return
    const trimmed = note.trim()
    if (!trimmed) {
      pushToast('error', t('admin.reviewNoteRequired'))
      return
    }
    const id = sendBackTarget.id
    setBusyId(id)
    try {
      await sendBackBanned(id, trimmed)
      pushToast('success', t('admin.sendBackDone'))
      setSendBackTarget(null)
      setNote('')
      load()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusyId(null)
    }
  }

  const remove = async (p: Post) => {
    if (!(await confirmDialog(t('admin.deleteBannedConfirm', { t: p.title })))) return
    setBusyId(p.id)
    try {
      await deleteBanned(p.id)
      pushToast('success', t('admin.deleted'))
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
          <h1>{t('admin.banVaultTitle')}</h1>
          <p class="muted">{t('admin.banVaultSub')}</p>
        </div>
      </div>

      <TableState loading={loading} isEmpty={items.length === 0} loadingText={t('common.loading')} emptyText={t('admin.banVaultEmpty')}>
        <div class="table-scroll">
          <table class="admin-table">
            <thead>
              <tr>
                <th>{t('admin.name')}</th>
                <th>{t('list.author')}</th>
                <th>{t('admin.bannedAt')}</th>
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
                  <td class="muted">{fmtDate(p.updated_at, locale)}</td>
                  <td class="actions">
                    <button
                      class="btn btn-primary btn-sm"
                      disabled={busyId === p.id}
                      onClick={() => {
                        setSendBackTarget(p)
                        setNote('')
                      }}
                    >
                      {t('admin.sendBack')}
                    </button>
                    <button
                      class="btn btn-danger btn-sm"
                      disabled={busyId === p.id}
                      onClick={() => remove(p)}
                    >
                      {t('admin.deleteBanned')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </TableState>

      {sendBackTarget && (
        <Modal
          title={`${t('admin.sendBackTitle')} — ${sendBackTarget.title}`}
          onClose={() => setSendBackTarget(null)}
          footer={
            <>
              <button class="btn btn-ghost" onClick={() => setSendBackTarget(null)}>
                {t('editor.cancel')}
              </button>
              <button class="btn btn-primary" disabled={busyId === sendBackTarget.id} onClick={submitSendBack}>
                {busyId === sendBackTarget.id ? '…' : t('admin.sendBack')}
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
          <p class="muted">{t('admin.sendBackHint')}</p>
        </Modal>
      )}
    </div>
  )
}
