import { useRef, useState } from 'preact/hooks'
import type { CaptchaChallenge } from '../core/types'
import { useI18n } from '../core/i18n'

// One challenge, two interactions. Which one renders is decided by
// challenge.kind, and the server picks that from this client's failure
// history — so a bot that retries gets the harder one automatically.
//
// The widget owns no network calls: it reports what the user produced and lets
// the page decide when to submit. That keeps the "refresh" path (fetch a new
// challenge) in one place, because a wrong answer also returns a new challenge.

interface Props {
  challenge: CaptchaChallenge
  /** Reports the solved answer. Exactly one of code/x is meaningful. */
  onSolved: (answer: { code: string; x: number }) => void
  /** Asks the page for a fresh challenge (user clicked the picture). */
  onRefresh: () => void
  disabled?: boolean
}

export function CaptchaWidget({ challenge, onSolved, onRefresh, disabled }: Props) {
  return challenge.kind === 'slide' ? (
    <SlidePuzzle challenge={challenge} onSolved={onSolved} disabled={disabled} />
  ) : (
    <ImageCaptcha challenge={challenge} onSolved={onSolved} onRefresh={onRefresh} disabled={disabled} />
  )
}

function ImageCaptcha({
  challenge,
  onSolved,
  onRefresh,
  disabled,
}: {
  challenge: CaptchaChallenge
  onSolved: Props['onSolved']
  onRefresh: () => void
  disabled?: boolean
}) {
  const { t } = useI18n()
  const [code, setCode] = useState('')

  return (
    <div class="captcha-block">
      <label>{t('auth.captchaLabel')}</label>
      <div class="captcha-row">
        {/* Clicking the picture is the refresh gesture everyone expects; the
            button next to it exists so it is also discoverable. */}
        <img
          class="captcha-img"
          src={challenge.image}
          alt=""
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
            onSolved({ code: v, x: 0 })
          }}
          onKeyDown={(e) => e.key === 'Enter' && onSolved({ code, x: 0 })}
        />
      </div>
      <button type="button" class="captcha-refresh" onClick={onRefresh} disabled={disabled}>
        {t('auth.captchaRefresh')}
      </button>
    </div>
  )
}

// SlidePuzzle drags a piece into a dimmed notch. The horizontal target is the
// answer and never reaches the browser: the notch is painted into the
// background by the server, so the user can see *that* a gap exists without the
// payload saying *where* the number is.
function SlidePuzzle({
  challenge,
  onSolved,
  disabled,
}: {
  challenge: CaptchaChallenge
  onSolved: Props['onSolved']
  disabled?: boolean
}) {
  const { t } = useI18n()
  const boxRef = useRef<HTMLDivElement>(null)
  const drag = useRef<{ startX: number; startPos: number; width: number; pos: number } | null>(null)
  const [pos, setPos] = useState(0)

  // Geometry comes from the server so the piece lines up with the notch no
  // matter how the image is scaled: everything is expressed as a fraction of
  // the 300x150 canvas.
  const tileW = ((challenge.tile_size || 44) / 300) * 100
  const tileTop = ((challenge.tile_y || 0) / 150) * 100
  const maxPos = Math.max(0, 1 - tileW / 100)

  const onDown = (e: PointerEvent) => {
    if (disabled) return
    const box = boxRef.current
    if (!box) return
    drag.current = {
      startX: e.clientX,
      startPos: pos,
      width: box.getBoundingClientRect().width || 1,
      pos,
    }
    ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
    e.preventDefault()
  }

  const onMove = (e: PointerEvent) => {
    const d = drag.current
    if (!d) return
    const next = Math.min(maxPos, Math.max(0, d.startPos + (e.clientX - d.startX) / d.width))
    d.pos = next
    setPos(next)
    e.preventDefault()
  }

  const onUp = () => {
    const d = drag.current
    if (!d) return
    drag.current = null
    // Per-mille, not pixels: the form and the image are both responsive, and a
    // pixel offset would disagree with the server by the scale factor.
    onSolved({ code: '', x: Math.round(d.pos * 1000) })
  }

  return (
    <div class="captcha-block">
      <label>{t('auth.captchaSlideLabel')}</label>
      <div class="captcha-slide" ref={boxRef}>
        <img class="captcha-slide-bg" src={challenge.bg} alt="" draggable={false} />
        <img
          class="captcha-slide-tile"
          src={challenge.tile}
          alt=""
          draggable={false}
          style={{ left: `${pos * 100}%`, top: `${tileTop}%`, width: `${tileW}%` }}
          onPointerDown={onDown}
          onPointerMove={onMove}
          onPointerUp={onUp}
          onPointerCancel={onUp}
        />
      </div>
      <p class="muted captcha-hint">{t('auth.captchaSlideHint')}</p>
    </div>
  )
}
