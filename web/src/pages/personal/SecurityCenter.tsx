import { useState } from 'preact/hooks'
import { useAuth } from '../../core/auth'
import { useI18n } from '../../core/i18n'
import { useNav } from '../../core/router'
import { useApp } from '../../core/app'
import { changePassword, meLoginHistory, logoutAll, errMessage, errStatus } from '../../core/api'
import type { UserPublic } from '../../core/types'
import { useQuery } from '../../core/useQuery'

// Partially mask an email: keep first + last char of the local part, hide the
// middle (spec 20 §六.1).
const maskEmail = (email: string) => {
  const [name, domain] = email.split('@')
  if (!domain) return email
  if (name.length <= 2) return `***@${domain}`
  return `${name[0]}***${name[name.length - 1]}@${domain}`
}

export function SecurityCenter({ user }: { user: UserPublic | null }) {
  const { t } = useI18n()
  const { logout, adoptToken } = useAuth()
  const { pushToast } = useApp()
  const [, navigate] = useNav()
  const [show, setShow] = useState(false)

  // Change-password form state.
  const [cur, setCur] = useState('')
  const [npw, setNpw] = useState('')
  const [cfm, setCfm] = useState('')
  const [pwBusy, setPwBusy] = useState(false)
  const [pwErr, setPwErr] = useState('')

  // Login history (self-service).
  const { data: logs, error: logsErr } = useQuery(() => meLoginHistory(20), [])

  if (!user) return <p class="muted center-empty">{t('center.securityEmpty')}</p>

  const submitPw = (e: Event) => {
    e.preventDefault()
    setPwErr('')
    if (npw.length < 8) {
      setPwErr(t('center.secPwTooShort'))
      return
    }
    if (npw !== cfm) {
      setPwErr(t('center.secPwMismatch'))
      return
    }
    setPwBusy(true)
    changePassword({ current_password: cur, new_password: npw })
      .then(async (res) => {
        // Swap in the fresh token the server issued (the change revoked the
        // previous one via the token-version bump).
        await adoptToken(res.token)
        pushToast('success', t('center.secPwChanged'))
        setCur('')
        setNpw('')
        setCfm('')
      })
      .catch((e) =>
        setPwErr(errStatus(e) === 401 ? t('center.secPwWrong') : errMessage(e, t('center.secPwWrong'))),
      )
      .finally(() => setPwBusy(false))
  }

  const doLogoutAll = () => {
    if (!confirm(t('center.secLogoutAllConfirm'))) return
    logoutAll()
      .then(() => {
        logout()
        pushToast('info', t('center.secLogoutAllDone'))
        navigate('/login')
      })
      .catch(() => {
        // Even if the server call fails, clear the local session.
        logout()
        navigate('/login')
      })
  }

  const fmtTime = (s: string) => {
    const d = new Date(s)
    return isNaN(d.getTime()) ? s : d.toLocaleString()
  }

  return (
    <div class="center-section">
      {/* Account basics (email masked) */}
      <section class="center-security">
        <div class="info-card">
          <div class="info-row">
            <span class="info-label">{t('center.account')}</span>
            <span class="info-value">{show ? user.email : maskEmail(user.email)}</span>
          </div>
          <div class="info-row">
            <span class="info-label">{t('center.username')}</span>
            <span class="info-value">{user.username}</span>
          </div>
          <button class="btn btn-sm btn-ghost" onClick={() => setShow((s) => !s)}>
            {show ? t('center.hide') : t('center.show')}
          </button>
        </div>
        <p class="muted center-note">{t('center.securityNote')}</p>
      </section>

      {/* Change password */}
      <section class="sec-card">
        <h2 class="center-subtitle">{t('center.secChangePw')}</h2>
        <form class="sec-form" onSubmit={submitPw}>
          <label class="sec-field">
            <span>{t('center.secCurrentPw')}</span>
            <input
              type="password"
              value={cur}
              autocomplete="current-password"
              onInput={(e) => setCur((e.currentTarget as HTMLInputElement).value)}
            />
          </label>
          <label class="sec-field">
            <span>{t('center.secNewPw')}</span>
            <input
              type="password"
              value={npw}
              autocomplete="new-password"
              onInput={(e) => setNpw((e.currentTarget as HTMLInputElement).value)}
            />
          </label>
          <label class="sec-field">
            <span>{t('center.secConfirmPw')}</span>
            <input
              type="password"
              value={cfm}
              autocomplete="new-password"
              onInput={(e) => setCfm((e.currentTarget as HTMLInputElement).value)}
            />
          </label>
          {pwErr && <p class="sec-error">{pwErr}</p>}
          <button class="btn btn-primary" type="submit" disabled={pwBusy}>
            {t('center.secPwBtn')}
          </button>
        </form>
      </section>

      {/* Login history */}
      <section class="sec-card">
        <h2 class="center-subtitle">{t('center.secLoginHistory')}</h2>
        {logs === null && <p class="muted center-loading">{t('center.loading')}</p>}
        {logsErr && <p class="muted">{t('center.loadError')}</p>}
        {logs && logs.length === 0 && <p class="muted center-empty">{t('center.secNoLogins')}</p>}
        {logs && logs.length > 0 && (
          <ul class="login-list">
            {logs.map((l) => (
              <li class="login-row" key={l.id}>
                <span class={`badge ${l.success ? 'badge--ok' : 'badge--fail'}`}>
                  {l.success ? t('center.secLoginOk') : t('center.secLoginFail')}
                </span>
                <div class="login-meta">
                  <div>
                    {t('center.secIp')}: {l.ip || '—'}
                  </div>
                  <div class="muted login-ua">
                    {t('center.secDevice')}: {l.ua || '—'} · {fmtTime(l.created_at)}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* Sign out everywhere */}
      <section class="sec-card">
        <button class="btn btn-danger" onClick={doLogoutAll}>
          {t('center.secLogoutAll')}
        </button>
      </section>
    </div>
  )
}
