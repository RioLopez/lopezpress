import { useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import {
  updateMe,
  uploadImage,
  listMyUploads,
  deleteMyUpload,
  getSiteConfig,
  requestEmailChange,
  confirmEmailChange,
  errMessage,
} from '../core/api'
import { fmtBytes, fmtDate } from '../core/format'
import { AvatarCropper } from '../ui/AvatarCropper'
import type { UploadPage } from '../core/types'

// "My images". Deliberately not called a library and deliberately
// scoped to the caller: the site is not an image host for its members. What an
// author gets is the ability to withdraw a picture they uploaded — a soft
// delete, because breaking an article that already embeds it would punish the
// wrong person.
function MyUploads() {
  const { t } = useI18n()
  const { pushToast } = useApp()
  const [page, setPage] = useState<UploadPage | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  const refresh = () => listMyUploads(1, 60).then(setPage).catch(() => setPage(null))

  useEffect(() => {
    refresh()
  }, [])

  const withdraw = (name: string) => {
    if (!confirm(t('myUploads.confirm'))) return
    setBusy(name)
    deleteMyUpload(name)
      .then(() => {
        pushToast('info', t('myUploads.deleted'))
        refresh()
      })
      .catch((e) => pushToast('error', errMessage(e, t('myUploads.deleteFailed'))))
      .finally(() => setBusy(null))
  }

  return (
    <section class="image-lib">
      <div class="image-lib-head">
        <div>
          <h2>{t('myUploads.title')}</h2>
          <p class="muted">{t('myUploads.sub')}</p>
        </div>
        {page && page.user_quota_bytes > 0 && (
          <span class="muted">
            {t('myUploads.used', {
              used: fmtBytes(page.user_used_bytes),
              quota: fmtBytes(page.user_quota_bytes),
            })}
          </span>
        )}
      </div>

      {page === null && <p class="muted">{t('myUploads.loading')}</p>}
      {page && page.items.length === 0 && <p class="muted">{t('myUploads.empty')}</p>}
      {page && page.items.length > 0 && (
        <div class="image-grid">
          {page.items.map((it) => (
            <div class="image-card" key={it.name}>
              <img src={it.thumb_url || it.url} alt={it.original_name || it.name} loading="lazy" />
              <div class="image-card-meta">
                <span>{fmtBytes(it.size)}</span>
                <button class="btn btn-ghost btn-sm" disabled={busy === it.name} onClick={() => withdraw(it.name)}>
                  {t('myUploads.delete')}
                </button>
              </div>
              <p class="image-card-sub">
                {fmtDate(it.created_at)}
                {it.ref_count > 0 && ` · ${t('myUploads.refNote', { n: String(it.ref_count) })}`}
              </p>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

// Profile editor. Replaces the old SM.MS-token settings page: the
// image-host token now lives server-side in site config.
export function SettingsPage() {
  const { user, refresh } = useAuth()
  const { t } = useI18n()
  const { pushToast } = useApp()
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [bio, setBio] = useState('')
  const [avatar, setAvatar] = useState('')
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)
  // The picked file, held only while the cropper is open.
  const [cropFile, setCropFile] = useState<File | null>(null)
 // (experimental): with verification on, the address is no longer a
  // field you edit — it moves through a code mailed to the NEW address, so the
  // form shows the current one and a "change" panel instead.
  const [needVerify, setNeedVerify] = useState(false)
  const [emailPanel, setEmailPanel] = useState(false)
  const [newEmail, setNewEmail] = useState('')
  const [emailCode, setEmailCode] = useState('')
  const [emailCd, setEmailCd] = useState(0)

  useEffect(() => {
    getSiteConfig()
      .then((c) => setNeedVerify(!!c.email_verify_enabled))
      .catch(() => setNeedVerify(false))
  }, [])

  useEffect(() => {
    if (emailCd <= 0) return
    const id = setTimeout(() => setEmailCd((n) => n - 1), 1000)
    return () => clearTimeout(id)
  }, [emailCd])

  useEffect(() => {
    if (user) {
      setUsername(user.username || '')
      setEmail(user.email || '')
      setDisplayName(user.display_name || user.username || '')
      setBio(user.bio || '')
      setAvatar(user.avatar || '')
    }
  }, [user])

  const save = async () => {
    if (!user) return
    if (!username.trim()) {
      pushToast('error', t('profile.usernameRequired'))
      return
    }
    if (!email.trim()) {
      pushToast('error', t('profile.emailRequired'))
      return
    }
    setSaving(true)
    try {
      // With verification on, the address is not part of this request: the
      // server rejects an unverified change outright, so sending it would turn
      // every profile save into a 403.
      await updateMe({
        username,
        ...(needVerify ? {} : { email }),
        display_name: displayName,
        bio,
        avatar,
      })
      await refresh()
      pushToast('success', t('profile.saved'))
    } catch (e) {
      pushToast('error', errMessage(e, t('profile.saveFailed')))
    } finally {
      setSaving(false)
    }
  }

 // The two halves of the verified email change. The code goes to the
  // NEW address, so receiving it is itself the proof of ownership.
  const sendEmailCode = async () => {
    if (!newEmail.trim()) {
      pushToast('error', t('auth.codeRequired'))
      return
    }
    try {
      await requestEmailChange({ email: newEmail.trim() })
      setEmailCd(60)
      pushToast('success', t('auth.codeSent'))
    } catch (e) {
      pushToast('error', errMessage(e, t('auth.codeSent')))
    }
  }

  const confirmEmail = async () => {
    if (!emailCode.trim()) {
      pushToast('error', t('auth.codeRequired'))
      return
    }
    try {
      const fresh = await confirmEmailChange({ email: newEmail.trim(), code: emailCode.trim() })
      setEmail(fresh.email || newEmail.trim())
      setEmailPanel(false)
      setNewEmail('')
      setEmailCode('')
      await refresh()
      pushToast('success', t('profile.emailChanged'))
    } catch (e) {
      pushToast('error', errMessage(e, t('profile.saveFailed')))
    }
  }

  // The cropper has already produced a 512×512 WebP; upload it as an avatar
  // (the server enforces the same contract and rejects anything else) and save
  // the profile right away, because "click avatar → pick → done" is the flow.
  const onCropped = async (blob: Blob) => {
    setCropFile(null)
    setUploading(true)
    try {
      const ext = blob.type === 'image/webp' ? 'webp' : 'jpg'
      const file = new File([blob], `avatar.${ext}`, { type: blob.type })
      const r = await uploadImage(file, 'avatar')
      if (r.url) {
        await updateMe({ avatar: r.url })
        setAvatar(r.url)
        await refresh()
        pushToast('success', t('imgUpload.uploaded'))
      }
    } catch (e) {
      pushToast('error', errMessage(e, t('imgUpload.failed')))
    } finally {
      setUploading(false)
    }
  }

  return (
    <div class="settings-page">
      <h1>{t('profile.title')}</h1>
      <div class="settings-field">
        <label>{t('profile.username')}</label>
        <input value={username} onInput={(e) => setUsername((e.target as HTMLInputElement).value)} />
        <p class="settings-hint">{t('profile.usernameHint')}</p>
      </div>
      <div class="settings-field">
        <label>{t('profile.email')}</label>
        {needVerify ? (
          <>
            <div class="email-current">
              <span>{email || '—'}</span>
              <button
                type="button"
                class="btn btn-ghost btn-sm"
                onClick={() => { setEmailPanel((v) => !v); setNewEmail(''); setEmailCode('') }}
              >
                {t('profile.changeEmail')}
              </button>
            </div>
            {emailPanel && (
              <div class="email-change-panel">
                <div class="field">
                  <label>{t('profile.newEmail')}</label>
                  <input
                    type="email"
                    value={newEmail}
                    onInput={(e) => setNewEmail((e.target as HTMLInputElement).value)}
                    placeholder="new@example.com"
                  />
                </div>
                <div class="field">
                  <label>{t('auth.codeLabel')}</label>
                  <div class="code-row">
                    <input
                      value={emailCode}
                      inputMode="numeric"
                      maxLength={6}
                      onInput={(e) => setEmailCode((e.target as HTMLInputElement).value)}
                      placeholder={t('auth.codePlaceholder')}
                    />
                    <button
                      type="button"
                      class="btn btn-ghost"
                      disabled={emailCd > 0 || !newEmail.trim()}
                      onClick={sendEmailCode}
                    >
                      {emailCd > 0 ? `${emailCd} ${t('auth.resendIn')}` : t('auth.sendCode')}
                    </button>
                  </div>
                </div>
                <button type="button" class="btn btn-primary btn-sm" onClick={confirmEmail}>
                  {t('profile.confirmEmail')}
                </button>
              </div>
            )}
            <p class="settings-hint">{t('profile.emailVerifyHint')}</p>
          </>
        ) : (
          <>
            <input type="email" value={email} onInput={(e) => setEmail((e.target as HTMLInputElement).value)} />
            <p class="settings-hint">{t('profile.emailHint')}</p>
          </>
        )}
      </div>
      <div class="settings-field">
        <label>{t('profile.displayName')}</label>
        <input value={displayName} onInput={(e) => setDisplayName((e.target as HTMLInputElement).value)} />
      </div>
      <div class="settings-field">
        <label>{t('profile.avatar')}</label>
        <div class="avatar-picker">
          <button
            class="avatar-picker-btn"
            type="button"
            disabled={uploading}
            onClick={() => {
              const input = document.getElementById('avatar-file') as HTMLInputElement | null
              input?.click()
            }}
            title={t('profile.avatarHint')}
          >
            {avatar ? (
              <img src={avatar} alt="" class="avatar-preview" />
            ) : (
              <span class="avatar-preview avatar-preview--empty">{t('imgUpload.uploadLocal')}</span>
            )}
            <span class="avatar-picker-badge">{uploading ? '…' : '✎'}</span>
          </button>
          <input
            id="avatar-file"
            type="file"
            accept="image/*"
            hidden
            disabled={uploading}
            onChange={(e) => {
              const input = e.target as HTMLInputElement
              const f = input.files?.[0]
              input.value = ''
              if (!f) return
              if (!f.type.startsWith('image/')) {
                pushToast('error', t('imgUpload.badFormat'))
                return
              }
              setCropFile(f)
            }}
          />
          <p class="settings-hint">{t('profile.avatarHint')}</p>
        </div>
      </div>
      <div class="settings-field">
        <label>{t('profile.bio')}</label>
        <textarea value={bio} onInput={(e) => setBio((e.target as HTMLTextAreaElement).value)} />
      </div>
      <div class="settings-actions">
        <button class="btn btn-primary" onClick={save} disabled={saving}>
          {saving ? '…' : t('profile.save')}
        </button>
      </div>

      <hr class="settings-divider" />
      <MyUploads />

      {cropFile && (
        <AvatarCropper file={cropFile} onCancel={() => setCropFile(null)} onDone={onCropped} />
      )}
    </div>
  )
}
