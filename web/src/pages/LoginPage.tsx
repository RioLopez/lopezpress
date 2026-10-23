import { useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { errMessage } from '../core/api'
import { useNav } from '../core/router'

export function LoginPage() {
  const { login } = useAuth()
  const { pushToast } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPwd, setShowPwd] = useState(false)
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!email.trim() || !password) {
      pushToast('error', t('auth.empty'))
      return
    }
    setBusy(true)
    try {
      const u = await login(email.trim(), password)
      // First-boot admin (spec #2): the seeded random password forces a
      // change on the very first login.
      if (u.must_change_password) {
        navigate('/change-password')
        return
      }
      pushToast('success', `${t('auth.welcomeBack')}${u.username}`)
      navigate('/')
    } catch (e) {
      pushToast('error', errMessage(e, t('auth.loginFailed')))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div class="auth-card">
      <h1>{t('auth.loginTitle')}</h1>
      <p class="muted">{t('auth.loginSub')}</p>
      <div class="field">
        <label>{t('auth.username')}</label>
        <input value={email} onInput={(e) => setEmail((e.target as HTMLInputElement).value)} placeholder={t('auth.usernamePlaceholder')} />
      </div>
      <div class="field">
        <label>{t('auth.password')}</label>
        <div class="pwd-wrap">
          <input
            type={showPwd ? 'text' : 'password'}
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            placeholder={t('auth.passwordPlaceholder')}
            onKeyDown={(e) => e.key === 'Enter' && submit()}
          />
          <button
            type="button"
            class="pwd-toggle"
            onClick={() => setShowPwd((v) => !v)}
            title={showPwd ? t('auth.hidePwd') : t('auth.showPwd')}
          >
            {showPwd ? t('auth.hidePwd') : t('auth.showPwd')}
          </button>
        </div>
      </div>
      <button class="btn btn-primary btn-block" disabled={busy} onClick={submit}>
        {busy ? t('auth.loginBusy') : t('auth.loginBtn')}
      </button>
      <p class="muted link">
        {t('auth.noAccount')}
        <a href="#" onClick={(e) => { e.preventDefault(); navigate('/register') }}>{t('auth.registering')}</a>
      </p>
    </div>
  )
}
