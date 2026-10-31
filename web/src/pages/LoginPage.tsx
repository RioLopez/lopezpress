import { useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { ApiError, errMessage, errStatus, getCaptcha } from '../core/api'
import { useNav } from '../core/router'
import type { CaptchaChallenge } from '../core/types'
import { CaptchaWidget } from '../components/CaptchaWidget'

// How long a 429 keeps the retry button disabled. The server's own window is
// one minute, so anything shorter just walks into the same refusal again.
const THROTTLE_COOLDOWN = 60

export function LoginPage() {
  const { login } = useAuth()
  const { pushToast } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPwd, setShowPwd] = useState(false)
  const [busy, setBusy] = useState(false)

  // The challenge is fetched before the user submits, because the password is
  // only compared once the picture is solved. `needed` stays false when the
  // server has the feature switched off, so this form degrades to exactly the
  // old one.
  const [challenge, setChallenge] = useState<CaptchaChallenge | null>(null)
  const [needed, setNeeded] = useState(false)
  const [answer, setAnswer] = useState({ code: '' })
  // Why the picture is missing. A 429 is not a failure — it means this client
  // asked for more pictures than its per-minute budget allows, and the honest
  // answer is a countdown, not "failed to load" (which invited people to tap
  // retry until they spent the budget all over again).
  const [blocked, setBlocked] = useState<{ throttled: boolean; status?: number } | null>(null)
  const [retryIn, setRetryIn] = useState(0)

  const loadCaptcha = async () => {
    try {
      const r = await getCaptcha()
      setNeeded(r.enabled)
      setChallenge(r.captcha || null)
      setAnswer({ code: '' })
      setBlocked(null)
      setRetryIn(0)
    } catch (e) {
      // The fetch failed. Keep the form locked rather than silently dropping
      // the requirement: the server would reject the login anyway, and asking
      // again is the honest answer.
      setNeeded(true)
      setChallenge(null)
      const st = errStatus(e)
      setBlocked({ throttled: st === 429, status: st })
      setRetryIn(st === 429 ? THROTTLE_COOLDOWN : 0)
    }
  }

  useEffect(() => {
    void loadCaptcha()
  }, [])

  // One ticker for the cooldown, keyed on "are we cooling down" so it is not
  // torn down and rebuilt on every tick.
  const cooling = retryIn > 0
  useEffect(() => {
    if (!cooling) return
    const id = setInterval(() => setRetryIn((n) => (n <= 1 ? 0 : n - 1)), 1000)
    return () => clearInterval(id)
  }, [cooling])

  const solved = !needed || (challenge ? answer.code.trim().length > 0 : false)

  const submit = async () => {
    if (!email.trim() || !password) {
      pushToast('error', t('auth.empty'))
      return
    }
    if (!solved) {
      pushToast('error', t('auth.captchaRequired'))
      return
    }
    setBusy(true)
    try {
      const u = await login(email.trim(), password, {
        id: challenge?.id,
        code: answer.code.trim(),
      })
 // First-boot admin: the seeded random password forces a
      // change on the very first login.
      if (u.must_change_password) {
        navigate('/change-password')
        return
      }
      pushToast('success', `${t('auth.welcomeBack')}${u.username}`)
      navigate('/')
    } catch (e) {
      // A rejected challenge comes back as a 400 whose body already carries the
      // replacement, so the user retries without retyping their password.
      const payload = e instanceof ApiError ? e.payload : null
      if (payload && payload.captcha) {
        setChallenge(payload.captcha as CaptchaChallenge)
        setAnswer({ code: '' })
        pushToast('error', t('auth.captchaWrong'))
        return
      }
      // A 429 is the limiter, not a wrong password. Reporting it as "login
      // failed" hides the only useful information: wait, do not retry.
      if (errStatus(e) === 429) {
        setChallenge(null)
        setBlocked({ throttled: true, status: 429 })
        setRetryIn(THROTTLE_COOLDOWN)
        pushToast('error', t('common.tooManyRequests'))
        return
      }
      pushToast('error', errMessage(e, t('auth.loginFailed')))
      // The picture was spent by this attempt (single use, whether it passed or
      // not), so fetch a new one before the user tries again.
      if (needed) void loadCaptcha()
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
      {needed && challenge && (
        <CaptchaWidget
          challenge={challenge}
          disabled={busy}
          onSolved={setAnswer}
          onRefresh={() => void loadCaptcha()}
        />
      )}
      {needed && !challenge && (
        <div class="captcha-block">
          <p class="muted">
            {retryIn > 0
              ? t('auth.captchaThrottled', { s: String(retryIn) })
              : blocked?.throttled
                ? t('auth.captchaUnavailable')
                : blocked?.status
                  ? t('auth.captchaError', { status: String(blocked.status) })
                  : t('auth.captchaUnavailable')}
          </p>
          {/* While cooling down the button keeps counting, so a visitor never
              has to guess whether tapping again is allowed yet. */}
          <button type="button" class="captcha-refresh" onClick={() => void loadCaptcha()} disabled={cooling}>
            {cooling ? `${retryIn}s` : t('auth.captchaRefresh')}
          </button>
        </div>
      )}
      {/* Shown even when the site has email verification switched off: the
          reset page then explains that in one line, which is more useful than
          a link that silently disappears. */}
      <p class="muted link forgot-link">
        <a href="#" onClick={(e) => { e.preventDefault(); navigate('/forgot-password') }}>
          {t('auth.forgotPassword')}
        </a>
      </p>
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
