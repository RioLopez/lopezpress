import { useI18n } from '../../../core/i18n'
import { fmtBytes } from '../../../core/format'
import type { DashboardData } from '../../../core/types'

// Six numbers an operator checks in the morning: how much content exists, how
// much of it was read (ever and today), who is waiting for review, who has an
// account, and how much of the disk the media library occupies.
export function StatCards({ data }: { data: DashboardData }) {
  const { t } = useI18n()
  const cards = [
    { num: String(data.posts_total), label: t('admin.statPosts') },
    { num: String(data.views_total), label: t('admin.statViews') },
    { num: String(data.pending_total), label: t('admin.statPending') },
    { num: String(data.views_today), label: t('admin.statViewsToday') },
    { num: String(data.users_total), label: t('admin.statUsers') },
    { num: fmtBytes(data.media_live_bytes), label: t('admin.statStorage') },
  ]
  return (
    <div class="dash-cards">
      {cards.map((c) => (
        <div class="dash-card" key={c.label}>
          <div class="dash-card-num">{c.num}</div>
          <div class="dash-card-label">{c.label}</div>
        </div>
      ))}
    </div>
  )
}
