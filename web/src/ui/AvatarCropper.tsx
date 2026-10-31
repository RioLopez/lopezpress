import { useEffect, useRef, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'

// Avatar cropper.
//
// Why the browser and not the server: Go's standard library only offers
// nearest-neighbour scaling (image/draw), which makes a 512px avatar look
// blurry. Adding golang.org/x/image/... for CatmullRom would mean a new
// dependency and CPU work on a 1 GB VPS. The browser already has a high-quality
// resampler and a WebP encoder, so it does the crop and the server's job is to
// REJECT anything that does not match the contract (square, 512, small).
//
// The result: one 512×512 WebP, usually 30–80 KB, instead of a 3 MB phone
// photo being stored and then downscaled by CSS on every render.

const VIEWPORT = 288 // on-screen crop box, in CSS pixels
const OUTPUT = 512 // uploaded image size, in device pixels

interface Props {
  file: File
  onCancel: () => void
  // Receives the encoded image. `blob.type` tells the caller which format the
  // browser actually managed to produce ('image/webp' on modern ones).
  onDone: (blob: Blob) => void
}

export function AvatarCropper({ file, onCancel, onDone }: Props) {
  const { t } = useI18n()
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const imgRef = useRef<HTMLImageElement | null>(null)
  const dragRef = useRef<{ startX: number; startY: number; ox: number; oy: number } | null>(null)

  const [ready, setReady] = useState(false)
  const [zoom, setZoom] = useState(1)
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const [error, setError] = useState(false)

  // Decode the selected file once. createObjectURL keeps the bytes in the
  // browser — nothing is uploaded until the user confirms the framing.
  useEffect(() => {
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.onload = () => {
      imgRef.current = img
      setReady(true)
    }
    img.onerror = () => setError(true)
    img.src = url
    return () => URL.revokeObjectURL(url)
  }, [file])

  // Redraw whenever the framing changes.
  useEffect(() => {
    const canvas = canvasRef.current
    const img = imgRef.current
    if (!canvas || !img) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    const geo = geometry(img, zoom, offset)
    ctx.clearRect(0, 0, VIEWPORT, VIEWPORT)
    ctx.drawImage(img, geo.dx, geo.dy, geo.dw, geo.dh)
  }, [zoom, offset, ready])

  const onPointerDown = (e: PointerEvent) => {
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
    dragRef.current = { startX: e.clientX, startY: e.clientY, ox: offset.x, oy: offset.y }
  }

  const onPointerMove = (e: PointerEvent) => {
    const drag = dragRef.current
    const img = imgRef.current
    if (!drag || !img) return
    setOffset(clampOffset(img, zoom, drag.ox + (e.clientX - drag.startX), drag.oy + (e.clientY - drag.startY)))
  }

  const onPointerUp = () => {
    dragRef.current = null
  }

  const apply = () => {
    const img = imgRef.current
    if (!img) return
    const geo = geometry(img, zoom, offset)
    const out = document.createElement('canvas')
    out.width = OUTPUT
    out.height = OUTPUT
    const ctx = out.getContext('2d')
    if (!ctx) {
      setError(true)
      return
    }
    // Scale the *visible* crop box up to the output size. Sampling from the
    // full-resolution image (not the on-screen canvas) is what keeps the
    // avatar sharp.
    const k = OUTPUT / VIEWPORT
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(img, geo.dx * k, geo.dy * k, geo.dw * k, geo.dh * k)
    out.toBlob(
      (blob) => {
        if (blob) onDone(blob)
        else setError(true)
      },
      'image/webp',
      0.85,
    )
  }

  return (
    <div class="modal-backdrop" onClick={onCancel}>
      <div class="modal avatar-crop" onClick={(e) => e.stopPropagation()}>
        <h2 class="avatar-crop-title">{t('imgUpload.cropTitle')}</h2>
        <p class="muted avatar-crop-hint">{t('imgUpload.cropHint')}</p>

        {error ? (
          <p class="sec-error">{t('imgUpload.badFormat')}</p>
        ) : (
          <>
            <div
              class="avatar-crop-stage"
              onPointerDown={onPointerDown}
              onPointerMove={onPointerMove}
              onPointerUp={onPointerUp}
              onPointerCancel={onPointerUp}
            >
              <canvas ref={canvasRef} width={VIEWPORT} height={VIEWPORT} class="avatar-crop-canvas" />
              <div class="avatar-crop-mask" aria-hidden="true" />
            </div>
            <label class="avatar-crop-zoom">
              <span class="muted">−</span>
              <input
                type="range"
                min="1"
                max="3"
                step="0.01"
                value={zoom}
                onInput={(e) => {
                  const z = Number((e.target as HTMLInputElement).value)
                  setZoom(z)
                  setOffset((o) => clampOffset(imgRef.current, z, o.x, o.y))
                }}
              />
              <span class="muted">+</span>
            </label>
          </>
        )}

        <div class="modal-foot">
          <button class="btn btn-ghost" onClick={onCancel}>
            {t('imgUpload.cropCancel')}
          </button>
          <button class="btn btn-primary" disabled={!ready || error} onClick={apply}>
            {t('imgUpload.cropApply')}
          </button>
        </div>
      </div>
    </div>
  )
}

// geometry computes where the image lands inside the fixed-size viewport, in
// viewport coordinates. `cover` (max of the two ratios) guarantees the crop box
// is always fully covered, so no transparent corners can be captured.
function geometry(img: HTMLImageElement, zoom: number, offset: { x: number; y: number }) {
  const base = Math.max(VIEWPORT / img.width, VIEWPORT / img.height)
  const scale = base * zoom
  const dw = img.width * scale
  const dh = img.height * scale
  return {
    dw,
    dh,
    dx: VIEWPORT / 2 - dw / 2 + offset.x,
    dy: VIEWPORT / 2 - dh / 2 + offset.y,
  }
}

// clampOffset keeps the image covering the viewport: you may slide it around,
// never far enough to expose a gap.
function clampOffset(img: HTMLImageElement | null, zoom: number, x: number, y: number) {
  if (!img) return { x: 0, y: 0 }
  const { dw, dh } = geometry(img, zoom, { x: 0, y: 0 })
  const maxX = Math.max(0, (dw - VIEWPORT) / 2)
  const maxY = Math.max(0, (dh - VIEWPORT) / 2)
  return {
    x: Math.min(maxX, Math.max(-maxX, x)),
    y: Math.min(maxY, Math.max(-maxY, y)),
  }
}
