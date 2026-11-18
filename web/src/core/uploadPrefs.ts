// Remembered image-upload defaults.
//
// The dialog is shown for a single picture; a multi-file drop or a paste has
// no place to ask, so it uses the last confirmed settings. Keeping this in
// localStorage means the answer survives a reload without the server having to
// store a per-user preference it does not otherwise need.

import type { ImageUploadOptions } from './types'

export type UploadPreset = 'original' | '1920' | '1280' | '640' | 'custom'

export interface UploadPrefs {
  preset: UploadPreset
  maxWidth: number
  maxHeight: number
  // JPEG quality, 50..100.
  quality: number
  lossless: boolean
}

const KEY = 'lopezpress.upload.prefs'

export const DEFAULT_UPLOAD_PREFS: UploadPrefs = {
  preset: 'original',
  maxWidth: 1920,
  maxHeight: 1920,
  quality: 82,
  lossless: false,
}

// Preset long edge. Both bounds are sent, so the server fits the picture into
// a square box — i.e. the LONG edge never exceeds the number, whatever the
// orientation.
export const PRESET_SIZES: Record<Exclude<UploadPreset, 'original' | 'custom'>, number> = {
  '1920': 1920,
  '1280': 1280,
  '640': 640,
}

function clampInt(n: unknown, lo: number, hi: number, fallback: number): number {
  const v = typeof n === 'number' ? n : Number(n)
  if (!Number.isFinite(v)) return fallback
  return Math.min(hi, Math.max(lo, Math.round(v)))
}

export function getUploadPrefs(): UploadPrefs {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { ...DEFAULT_UPLOAD_PREFS }
    const p = JSON.parse(raw) as Partial<UploadPrefs>
    const preset: UploadPreset =
      p.preset === '1920' || p.preset === '1280' || p.preset === '640' || p.preset === 'custom'
        ? p.preset
        : 'original'
    return {
      preset,
      maxWidth: clampInt(p.maxWidth, 16, 12000, DEFAULT_UPLOAD_PREFS.maxWidth),
      maxHeight: clampInt(p.maxHeight, 16, 12000, DEFAULT_UPLOAD_PREFS.maxHeight),
      quality: clampInt(p.quality, 50, 100, DEFAULT_UPLOAD_PREFS.quality),
      lossless: p.lossless === true,
    }
  } catch {
    return { ...DEFAULT_UPLOAD_PREFS }
  }
}

export function saveUploadPrefs(p: UploadPrefs): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(p))
  } catch {
    /* quota / private mode: the dialog still works for this upload */
  }
}

// prefsToOptions turns the remembered UI state into the request fields.
//
// It is deliberately conservative: an untouched "original" sends NOTHING, so
// the server keeps its old behaviour and a PNG is not needlessly re-encoded
// through the quality path. Quality/lossless are only meaningful for JPEG, so
// they are attached when the selected file says it is one (or when the caller
// does not know the type, in which case the server ignores them safely).
export function prefsToOptions(p: UploadPrefs, file?: File): ImageUploadOptions {
  // A batch can mix formats. The server refuses a resize for GIF/WebP (it
  // would drop frames / has no encoder), and it does so with a 400 that loses
  // that one file, so a format that cannot be transformed is sent as-is even
  // when the remembered preset asks for 1920px.
  if (file && !isTransformableImage(file)) return {}
  const isJPEG = !file || /^image\/jpe?g$/i.test(file.type)
  const qualityTouched = p.lossless || p.quality !== DEFAULT_UPLOAD_PREFS.quality
  if (p.preset === 'original') {
    return isJPEG && qualityTouched ? { quality: p.quality, lossless: p.lossless } : {}
  }
  const opts: ImageUploadOptions = {
    max_width: p.maxWidth,
    max_height: p.maxHeight,
  }
  if (isJPEG && qualityTouched) {
    opts.quality = p.quality
    opts.lossless = p.lossless
  }
  return opts
}

// isTransformableImage reports whether the format accepts a server-side size
// change. WebP has no encoder on the server and GIF resize would freeze the
// animation, so the dialog disables the size controls for them instead of
// letting the upload fail after the bytes were sent.
export function isTransformableImage(file: File): boolean {
  return /^image\/(png|jpe?g)$/i.test(file.type)
}

export function isLosslessFormat(file: File): boolean {
  return /^image\/png$/i.test(file.type)
}
