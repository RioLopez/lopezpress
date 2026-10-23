import { useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { registrationOpen, errMessage } from '../core/api'
import { useNav } from '../core/router'

export function RegisterPage() {
  const { register } = useAuth()
  const { pushToast } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPwd, setShowPwd] = useState(false)
  const [busy, setBusy] = useState(false)
  // Registration gate (spec 16): open registration is off by default for a
  // single-user install, so the form is hidden until the server confirms it.
  const [open, setOpen] = useState<boolean | null>(null)

  useEffect(() => {
    registrationOpen()
      .then((r) => setOpen(r.open))
      .catch(() => setOpen(false))
  }, [])

  const submit = async () => {
    if (!username.trim() || !email.trim() || !password) {
      pushToast('error', t('auth.empty'))
      return
    }
    if (password.length < 8) {
      pushToast('error', t('auth.passwordTooShort'))
      return
    }
    setBusy(true)
    try {
      await register(username.trim(), email.trim(), password)
      pushToast('success', t('auth.welcomeBack') + username.trim())
      navigate('/')
    } catch (e) {
      pushToast('error', errMessage(e, t('auth.registerFailed')))
    } finally {
      setBusy(false)
    }
  }

  if (open === null) {
    return (
      <div class="auth-card">
        <p class="muted">{t('common.loading')}</p>
      </div>
    )
  }

  if (!open) {
    return (
      <div class="auth-card">
        <h1>{t('auth.registerTitle')}</h1>
        <p class="muted register-closed">{t('register.closed')}</p>
        <p class="muted">{t('register.contactAdmin')}</p>
        <p class="muted link">
          {t('auth.hasAccount')}
          <a href="#" onClick={(e) => { e.preventDefault(); navigate('/login') }}>{t('auth.loginBtn')}</a>
        </p>
      </div>
    )
  }

  return (
    <div class="auth-card">
      <h1>{t('auth.registerTitle')}</h1>
      <p class="muted">{t('auth.registerSub')}</p>
      <div class="field">
        <label>{t('auth.username')}</label>
        <input value={username} onInput={(e) => setUsername((e.target as HTMLInputElement).value)} placeholder="bob" />
      </div>
      <div class="field">
        <label>{t('auth.email')}</label>
        <input type="email" value={email} onInput={(e) => setEmail((e.target as HTMLInputElement).value)} placeholder="bob@example.com" />
      </div>
      <div class="field">
        <label>{t('auth.password')}</label>
        <div class="pwd-wrap">
          <input
            type={showPwd ? 'text' : 'password'}
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            placeholder="6+"
            onKeyDown={(e) => e.key === 'Enter' && submit()}
          />
          <button
            type="button"
            class="pwd-toggle"
            onClick={() => setShowPwd((v) => !v)}
            title={showPwd ? 'Hide' : 'Show'}
          >
            {showPwd ? 'HIDE' : 'SHOW'}
          </button>
        </div>
      </div>
      <button class="btn btn-primary btn-block" disabled={busy} onClick={submit}>
        {busy ? '…' : t('auth.registering')}
      </button>
      <p class="muted link">
        {t('auth.hasAccount')}
        <a href="#" onClick={(e) => { e.preventDefault(); navigate('/login') }}>{t('auth.loginBtn')}</a>
      </p>
    </div>
  )
}
