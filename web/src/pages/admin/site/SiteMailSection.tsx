import { useI18n } from '../../../core/i18n'
import type { SiteConfigFull } from '../../../core/types'

interface Props {
  cfg: SiteConfigFull
  set: <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) => void
  isRootUser: boolean
  testing: boolean
  onTest: () => void
}

export function SiteMailSection({ cfg, set, isRootUser, testing, onTest }: Props) {
  const { t } = useI18n()

  return (
    <>
      {/* Email verification, labelled as experimental in the UI
          because that is what it is — it needs working SMTP, and switching it
          on changes what the sign-up form asks for. Root-only, the same tier
          as allow_register and the SMTP credentials it depends on. */}
      <section class="settings-section">
        <h3>
          {t('site.emailVerify')} <span class="tag tag-warn">{t('auth.testFeature')}</span>
        </h3>
        <p class="settings-hint">{t('site.emailVerifyHint')}</p>
        <label class="toggle">
          <input
            type="checkbox"
            checked={cfg.email_verify_enabled}
            disabled={!isRootUser}
            onInput={(e) => set('email_verify_enabled', (e.target as HTMLInputElement).checked)}
          />
          {t('site.emailVerifyEnable')}
        </label>
        <div class="settings-field">
          <label>{t('site.codeTtl')}</label>
          <input
            type="number"
            min="2"
            max="60"
            value={cfg.email_code_ttl_min}
            disabled={!isRootUser}
            onInput={(e) => set('email_code_ttl_min', Number((e.target as HTMLInputElement).value))}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.codeCooldown')}</label>
          <input
            type="number"
            min="30"
            max="600"
            value={cfg.email_code_cooldown}
            disabled={!isRootUser}
            onInput={(e) => set('email_code_cooldown', Number((e.target as HTMLInputElement).value))}
          />
        </div>
        {!isRootUser && <p class="settings-hint">{t('site.rootOnlyHint')}</p>}
      </section>

      <section class="settings-section">
        <h3>{t('site.mail')}</h3>
        <p class="settings-hint">{t('site.mailDisabledHint')}</p>
        {/* Mail credentials are root-only on the server: the SMTP password is
            a secret (and is routinely the same string as the root login), so
            the field is never sent to a plain admin and their edits are
            ignored. Disabled inputs say so plainly instead of pretending. */}
        {!isRootUser && <p class="settings-hint">{t('site.rootOnlyHint')}</p>}
        <div class="settings-field">
          <label>{t('site.mailHost')}</label>
          <input
            value={cfg.smtp_host}
            disabled={!isRootUser}
            placeholder="smtp.example.com"
            onInput={(e) => set('smtp_host', (e.target as HTMLInputElement).value)}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.mailPort')}</label>
          <input
            type="number"
            min={1}
            max={65535}
            value={cfg.smtp_port || ''}
            disabled={!isRootUser}
            placeholder="587"
            onInput={(e) => set('smtp_port', Number((e.target as HTMLInputElement).value) || 0)}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.mailUser')}</label>
          <input
            value={cfg.smtp_user}
            disabled={!isRootUser}
            placeholder="you@example.com"
            onInput={(e) => set('smtp_user', (e.target as HTMLInputElement).value)}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.mailPass')}</label>
          <input
            type="password"
            value={cfg.smtp_pass}
            disabled={!isRootUser}
            placeholder="••••••••"
            onInput={(e) => set('smtp_pass', (e.target as HTMLInputElement).value)}
          />
        </div>
        <div class="settings-row">
          <button class="btn btn-ghost btn-sm" onClick={onTest} disabled={testing || !cfg.smtp_host}>
            {testing ? t('common.loading') : t('site.mailTest')}
          </button>
          <span class="muted">{t('site.mailEnabled')}</span>
        </div>
      </section>
    </>
  )
}
