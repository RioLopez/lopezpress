// ChangePasswordPage: forced first-login password change for the first-boot
// admin. Rendered at /change-password. The user can only leave this
// screen by successfully changing their password; they are not allowed to
// navigate away or cancel.

import { useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { passwordProblem } from '../core/password'
import { changePassword, errMessage } from '../core/api'
import { useNav } from '../core/router'

export function ChangePasswordPage() {
  const { user, loading, adoptToken } = useAuth()
  const { pushToast } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)

  // `hasToken` (the route guard) reads localStorage, so it lets this page
  // render before /api/auth/me has answered. Deciding anything about the flag
  // during that round-trip showed the "nothing to change" verdict first and
  // the real form a moment later; when /me used to omit the flag, the wrong
  // verdict was also the final one and the user got stuck behind 403s.
  if (loading || !user) return null

  // If the user is no longer flagged for a forced change, let them through.
  if (!user.must_change_password) {
    return (
      <div class="auth-card">
        <h1>{t('auth.welcomeBack')}{user.username}</h1>
        <p class="muted">{t('password.alreadySet')}</p>
        <button class="btn btn-primary btn-block" onClick={() => navigate('/')}>
          {t('password.continue')}
        </button>
      </div>
    )
  }

  const submit = async () => {
    if (!current || !next || !confirm) {
      pushToast('error', t('password.fillAll'))
      return
    }
    if (next !== confirm) {
      pushToast('error', t('password.mismatch'))
      return
    }
    const problem = passwordProblem(next)
    if (problem) {
      pushToast('error', t(problem === 'long' ? 'password.tooLong' : 'password.tooShort'))
      return
    }
    setBusy(true)
    try {
      const res = await changePassword({ current_password: current, new_password: next })
      // The change revoked the old token; adopt the fresh one so this
      // session stays valid (and the must_change flag clears in `user`).
      await adoptToken(res.token)
      pushToast('success', t('password.changed'))
      navigate('/')
    } catch (e) {
      pushToast('error', errMessage(e, t('password.changeFailed')))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div class="auth-card">
      <h1>{t('password.title')}</h1>
      <p class="muted">{t('password.subtitle')}</p>
      <div class="field">
        <label>{t('auth.password')}</label>
        <input
          type="password"
          value={current}
          onInput={(e) => setCurrent((e.target as HTMLInputElement).value)}
          placeholder={t('password.currentPlaceholder')}
        />
      </div>
      <div class="field">
        <label>{t('password.newLabel')}</label>
        <input
          type="password"
          value={next}
          onInput={(e) => setNext((e.target as HTMLInputElement).value)}
          placeholder={t('password.newPlaceholder')}
        />
      </div>
      <div class="field">
        <label>{t('password.confirmLabel')}</label>
        <input
          type="password"
          value={confirm}
          onInput={(e) => setConfirm((e.target as HTMLInputElement).value)}
          onKeyDown={(e) => e.key === 'Enter' && submit()}
        />
      </div>
      <button class="btn btn-primary btn-block" disabled={busy} onClick={submit}>
        {busy ? t('password.busy') : t('password.submit')}
      </button>
    </div>
  )
}
