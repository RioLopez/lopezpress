import { useI18n } from '../../../core/i18n'
import type { SiteConfigFull } from '../../../core/types'

interface Props {
  cfg: SiteConfigFull
  set: <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) => void
  isRootUser: boolean
}

// mbValue/mbToBytes keep the storage inputs in megabytes while the API speaks
// bytes. One decimal is enough: nobody tunes a quota to the kilobyte.
function mbValue(bytes: number): string {
  if (!bytes || bytes <= 0) return ''
  return (bytes / 1048576).toFixed(1)
}

function mbToBytes(mb: string): number {
  const n = Number(mb)
  if (!isFinite(n) || n <= 0) return 0
  return Math.round(n * 1048576)
}

export function SiteMediaSection({ cfg, set, isRootUser }: Props) {
  const { t } = useI18n()

  return (
    <>
      <section class="settings-section">
        <h3>{t('site.imgHost')}</h3>
        <div class="settings-field">
          <label>{t('site.mode')}</label>
          <select
            value={cfg.img_host_mode}
            onChange={(e) => set('img_host_mode', (e.target as HTMLSelectElement).value as 'local' | 'smms')}
          >
            <option value="local">{t('site.local')}</option>
            <option value="smms">{t('site.smms')}</option>
          </select>
        </div>
        {cfg.img_host_mode === 'smms' && (
          <div class="settings-field">
            <label>{t('site.token')}</label>
            <input
              type="password"
              value={cfg.img_host_token}
              disabled={!isRootUser}
              placeholder={isRootUser ? t('site.tokenHint') : t('site.rootOnlyHint')}
              onInput={(e) => set('img_host_token', (e.target as HTMLInputElement).value)}
            />
            {/* The preferred location is the server's .env
                (SM_MS_TOKEN); this DB field is a legacy fallback that the
                server still honours for backwards compatibility. */}
            <p class="settings-hint">{t('site.tokenHint')}</p>
          </div>
        )}
      </section>

      {/* The disk-protection numbers. They live next to the image
          host setting because they answer the same question — how much image
          data this instance is willing to hold. A zero/blank value is treated
          by the server as "keep the stored one", so a half-filled form can
          never switch the limits off. */}
      <section class="settings-section">
        <h3>{t('site.storage')}</h3>
        <p class="settings-hint">{t('site.storageHint')}</p>
        <div class="settings-field">
          <label>{t('site.uploadMaxImage')}</label>
          <input
            type="number"
            min="0.1"
            step="0.5"
            value={mbValue(cfg.upload_max_image)}
            onInput={(e) => set('upload_max_image', mbToBytes((e.target as HTMLInputElement).value))}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.uploadMaxAvatar')}</label>
          <input
            type="number"
            min="0.1"
            step="0.1"
            value={mbValue(cfg.upload_max_avatar)}
            onInput={(e) => set('upload_max_avatar', mbToBytes((e.target as HTMLInputElement).value))}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.userQuota')}</label>
          <input
            type="number"
            min="1"
            step="5"
            value={mbValue(cfg.user_quota_bytes)}
            onInput={(e) => set('user_quota_bytes', mbToBytes((e.target as HTMLInputElement).value))}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.siteQuota')}</label>
          <input
            type="number"
            min="20"
            step="50"
            value={mbValue(cfg.site_quota_bytes)}
            onInput={(e) => set('site_quota_bytes', mbToBytes((e.target as HTMLInputElement).value))}
          />
        </div>
        <label class="toggle">
          <input
            type="checkbox"
            checked={cfg.allow_user_delete_upload}
            onInput={(e) => set('allow_user_delete_upload', (e.target as HTMLInputElement).checked)}
          />
          {t('site.allowUserDeleteUpload')}
        </label>
        <label class="toggle">
          <input
            type="checkbox"
            checked={cfg.allow_user_change_avatar}
            onInput={(e) => set('allow_user_change_avatar', (e.target as HTMLInputElement).checked)}
          />
          {t('site.allowUserChangeAvatar')}
        </label>
        <label class="toggle">
          <input
            type="checkbox"
            checked={cfg.allow_external_avatar}
            onInput={(e) => set('allow_external_avatar', (e.target as HTMLInputElement).checked)}
          />
          {t('site.allowExternalAvatar')}
        </label>
        <p class="settings-hint">{t('site.externalAvatarHint')}</p>
      </section>
    </>
  )
}
