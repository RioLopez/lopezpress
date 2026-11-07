import { useI18n } from '../../../core/i18n'
import { fmtDate } from '../../../core/format'
import { useNav } from '../../../core/router'
import type { ActionLog } from '../../../core/types'

// The last five audited writes. Read-only on purpose: the audit page owns the
// full trail (filters, actors); this is the "did anything happen while I was
// away?" glance.
export function RecentActions({ actions }: { actions: ActionLog[] }) {
  const { t, locale } = useI18n()
  const [, navigate] = useNav()

  return (
    <section class="dash-block">
      <div class="dash-block-head">
        <h3>{t('admin.actionsTitle')}</h3>
        <button class="btn btn-ghost btn-sm" onClick={() => navigate('/admin/audit')}>
          {t('admin.actionsMore')}
        </button>
      </div>
      {actions.length === 0 ? (
        <p class="muted">{t('admin.actionsEmpty')}</p>
      ) : (
        <ul class="dash-list">
          {actions.map((a) => (
            <li key={a.id} class="dash-list-item">
              <span class="dash-list-title">
                {a.actor_name || '—'} · {a.action}
              </span>
              <span class="dash-list-date">{fmtDate(a.created_at, locale)}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
