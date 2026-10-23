import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { getAdminSiteConfig, updateSiteConfig, uploadImage, sendTestMail, errMessage } from '../core/api'
import { useSite } from '../core/site'
import { useAuth } from '../core/auth'
import type { SiteConfigFull, FriendLink } from '../core/types'

export function AdminSitePage() {
  const { t } = useI18n()
  const { user } = useAuth()
  const isRoot = user?.role === 'root'
  const { pushToast } = useApp()
  const { reload } = useSite()
  const [cfg, setCfg] = useState<SiteConfigFull | null>(null)
  const [links, setLinks] = useState<FriendLink[]>([])
  const [busy, setBusy] = useState(false)
  const [uploading, setUploading] = useState<'logo' | 'favicon' | null>(null)
  const [testing, setTesting] = useState(false)

  const testMail = async () => {
    setTesting(true)
    try {
      await sendTestMail()
      pushToast('success', t('site.mailTestSent'))
    } catch (e) {
      pushToast('error', errMessage(e, t('site.mailTestFailed')))
    } finally {
      setTesting(false)
    }
  }

  useEffect(() => {
    getAdminSiteConfig()
      .then((c) => {
        setCfg(c)
        try {
          setLinks(c.friend_links ? (JSON.parse(c.friend_links) as FriendLink[]) : [])
        } catch {
          setLinks([])
        }
      })
      .catch((e) => pushToast('error', errMessage(e, t('admin.failed'))))
  }, [pushToast, t])

  if (!cfg) {
    return (
      <div class="admin-page">
        <p class="muted">{t('common.loading')}</p>
      </div>
    )
  }

  const set = <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) =>
    setCfg({ ...cfg, [k]: v })

  const upload = async (file: File, target: 'logo' | 'favicon') => {
    setUploading(target)
    try {
      const r = await uploadImage(file)
      if (r.url) set(target === 'logo' ? 'site_logo' : 'site_favicon', r.url)
    } catch (e) {
      pushToast('error', errMessage(e, t('imgUpload.failed')))
    } finally {
      setUploading(null)
    }
  }

  const save = async () => {
    setBusy(true)
    try {
      const body = { ...cfg, friend_links: JSON.stringify(links) }
      await updateSiteConfig(body)
      reload()
      pushToast('success', t('site.saved'))
    } catch (e) {
      pushToast('error', errMessage(e, t('site.saveFailed')))
    } finally {
      setBusy(false)
    }
  }

  const addLink = () => setLinks([...links, { name: '', url: '' }])
  const updateLink = (i: number, k: keyof FriendLink, v: string) =>
    setLinks(links.map((l, idx) => (idx === i ? { ...l, [k]: v } : l)))
  const removeLink = (i: number) => setLinks(links.filter((_, idx) => idx !== i))

  return (
    <div class="admin-page">
      <h1>{t('site.title')}</h1>

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
                  if (f) upload(f, 'logo')
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
                  if (f) upload(f, 'favicon')
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
            <button class="btn btn-danger btn-sm" onClick={() => removeLink(i)}>
              ×
            </button>
          </div>
        ))}
        <button class="btn btn-primary btn-sm" onClick={addLink}>
          {t('site.addLink')}
        </button>
      </section>

      <section class="settings-section">
        <h3>{t('site.imgHost')}</h3>
        <div class="settings-field">
          <label>{t('site.mode')}</label>
          <select value={cfg.img_host_mode} onChange={(e) => set('img_host_mode', (e.target as HTMLSelectElement).value as 'local' | 'smms')}>
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
              placeholder={t('site.tokenHint')}
              onInput={(e) => set('img_host_token', (e.target as HTMLInputElement).value)}
            />
            <p class="settings-hint">
              {t('site.tokenHint')}
              {/* Spec #7: the preferred location is the server's .env
                  (SM_MS_TOKEN); this DB field is a legacy fallback that the
                  server still honours for backwards compatibility. */}
            </p>
          </div>
        )}
      </section>

      <section class="settings-section">
        <h3>{t('site.display')}</h3>
        <div class="settings-field">
          <label>{t('site.pageSize')}</label>
          <select
            value={String(cfg.page_size || 10)}
            onChange={(e) => set('page_size', Number((e.target as HTMLSelectElement).value))}
          >
            {[5, 10, 20, 30].map((n) => (
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

      {/* Spec 34: root-only backup policy. Hidden from plain admins because the
          server ignores their value anyway — showing a switch that does not
          work is worse than not showing it. */}
      {isRoot && (
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
      )}

      <section class="settings-section">
        <h3>{t('site.register')}</h3>
        <label class="toggle">
          <input
            type="checkbox"
            checked={cfg.allow_register}
            onInput={(e) => set('allow_register', (e.target as HTMLInputElement).checked)}
          />
          {t('site.registerOpen')}
        </label>
      </section>

      <section class="settings-section">
        <h3>{t('site.mail')}</h3>
        <p class="settings-hint">{t('site.mailDisabledHint')}</p>
        <div class="settings-field">
          <label>{t('site.mailHost')}</label>
          <input
            value={cfg.smtp_host}
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
            placeholder="587"
            onInput={(e) => set('smtp_port', Number((e.target as HTMLInputElement).value) || 0)}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.mailUser')}</label>
          <input
            value={cfg.smtp_user}
            placeholder="you@example.com"
            onInput={(e) => set('smtp_user', (e.target as HTMLInputElement).value)}
          />
        </div>
        <div class="settings-field">
          <label>{t('site.mailPass')}</label>
          <input
            type="password"
            value={cfg.smtp_pass}
            placeholder="••••••••"
            onInput={(e) => set('smtp_pass', (e.target as HTMLInputElement).value)}
          />
        </div>
        <div class="settings-row">
          <button class="btn btn-ghost btn-sm" onClick={testMail} disabled={testing || !cfg.smtp_host}>
            {testing ? t('common.loading') : t('site.mailTest')}
          </button>
          <span class="muted">{t('site.mailEnabled')}</span>
        </div>
      </section>

      <div class="settings-actions">
        <button class="btn btn-primary" onClick={save} disabled={busy}>
          {busy ? '…' : t('site.save')}
        </button>
      </div>
    </div>
  )
}
