import { useI18n } from '../../../core/i18n'
import { fmtBytes, fmtDate } from '../../../core/format'
import type { DashboardData } from '../../../core/types'

// The facts that decide whether this box is healthy: which build is running,
// how much media it holds, who has accounts, and (for the root only) whether
// the daily snapshot still exists.
export function SystemStatus({ data }: { data: DashboardData }) {
  const { t, locale } = useI18n()
  const backup = data.auto_backup
  const rows = [
    { label: t('admin.sysVersion'), value: data.app_version || '—' },
    {
      label: t('admin.sysMedia'),
      value: `${data.media_live_count} · ${fmtBytes(data.media_live_bytes)}`,
    },
    { label: t('admin.sysUsers'), value: String(data.users_total) },
    { label: t('admin.sysUsersDisabled'), value: String(data.users_disabled) },
    { label: t('admin.sysUsersNew'), value: String(data.users_new_7d) },
  ]
  if (backup) {
    rows.push(
      { label: t('admin.autoBackupTitle'), value: backup.enabled ? t('admin.sysBackupOn') : t('admin.sysBackupOff') },
      {
        label: t('admin.autoBackupLast'),
        value: backup.last_at ? fmtDate(backup.last_at, locale) : t('admin.autoBackupNever'),
      },
      { label: t('admin.sysBackupFiles'), value: String(backup.files) },
    )
  }

  return (
    <section class="dash-block">
      <h3>{t('admin.systemTitle')}</h3>
      <div class="dash-facts">
        {rows.map((r) => (
          <div class="dash-fact" key={r.label}>
            <span class="dash-fact-label">{r.label}</span>
            <span class="dash-fact-value">{r.value}</span>
          </div>
        ))}
      </div>
    </section>
  )
}
