import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { listActions, errMessage } from '../core/api'
import type { ActionLog } from '../core/types'

// Admin audit trail page: who did what on the site (content + admin ops).
// The action verb is rendered from a small i18n map with a raw-value fallback
// for any verb the labels don't cover yet.
export function AdminAuditPage() {
  const { t, locale } = useI18n()
  const { pushToast } = useApp()
  const [rows, setRows] = useState<ActionLog[] | null>(null)
  const [limit, setLimit] = useState(50)

  useEffect(() => {
    let cancelled = false
    listActions(limit)
      .then((d) => { if (!cancelled) setRows(d) })
      .catch((e) => { if (!cancelled) pushToast('error', errMessage(e, 'load failed')) })
    return () => { cancelled = true }
  }, [limit])

  const verb = (action: string) => {
    const key = `audit.verb.${action}`
    const probe = t(key)
    // t() returns the key string when the key is missing; fall back to raw.
    return probe === key ? action : probe
  }

  // Deliberately NOT the shared fmtDate: an unparsable timestamp echoes the
  // raw value instead of an em-dash, so an audit row never hides its source
  // data. Locale still follows the UI language like every other date cell.
  const fmt = (s: string) => {
    const d = new Date(s)
    return isNaN(d.getTime()) ? s : d.toLocaleString(locale)
  }

  return (
    <div class="admin-page">
      <h1>{t('audit.title')}</h1>
      <p class="muted">{t('audit.sub')}</p>

      <div class="audit-controls">
        <label class="muted">
          {t('audit.limit')}
          <select class="select" value={limit} onChange={(e) => setLimit(Number((e.currentTarget as HTMLSelectElement).value))}>
            <option value="20">20</option>
            <option value="50">50</option>
            <option value="100">100</option>
          </select>
        </label>
      </div>

      {rows === null && <p class="muted">{t('center.loading')}</p>}
      {rows && rows.length === 0 && <p class="muted">{t('audit.empty')}</p>}
      {rows && rows.length > 0 && (
        <div class="table-scroll">
          <table class="admin-table audit-table">
            <thead>
              <tr>
                <th>{t('audit.time')}</th>
                <th>{t('audit.actor')}</th>
                <th>{t('audit.action')}</th>
                <th>{t('audit.target')}</th>
                <th>{t('audit.detail')}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((a) => (
                <tr key={a.id}>
                  <td class="audit-time">{fmt(a.created_at)}</td>
                  <td>{a.actor_name || '—'}</td>
                  <td><span class="audit-verb">{verb(a.action)}</span></td>
                  <td class="audit-target">{a.target || '—'}</td>
                  <td class="audit-detail">{a.detail || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
