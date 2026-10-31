import { useState } from 'preact/hooks'
import type { CaptchaChallenge } from '../core/types'
import { useI18n } from '../core/i18n'

// One interaction: read four characters off a picture and type them.
//
// An earlier version escalated to a sliding puzzle after repeated failures. It
// was removed — an unfamiliar widget costs honest users more than it costs
// attackers, and every defence that actually matters is server side and
// invisible: the answer is never sent, each picture authorises one attempt,
// three wrong tries kill it, it expires in two minutes, it is bound to the
// client that asked for it, and issuing is rate limited per client.
//
// The widget owns no network calls: it reports what the user typed and lets the
// page decide when to submit, so the "refresh" path lives in one place.

interface Props {
  challenge: CaptchaChallenge
  /** Reports the characters the user read off the picture. */
  onSolved: (answer: { code: string }) => void
  /** Asks the page for a fresh challenge (user clicked the picture). */
  onRefresh: () => void
  disabled?: boolean
}

export function CaptchaWidget({ challenge, onSolved, onRefresh, disabled }: Props) {
  const { t } = useI18n()
  const [code, setCode] = useState('')

  return (
    <div class="captcha-block">
      <label>{t('auth.captchaLabel')}</label>
      <div class="captcha-row">
        {/* Clicking the picture is the refresh gesture everyone expects; the
            button under it exists so it is also discoverable. */}
        <img
          class="captcha-img"
          src={challenge.image}
          alt=""
          draggable={false}
          onClick={onRefresh}
          title={t('auth.captchaRefresh')}
        />
        <input
          value={code}
          maxLength={4}
          autoComplete="off"
          autoCapitalize="characters"
          spellcheck={false}
          placeholder={t('auth.captchaPlaceholder')}
          disabled={disabled}
          onInput={(e) => {
            const v = (e.target as HTMLInputElement).value
            setCode(v)
            onSolved({ code: v })
          }}
          onKeyDown={(e) => e.key === 'Enter' && onSolved({ code })}
        />
      </div>
      <button type="button" class="captcha-refresh" onClick={onRefresh} disabled={disabled}>
        {t('auth.captchaRefresh')}
      </button>
    </div>
  )
}
