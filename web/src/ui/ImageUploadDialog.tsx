import { useEffect, useMemo, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { Modal } from './Modal'
import type { ImageUploadOptions } from '../core/types'
import {
  getUploadPrefs,
  isLosslessFormat,
  isTransformableImage,
  PRESET_SIZES,
  prefsToOptions,
  saveUploadPrefs,
  type UploadPreset,
} from '../core/uploadPrefs'

interface Props {
  file: File
  onConfirm: (opts: ImageUploadOptions) => void
  onCancel: () => void
}

// A single-image upload is the only moment where asking makes sense: a batch
// or a paste has no per-file conversation, and uses the remembered defaults.
//
// The dialog never computes or claims a final byte size (the server does the
// encode); it shows the target BOX and the rule, which is the part the author
// actually controls.
export function ImageUploadDialog({ file, onConfirm, onCancel }: Props) {
  const { t } = useI18n()
  const saved = useMemo(() => getUploadPrefs(), [])
  const supported = isTransformableImage(file)
  const isPNG = isLosslessFormat(file)
  const isJPEG = /^image\/jpe?g$/i.test(file.type)

  const [preset, setPreset] = useState<UploadPreset>(supported ? saved.preset : 'original')
  const [maxWidth, setMaxWidth] = useState(saved.maxWidth)
  const [maxHeight, setMaxHeight] = useState(saved.maxHeight)
  const [quality, setQuality] = useState(saved.quality)
  const [lossless, setLossless] = useState(saved.lossless)
  const [src, setSrc] = useState<{ w: number; h: number } | null>(null)

  const previewURL = useMemo(() => URL.createObjectURL(file), [file])
  useEffect(() => () => URL.revokeObjectURL(previewURL), [previewURL])

  // Read the natural size from the browser so the "output size" line can be
  // concrete. The server measures the file itself; this is display only.
  useEffect(() => {
    const img = new Image()
    img.onload = () => setSrc({ w: img.naturalWidth, h: img.naturalHeight })
    img.src = previewURL
    return () => {
      img.onload = null
    }
  }, [previewURL])

  const pickPreset = (p: UploadPreset) => {
    if (!supported && p !== 'original') return
    setPreset(p)
    if (p === '1920' || p === '1280' || p === '640') {
      setMaxWidth(PRESET_SIZES[p])
      setMaxHeight(PRESET_SIZES[p])
    }
  }

  const target = useMemo(() => {
    if (!src) return ''
    if (!supported || preset === 'original') return `${src.w} × ${src.h}`
    const scale = Math.min(
      1,
      maxWidth > 0 ? maxWidth / src.w : 1,
      maxHeight > 0 ? maxHeight / src.h : 1,
    )
    const w = Math.max(1, Math.round(src.w * scale))
    const h = Math.max(1, Math.round(src.h * scale))
    return `${w} × ${h}`
  }, [src, supported, preset, maxWidth, maxHeight])

  const apply = () => {
    const prefs = {
      preset: supported ? preset : ('original' as UploadPreset),
      maxWidth,
      maxHeight,
      quality,
      lossless,
    }
    saveUploadPrefs(prefs)
    onConfirm(prefsToOptions(prefs, file))
  }

  return (
    <Modal
      title={t('uploadOpts.title')}
      onClose={onCancel}
      footer={
        <>
          <button class="btn btn-ghost" onClick={onCancel}>
            {t('uploadOpts.cancel')}
          </button>
          <button class="btn btn-primary" onClick={apply}>
            {t('uploadOpts.apply')}
          </button>
        </>
      }
    >
      <div class="upload-opt">
        <div class="upload-opt-preview">
          <img src={previewURL} alt={file.name} />
          <div class="upload-opt-meta">
            <div class="upload-opt-name">{file.name}</div>
            <div class="muted">
              {t('uploadOpts.target')}:{' '}
              {target || (src ? `—` : '…')}
              {preset === 'original' && supported ? ` · ${t('uploadOpts.keepOriginal')}` : ''}
            </div>
          </div>
        </div>

        {!supported && <p class="settings-hint">{t('uploadOpts.unsupported')}</p>}

        <div class="upload-opt-presets" role="group">
          {(['original', '1920', '1280', '640', 'custom'] as UploadPreset[]).map((p) => (
            <button
              key={p}
              class={`btn btn-ghost btn-sm${preset === p ? ' is-active' : ''}`}
              disabled={!supported && p !== 'original'}
              onClick={() => pickPreset(p)}
            >
              {p === 'original' ? t('uploadOpts.original') : p === 'custom' ? t('uploadOpts.custom') : p}
            </button>
          ))}
        </div>

        {supported && preset === 'custom' && (
          <div class="settings-row upload-opt-size">
            <label class="settings-field">
              <span>{t('uploadOpts.maxWidth')}</span>
              <input
                type="number"
                min={16}
                max={12000}
                value={maxWidth}
                onInput={(e) => {
                  setMaxWidth(Number((e.target as HTMLInputElement).value) || 0)
                  setPreset('custom')
                }}
              />
            </label>
            <label class="settings-field">
              <span>{t('uploadOpts.maxHeight')}</span>
              <input
                type="number"
                min={16}
                max={12000}
                value={maxHeight}
                onInput={(e) => {
                  setMaxHeight(Number((e.target as HTMLInputElement).value) || 0)
                  setPreset('custom')
                }}
              />
            </label>
          </div>
        )}
        {supported && <p class="settings-hint">{t('uploadOpts.ratioHint')}</p>}

        {isJPEG && (
          <div class="settings-field">
            <label>{t('uploadOpts.quality')}</label>
            <div class="upload-opt-quality">
              <input
                type="range"
                min={50}
                max={100}
                value={lossless ? 100 : quality}
                disabled={lossless}
                onInput={(e) => setQuality(Number((e.target as HTMLInputElement).value))}
              />
              <span>{lossless ? 100 : quality}</span>
            </div>
            <label class="toggle">
              <input
                type="checkbox"
                checked={lossless}
                onInput={(e) => setLossless((e.target as HTMLInputElement).checked)}
              />
              {t('uploadOpts.lossless')}
            </label>
            <p class="settings-hint">{t('uploadOpts.losslessHint')}</p>
          </div>
        )}
        {isPNG && <p class="settings-hint">{t('uploadOpts.pngHint')}</p>}
        <p class="settings-hint">{t('uploadOpts.remember')}</p>
      </div>
    </Modal>
  )
}
