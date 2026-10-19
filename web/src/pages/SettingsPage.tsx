import { useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { updateMe, uploadImage, listUploads, deleteUpload, ApiError } from '../core/api'
import type { LocalUpload } from '../core/types'

const fmtBytes = (n: number) =>
  n < 1024 ? `${n} B` : n < 1048576 ? `${(n / 1024).toFixed(1)} KB` : `${(n / 1048576).toFixed(1)} MB`

// Image library (browse + delete locally-stored uploads).
function ImageLibrary() {
  const { t } = useI18n()
  const { pushToast } = useApp()
  const [items, setItems] = useState<LocalUpload[] | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)

  const refresh = () => listUploads().then(setItems).catch(() => setItems([]))

  useEffect(() => {
    refresh()
  }, [])

  const onUpload = async (file: File) => {
    setUploading(true)
    try {
      await uploadImage(file)
      await refresh()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('imgUpload.failed'))
    } finally {
      setUploading(false)
    }
  }

  const doDelete = (name: string) => {
    if (!confirm(t('imageLib.confirm'))) return
    setBusy(name)
    deleteUpload(name)
      .then(() => {
        pushToast('info', t('imageLib.deleted'))
        refresh()
      })
      .catch((e) => pushToast('error', e instanceof ApiError ? e.message : t('imageLib.deleteFailed')))
      .finally(() => setBusy(null))
  }

  return (
    <section class="image-lib">
      <div class="image-lib-head">
        <div>
          <h1>{t('imageLib.title')}</h1>
          <p class="muted">{t('imageLib.sub')}</p>
        </div>
        <label class="btn btn-primary btn-sm">
          {uploading ? t('common.loading') : t('imageLib.upload')}
          <input
            type="file"
            accept="image/*"
            hidden
            disabled={uploading}
            onChange={(e) => {
              const f = (e.target as HTMLInputElement).files?.[0]
              if (f) onUpload(f)
            }}
          />
        </label>
      </div>

      {items === null && <p class="muted">{t('imageLib.loading')}</p>}
      {items && items.length === 0 && <p class="muted">{t('imageLib.empty')}</p>}
      {items && items.length > 0 && (
        <div class="image-grid">
          {items.map((it) => (
            <div class="image-card" key={it.name}>
              <img src={it.url} alt={it.name} loading="lazy" />
              <div class="image-card-meta">
                <span>{fmtBytes(it.size)}</span>
                <button class="btn btn-ghost btn-sm" onClick={() => doDelete(it.name)} disabled={busy === it.name}>
                  {t('imageLib.delete')}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

// Profile editor (spec 15). Replaces the old SM.MS-token settings page: the
// image-host token now lives server-side in site config (spec 05).
export function SettingsPage() {
  const { user, refresh } = useAuth()
  const { t } = useI18n()
  const { pushToast } = useApp()
  const [displayName, setDisplayName] = useState('')
  const [bio, setBio] = useState('')
  const [avatar, setAvatar] = useState('')
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)

  useEffect(() => {
    if (user) {
      setDisplayName(user.display_name || user.username || '')
      setBio(user.bio || '')
      setAvatar(user.avatar || '')
    }
  }, [user])

  const save = async () => {
    if (!user) return
    setSaving(true)
    try {
      await updateMe({ display_name: displayName, bio, avatar })
      await refresh()
      pushToast('success', t('profile.saved'))
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('profile.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  const uploadAvatar = async (file: File) => {
    setUploading(true)
    try {
      const r = await uploadImage(file)
      if (r.url) setAvatar(r.url)
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('imgUpload.failed'))
    } finally {
      setUploading(false)
    }
  }

  return (
    <div class="settings-page">
      <h1>{t('profile.title')}</h1>
      <div class="settings-field">
        <label>{t('profile.displayName')}</label>
        <input value={displayName} onInput={(e) => setDisplayName((e.target as HTMLInputElement).value)} />
      </div>
      <div class="settings-field">
        <label>{t('profile.avatar')}</label>
        {avatar && <img class="settings-preview settings-preview--sm" src={avatar} alt="" />}
        <div class="settings-row">
          <input value={avatar} placeholder="URL" onInput={(e) => setAvatar((e.target as HTMLInputElement).value)} />
          <label class="btn btn-ghost btn-sm">
            {uploading ? t('common.loading') : t('imgUpload.uploadLocal')}
            <input
              type="file"
              accept="image/*"
              hidden
              disabled={uploading}
              onChange={(e) => {
                const f = (e.target as HTMLInputElement).files?.[0]
                if (f) uploadAvatar(f)
              }}
            />
          </label>
        </div>
        <p class="settings-hint">{t('profile.avatarHint')}</p>
      </div>
      <div class="settings-field">
        <label>{t('profile.bio')}</label>
        <textarea value={bio} onInput={(e) => setBio((e.target as HTMLInputElement).value)} />
      </div>
      <div class="settings-actions">
        <button class="btn btn-primary" onClick={save} disabled={saving}>
          {saving ? '…' : t('profile.save')}
        </button>
      </div>

      <hr class="settings-divider" />
      <ImageLibrary />
    </div>
  )
}
