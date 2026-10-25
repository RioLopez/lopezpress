import { useEffect, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { getSiteConfig, requestEmailCode, resetPasswordWithCode, errMessage } from '../core/api'
import { useNav } from '../core/router'

// Self-service password reset (spec 37, experimental).
//
// Two steps on one page rather than two routes: the code is short-lived and
// the user is already looking at the form they typed their address into.
// Reusing `reset_password` as the code purpose means a register code cannot be
// spent here.
export function ForgotPasswordPage() {
  const { pushToast } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [enabled, setEnabled] = useState<boolean | null>(null)
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [cooldown, setCooldown] = useState(0)
  const [sending, setSending] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    getSiteConfig()
      .then((c) => setEnabled(!!c.email_verify_enabled))
      .catch(() => setEnabled(false))
  }, [])

  useEffect(() => {
    if (cooldown <= 0) return
    const id = setTimeout(() => setCooldown((n) => n - 1), 1000)
    return () => clearTimeout(id)
  }, [cooldown])

  const sendCode = async () => {
    if (!email.trim()) {
      pushToast('error', t('auth.codeRequired'))
      return
    }
    setSending(true)
    try {
      await requestEmailCode({ email: email.trim(), purpose: 'reset_password' })
      setCooldown(60)
      pushToast('success', t('auth.codeSent'))
    } catch (e) {
      pushToast('error', errMessage(e, t('auth.codeSent')))
    } finally {
      setSending(false)
    }
  }

  const submit = async () => {
    if (!email.trim() || !code.trim() || !password) {
      pushToast('error', t('auth.empty'))
      return
    }
    if (password.length < 8) {
      pushToast('error', t('auth.passwordTooShort'))
      return
    }
    setBusy(true)
    try {
      await resetPasswordWithCode({ email: email.trim(), code: code.trim(), password })
      pushToast('success', t('auth.resetDone'))
      navigate('/login')
    } catch (e) {
      pushToast('error', errMessage(e, t('auth.resetBtn')))
    } finally {
      setBusy(false)
    }
  }

  if (enabled === null) {
    return (
      <div class="auth-card">
        <p class="muted">{t('common.loading')}</p>
      </div>
    )
  }

  return (
    <div class="auth-card">
      <h1>{t('auth.resetTitle')}</h1>
      <p class="muted">{t('auth.resetSub')}</p>
      {!enabled ? (
        <p class="muted register-closed">{t('auth.resetDisabled')}</p>
      ) : (
        <>
          <div class="field">
            <label>{t('auth.email')}</label>
            <input
              type="email"
              value={email}
              onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
              placeholder="bob@example.com"
            />
          </div>
          <div class="field">
            <label>{t('auth.codeLabel')}</label>
            <div class="code-row">
              <input
                value={code}
                inputMode="numeric"
                maxLength={6}
                onInput={(e) => setCode((e.target as HTMLInputElement).value)}
                placeholder={t('auth.codePlaceholder')}
              />
              <button
                type="button"
                class="btn btn-ghost"
                disabled={sending || cooldown > 0}
                onClick={sendCode}
              >
                {cooldown > 0 ? `${cooldown} ${t('auth.resendIn')}` : sending ? '…' : t('auth.sendCode')}
              </button>
            </div>
          </div>
          <div class="field">
            <label>{t('auth.newPassword')}</label>
            <input
              type="password"
              value={password}
              onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
              onKeyDown={(e) => e.key === 'Enter' && submit()}
            />
          </div>
          <button class="btn btn-primary btn-block" disabled={busy} onClick={submit}>
            {busy ? '…' : t('auth.resetBtn')}
          </button>
        </>
      )}
      <p class="muted link">
        <a href="#" onClick={(e) => { e.preventDefault(); navigate('/login') }}>
          {t('auth.loginBtn')}
        </a>
      </p>
    </div>
  )
}
