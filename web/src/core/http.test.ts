import { describe, it, expect, afterEach } from 'vitest'
import { ApiError, errMessage, errStatus, setErrorTranslator } from './http'

describe('ApiError', () => {
  it('carries both status and message, and is an Error', () => {
    const e = new ApiError(500, 'boom')
    expect(e.status).toBe(500)
    expect(e.message).toBe('boom')
    expect(e).toBeInstanceOf(Error)
  })
})

describe('errMessage', () => {
  it('returns the ApiError message verbatim', () => {
    expect(errMessage(new ApiError(500, 'boom'), 'fallback')).toBe('boom')
  })

  it('falls back for any non-ApiError value', () => {
    expect(errMessage(new Error('x'), 'fallback')).toBe('fallback')
    expect(errMessage('string error', 'fallback')).toBe('fallback')
    expect(errMessage(undefined, 'fallback')).toBe('fallback')
  })
})

// The backend speaks English only, so a Chinese page that shows its text
// verbatim reads as half-translated. Mechanical statuses get a localised line;
// business errors keep the server text because it names the failing rule.
describe('errMessage localisation', () => {
  afterEach(() => setErrorTranslator(null))

  it('uses the injected line for a mechanical status', () => {
    setErrorTranslator((s) => (s === 429 ? '操作太频繁，请稍后再试' : undefined))
    expect(errMessage(new ApiError(429, 'too many captcha requests, slow down'), '备用')).toBe(
      '操作太频繁，请稍后再试',
    )
  })

  it('keeps the server text for a status with no localised line', () => {
    setErrorTranslator((s) => (s === 429 ? '操作太频繁，请稍后再试' : undefined))
    expect(errMessage(new ApiError(400, 'password is too common'), '备用')).toBe('password is too common')
  })

  it('keeps the server text when no translator is installed', () => {
    setErrorTranslator(null)
    expect(errMessage(new ApiError(429, 'slow down'), '备用')).toBe('slow down')
  })

  it('still falls back for non-ApiError values while a translator is installed', () => {
    setErrorTranslator(() => '不应被用到')
    expect(errMessage(new TypeError('boom'), '备用')).toBe('备用')
  })
})

describe('errStatus', () => {
  it('returns the ApiError status code', () => {
    expect(errStatus(new ApiError(404, 'nope'))).toBe(404)
  })

  it('returns undefined for any non-ApiError value', () => {
    expect(errStatus(new Error('x'))).toBeUndefined()
    expect(errStatus(undefined)).toBeUndefined()
  })
})
