import { describe, it, expect } from 'vitest'
import { ApiError, errMessage, errStatus } from './http'

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

describe('errStatus', () => {
  it('returns the ApiError status code', () => {
    expect(errStatus(new ApiError(404, 'nope'))).toBe(404)
  })

  it('returns undefined for any non-ApiError value', () => {
    expect(errStatus(new Error('x'))).toBeUndefined()
    expect(errStatus(undefined)).toBeUndefined()
  })
})
