import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { getAdminSiteConfig, updateSiteConfig, uploadImage, sendTestMail, errMessage } from '../core/api'
import { useSite } from '../core/site'
import { useAuth } from '../core/auth'
import { isRoot } from '../core/roles'
import type { SiteConfigFull, FriendLink } from '../core/types'
import { SiteBasicsSection } from './admin/site/SiteBasicsSection'
import { SiteContentSection } from './admin/site/SiteContentSection'
import { SiteUsersSection } from './admin/site/SiteUsersSection'
import { SiteMediaSection } from './admin/site/SiteMediaSection'
import { SiteMailSection } from './admin/site/SiteMailSection'
import { SiteBackupSection } from './admin/site/SiteBackupSection'

// The form is the same object it always was (one GET, one PUT); only the
// presentation is split. Ten sections stacked in one column meant changing a
// switch at the bottom started with scrolling past everything above it.
type Group = 'basic' | 'content' | 'users' | 'media' | 'mail' | 'backup'

export function AdminSitePage() {
  const { t } = useI18n()
  const { user } = useAuth()
  const isRootUser = isRoot(user?.role)
  const { pushToast } = useApp()
  const { reload } = useSite()
  const [cfg, setCfg] = useState<SiteConfigFull | null>(null)
  const [links, setLinks] = useState<FriendLink[]>([])
  const [group, setGroup] = useState<Group>('basic')
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

  // Backup policy is root-only because the server ignores an admin's value;
  // showing the nav item would only lead them to a switch that does nothing.
  const groups: { key: Group; label: string }[] = [
    { key: 'basic', label: t('site.navBasic') },
    { key: 'content', label: t('site.navContent') },
    { key: 'users', label: t('site.navUsers') },
    { key: 'media', label: t('site.navMedia') },
    { key: 'mail', label: t('site.navMail') },
    ...(isRootUser ? [{ key: 'backup' as Group, label: t('site.navBackup') }] : []),
  ]

  return (
    <div class="admin-page">
      <h1>{t('site.title')}</h1>

      <div class="settings-layout">
        <nav class="settings-nav" aria-label={t('site.title')}>
          {groups.map((g) => (
            <button
              key={g.key}
              class={`settings-nav-item ${group === g.key ? 'is-active' : ''}`}
              onClick={() => setGroup(g.key)}
              aria-current={group === g.key ? 'true' : undefined}
            >
              {g.label}
            </button>
          ))}
        </nav>

        <div class="settings-body">
          {group === 'basic' && (
            <SiteBasicsSection
              cfg={cfg}
              set={set}
              uploading={uploading}
              onUpload={upload}
              links={links}
              onLinks={setLinks}
            />
          )}
          {group === 'content' && <SiteContentSection cfg={cfg} set={set} />}
          {group === 'users' && <SiteUsersSection cfg={cfg} set={set} isRootUser={isRootUser} />}
          {group === 'media' && <SiteMediaSection cfg={cfg} set={set} isRootUser={isRootUser} />}
          {group === 'mail' && (
            <SiteMailSection cfg={cfg} set={set} isRootUser={isRootUser} testing={testing} onTest={testMail} />
          )}
          {group === 'backup' && isRootUser && <SiteBackupSection cfg={cfg} set={set} />}
        </div>
      </div>

      <div class="settings-actions">
        <button class="btn btn-primary" onClick={save} disabled={busy}>
          {busy ? '…' : t('site.save')}
        </button>
      </div>
    </div>
  )
}
