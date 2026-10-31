import { useEffect, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listRevisions, restoreRevision, errMessage } from '../core/api'
import type { Revision } from '../core/types'

// Collapsed history panel on the edit page. Shows the pre-edit
// snapshots newest-first; restoring one navigates to the read view so the user
// sees exactly what came back (the editor already holds unsaved state).
export function RevisionList({ postId, onRestored }: { postId: string; onRestored: () => void }) {
  const { t, locale } = useI18n()
  const { pushToast, confirmDialog } = useApp()
  const [open, setOpen] = useState(false)
  const [revs, setRevs] = useState<Revision[]>([])
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    listRevisions(postId).then(setRevs).catch(() => {})
  }, [open, postId])

  const restore = async (rev: Revision) => {
    if (!(await confirmDialog(t('rev.confirm')))) return
    setBusy(true)
    try {
      await restoreRevision(postId, rev.id)
      pushToast('success', t('rev.restored'))
      onRestored()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusy(false)
    }
  }

  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'

  return (
    <div class="rev-panel">
      <button class="btn btn-ghost btn-sm" onClick={() => setOpen((o) => !o)}>
        {open ? '▾' : '▸'} {t('rev.title')}
      </button>
      {open && (
        revs.length === 0 ? (
          <p class="muted">{t('rev.empty')}</p>
        ) : (
          <ul class="rev-list">
            {revs.map((r) => (
              <li key={r.id} class="rev-item">
                <div class="rev-meta">
                  <strong>{r.title || '—'}</strong>
                  <span class="muted">
                    {' '}· {new Date(r.created_at).toLocaleString(localeTag)}
                  </span>
                  <span class="muted"> · {(r.body || '').length} {t('rev.chars')}</span>
                </div>
                <button class="btn btn-ghost btn-sm" disabled={busy} onClick={() => restore(r)}>
                  {t('rev.restore')}
                </button>
              </li>
            ))}
          </ul>
        )
      )}
    </div>
  )
}
