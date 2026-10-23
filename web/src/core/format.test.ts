import { describe, it, expect } from 'vitest'
import { fmtDate, fmtDay } from './format'

describe('fmtDate', () => {
  it('renders an em-dash for missing input', () => {
    expect(fmtDate()).toBe('—')
    expect(fmtDate('')).toBe('—')
  })

  it('renders an em-dash for an invalid date', () => {
    expect(fmtDate('not-a-date')).toBe('—')
  })

  it('formats a valid ISO timestamp (locale-aware)', () => {
    const out = fmtDate('2026-10-23T12:00:00Z', 'zh-CN')
    expect(out).not.toBe('—')
    expect(out.length).toBeGreaterThan(0)
  })

  it('falls back to the browser locale when none is passed', () => {
    const out = fmtDate('2026-10-23T12:00:00Z')
    expect(out).not.toBe('—')
  })
})

describe('fmtDay', () => {
  it('returns empty string for missing input', () => {
    expect(fmtDay()).toBe('')
    expect(fmtDay('')).toBe('')
  })

  it('returns only the ISO calendar day', () => {
    expect(fmtDay('2026-10-23T14:30:00Z')).toBe('2026-10-23')
  })
})
