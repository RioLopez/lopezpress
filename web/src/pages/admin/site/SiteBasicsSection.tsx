import { useI18n } from '../../../core/i18n'
import type { FriendLink, SiteConfigFull } from '../../../core/types'

interface Props {
  cfg: SiteConfigFull
  set: <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) => void
  uploading: 'logo' | 'favicon' | null
  onUpload: (file: File, target: 'logo' | 'favicon') => void
  links: FriendLink[]
  onLinks: (links: FriendLink[]) => void
}

export function SiteBasicsSection({ cfg, set, uploading, onUpload, links, onLinks }: Props) {
  const { t } = useI18n()

  const updateLink = (i: number, k: keyof FriendLink, v: string) =>
    onLinks(links.map((l, idx) => (idx === i ? { ...l, [k]: v } : l)))

  return (
    <>
      <section class="settings-section">
        <h3>{t('site.basic')}</h3>
        <div class="settings-field">
          <label>{t('site.siteName')}</label>
          <input value={cfg.site_name} onInput={(e) => set('site_name', (e.target as HTMLInputElement).value)} />
        </div>
        <div class="settings-field">
          <label>{t('site.siteTitle')}</label>
          <input value={cfg.site_title} onInput={(e) => set('site_title', (e.target as HTMLInputElement).value)} />
        </div>
        <div class="settings-field">
          <label>{t('site.logo')}</label>
          {cfg.site_logo && <img class="settings-preview" src={cfg.site_logo} alt="" />}
          <div class="settings-row">
            <input
              value={cfg.site_logo}
              placeholder="URL"
              onInput={(e) => set('site_logo', (e.target as HTMLInputElement).value)}
            />
            <label class="btn btn-ghost btn-sm">
              {uploading === 'logo' ? t('common.loading') : t('imgUpload.uploadLocal')}
              <input
                type="file"
                accept="image/*"
                hidden
                disabled={uploading !== null}
                onChange={(e) => {
                  const f = (e.target as HTMLInputElement).files?.[0]
                  if (f) onUpload(f, 'logo')
                }}
              />
            </label>
          </div>
        </div>
        <div class="settings-field">
          <label>{t('site.favicon')}</label>
          {cfg.site_favicon && <img class="settings-preview settings-preview--sm" src={cfg.site_favicon} alt="" />}
          <div class="settings-row">
            <input
              value={cfg.site_favicon}
              placeholder="URL"
              onInput={(e) => set('site_favicon', (e.target as HTMLInputElement).value)}
            />
            <label class="btn btn-ghost btn-sm">
              {uploading === 'favicon' ? t('common.loading') : t('imgUpload.uploadLocal')}
              <input
                type="file"
                accept="image/*"
                hidden
                disabled={uploading !== null}
                onChange={(e) => {
                  const f = (e.target as HTMLInputElement).files?.[0]
                  if (f) onUpload(f, 'favicon')
                }}
              />
            </label>
          </div>
        </div>
        <div class="settings-field">
          <label>{t('site.siteUrl')}</label>
          <input
            value={cfg.site_url}
            placeholder="https://example.com"
            onInput={(e) => set('site_url', (e.target as HTMLInputElement).value)}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.siteDescription')}</label>
          <input
            value={cfg.site_description}
            maxLength={512}
            onInput={(e) => set('site_description', (e.target as HTMLInputElement).value)}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.footer')}</label>
          <textarea value={cfg.footer_text} onInput={(e) => set('footer_text', (e.target as HTMLInputElement).value)} />
        </div>
        <div class="settings-field">
          <label>{t('site.icp')}</label>
          <input value={cfg.icp_no} onInput={(e) => set('icp_no', (e.target as HTMLInputElement).value)} />
        </div>
      </section>

      <section class="settings-section">
        <h3>{t('site.friendLinks')}</h3>
        {links.map((l, i) => (
          <div class="friend-link-row" key={i}>
            <input
              placeholder={t('site.friendName')}
              value={l.name}
              onInput={(e) => updateLink(i, 'name', (e.target as HTMLInputElement).value)}
            />
            <input
              placeholder={t('site.friendUrl')}
              value={l.url}
              onInput={(e) => updateLink(i, 'url', (e.target as HTMLInputElement).value)}
            />
            <button class="btn btn-danger btn-sm" onClick={() => onLinks(links.filter((_, idx) => idx !== i))}>
              ×
            </button>
          </div>
        ))}
        <button class="btn btn-primary btn-sm" onClick={() => onLinks([...links, { name: '', url: '' }])}>
          {t('site.addLink')}
        </button>
      </section>
    </>
  )
}
