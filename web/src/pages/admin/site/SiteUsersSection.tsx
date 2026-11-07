import { useI18n } from '../../../core/i18n'
import type { SiteConfigFull } from '../../../core/types'

interface Props {
  cfg: SiteConfigFull
  set: <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) => void
  isRootUser: boolean
}

export function SiteUsersSection({ cfg, set, isRootUser }: Props) {
  const { t } = useI18n()

  return (
    <>
      <section class="settings-section">
        <h3>{t('site.register')}</h3>
        <label class="toggle">
          <input
            type="checkbox"
            checked={cfg.allow_register}
            disabled={!isRootUser}
            onInput={(e) => set('allow_register', (e.target as HTMLInputElement).checked)}
          />
          {t('site.registerOpen')}
        </label>
        <p class="settings-hint">{t('site.registerOpenHint')}</p>
        {/* The registration gate is a site-wide admission policy: the server
            ignores an admin's value, so the control is disabled rather than
            offering a switch that silently does nothing. */}
        {!isRootUser && <p class="settings-hint">{t('site.rootOnlyHint')}</p>}
      </section>

      <section class="settings-section">
        <h3>{t('site.permissions')}</h3>
        <p class="settings-hint">{t('site.permissionsHint')}</p>
        <table class="perm-matrix">
          <thead>
            <tr>
              <th>{t('site.permColRole')}</th>
              <th>{t('site.permRoot')}</th>
              <th>{t('site.permAdmin')}</th>
              <th>{t('site.permUser')}</th>
            </tr>
          </thead>
          <tbody>
            <tr><td>{t('site.permRowManageUsers')}</td><td>{t('site.permYes')}</td><td>{t('site.permOwnOnly')}</td><td>{t('site.permNo')}</td></tr>
            <tr><td>{t('site.permRowManageContent')}</td><td>{t('site.permYes')}</td><td>{t('site.permYes')}</td><td>{t('site.permNo')}</td></tr>
            <tr><td>{t('site.permRowReview')}</td><td>{t('site.permYes')}</td><td>{t('site.permYes')}</td><td>{t('site.permNo')}</td></tr>
            <tr><td>{t('site.permRowBan')}</td><td>{t('site.permYes')}</td><td>{t('site.permOwnOnly')}</td><td>{t('site.permNo')}</td></tr>
            <tr><td>{t('site.permRowBackup')}</td><td>{t('site.permYes')}</td><td>{t('site.permNo')}</td><td>{t('site.permNo')}</td></tr>
            <tr><td>{t('site.permRowSiteConfig')}</td><td>{t('site.permYes')}</td><td>{t('site.permYes')}</td><td>{t('site.permNo')}</td></tr>
            <tr><td>{t('site.permRowAuthorPost')}</td><td>{t('site.permYes')}</td><td>{t('site.permYes')}</td><td>{t('site.permYes')}</td></tr>
            <tr><td>{t('site.permRowSelfEdit')}</td><td>{t('site.permYes')}</td><td>{t('site.permYes')}</td><td>{t('site.permYes')}</td></tr>
          </tbody>
        </table>
      </section>
    </>
  )
}
