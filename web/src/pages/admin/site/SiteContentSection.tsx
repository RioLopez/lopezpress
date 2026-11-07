import { useI18n } from '../../../core/i18n'
import type { SiteConfigFull } from '../../../core/types'

interface Props {
  cfg: SiteConfigFull
  set: <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) => void
}

export function SiteContentSection({ cfg, set }: Props) {
  const { t } = useI18n()

  return (
    <section class="settings-section">
      <h3>{t('site.display')}</h3>
      <div class="settings-field">
        <label>{t('site.pageSize')}</label>
        <select
          value={String(cfg.page_size || 10)}
          onChange={(e) => set('page_size', Number((e.target as HTMLSelectElement).value))}
        >
          {/* Three sizes only: the list is a reading surface, not a data table,
              and anything above 20 turns the public list into a scroll test. */}
          {[5, 10, 20].map((n) => (
            <option key={n} value={String(n)}>{n}</option>
          ))}
        </select>
        <p class="settings-hint">{t('site.pageSizeHint')}</p>
      </div>
      <label class="toggle">
        <input
          type="checkbox"
          checked={cfg.public_read}
          onInput={(e) => set('public_read', (e.target as HTMLInputElement).checked)}
        />
        {t('site.publicRead')}
      </label>
      <p class="settings-hint">{t('site.publicReadHint')}</p>
    </section>
  )
}
