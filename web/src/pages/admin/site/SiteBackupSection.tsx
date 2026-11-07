import { useI18n } from '../../../core/i18n'
import type { SiteConfigFull } from '../../../core/types'

interface Props {
  cfg: SiteConfigFull
  set: <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) => void
}

export function SiteBackupSection({ cfg, set }: Props) {
  const { t } = useI18n()

  return (
    <section class="settings-section">
      <h3>{t('admin.autoBackupTitle')}</h3>
      <label class="toggle">
        <input
          type="checkbox"
          checked={cfg.auto_backup}
          onInput={(e) => set('auto_backup', (e.target as HTMLInputElement).checked)}
        />
        {t('site.autoBackup')}
      </label>
      <p class="settings-hint">{t('site.autoBackupHint')}</p>
      <div class="settings-field">
        <label>{t('site.backupKeep')}</label>
        <select
          value={String(cfg.backup_keep || 7)}
          onChange={(e) => set('backup_keep', Number((e.target as HTMLSelectElement).value))}
        >
          {[3, 7, 14, 30, 90].map((n) => (
            <option key={n} value={String(n)}>{n}</option>
          ))}
        </select>
        <p class="settings-hint">{t('site.backupKeepHint')}</p>
      </div>
    </section>
  )
}
