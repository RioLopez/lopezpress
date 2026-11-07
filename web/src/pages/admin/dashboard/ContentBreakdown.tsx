import { useI18n } from '../../../core/i18n'
import type { StatusCounts } from '../../../core/types'

// Where the posts actually are. Bars are relative to the total, so "12 drafts
// and 3 published" reads at a glance instead of needing arithmetic.
// Banned is only present for the root — the vault is not an admin's business.
export function ContentBreakdown({ counts }: { counts: StatusCounts }) {
  const { t } = useI18n()
  const rows = [
    { key: 'published', n: counts.published, label: t('admin.statusPublished') },
    { key: 'pending', n: counts.pending, label: t('admin.statusPending') },
    { key: 'draft', n: counts.draft, label: t('admin.statusDraft') },
    { key: 'private', n: counts.private, label: t('admin.statusPrivate') },
  ]
  if (counts.banned !== undefined) {
    rows.push({ key: 'banned', n: counts.banned, label: t('admin.statusBanned') })
  }
  const total = rows.reduce((sum, r) => sum + r.n, 0)

  return (
    <section class="dash-block">
      <h3>{t('admin.contentTitle')}</h3>
      {total === 0 ? (
        <p class="muted">{t('admin.contentEmpty')}</p>
      ) : (
        <div class="dash-metrics">
          {rows.map((r) => (
            <div class="dash-metric" key={r.key}>
              <span class="dash-metric-label">{r.label}</span>
              <span class="dash-metric-track">
                <span
                  class={`dash-metric-fill dash-metric-fill--${r.key}`}
                  style={{ width: `${(r.n / total) * 100}%` }}
                />
              </span>
              <span class="dash-metric-num">{r.n}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
