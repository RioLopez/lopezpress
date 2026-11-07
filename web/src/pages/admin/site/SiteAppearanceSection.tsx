import { useI18n } from '../../../core/i18n'
import type { SiteConfigFull } from '../../../core/types'

interface Props {
  cfg: SiteConfigFull
  set: <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) => void
}

// Six presets plus the colour picker. Presets are hex constants, not a
// separate theme registry: the server stores exactly what the browser applies.
const PRESETS = ['', '#b8492c', '#4f46e5', '#0f766e', '#7c3aed', '#1d4ed8', '#b45309']

// Appearance group (B4): accent colour, home layout and footer copy.
// The colour is applied live by SiteProvider (core/site.tsx), so the operator
// sees the change on the public site the moment the config reloads.
export function SiteAppearanceSection({ cfg, set }: Props) {
  const { t } = useI18n()
  return (
    <>
      <section class="settings-section">
        <h3>{t('site.appearance')}</h3>
        <div class="settings-field">
          <label>{t('site.themeColor')}</label>
          <div class="theme-swatches">
            {PRESETS.map((hex) => (
              <button
                key={hex || 'default'}
                type="button"
                class={`theme-swatch${cfg.theme_color === hex ? ' is-active' : ''}`}
                style={hex ? `--swatch:${hex}` : ''}
                title={hex || t('site.themeDefault')}
                onClick={() => set('theme_color', hex)}
              >
                {hex ? '' : t('site.themeDefault')}
              </button>
            ))}
            <input
              type="color"
              class="theme-color-input"
              aria-label={t('site.themeCustom')}
              value={cfg.theme_color || '#b8492c'}
              onInput={(e) => set('theme_color', (e.target as HTMLInputElement).value)}
            />
          </div>
          <p class="muted">{t('site.themeColorHint')}</p>
        </div>
        <div class="settings-field">
          <label>{t('site.homeLayout')}</label>
          <select
            value={cfg.home_layout}
            onChange={(e) => set('home_layout', (e.target as HTMLSelectElement).value as 'list' | 'cards')}
          >
            <option value="list">{t('site.layoutList')}</option>
            <option value="cards">{t('site.layoutCards')}</option>
          </select>
          <p class="muted">{t('site.homeLayoutHint')}</p>
        </div>
        <div class="settings-field">
          <label>{t('site.footer')}</label>
          <textarea value={cfg.footer_text} onInput={(e) => set('footer_text', (e.target as HTMLInputElement).value)} />
        </div>
      </section>
    </>
  )
}
