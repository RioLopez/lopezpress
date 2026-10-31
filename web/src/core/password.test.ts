import { describe, expect, it } from 'vitest'
import { PASSWORD_MAX_BYTES, PASSWORD_MIN_CHARS, passwordProblem } from './password'

describe('passwordProblem', () => {
  it('rejects anything under eight characters', () => {
    expect(passwordProblem('')).toBe('short')
    expect(passwordProblem('aG7!kLp')).toBe('short')
  })

  it('accepts eight characters', () => {
    expect(passwordProblem('aG7!kLpq')).toBeNull()
  })

  it('counts characters, not UTF-16 units, so an emoji is one character', () => {
    // Seven characters: if this were measured with .length it would be eight
    // (seven emoji = 14 UTF-16 units) and wrongly accepted.
    expect(passwordProblem('😀😀😀😀😀😀😀')).toBe('short')
    expect(passwordProblem('😀😀😀😀😀😀😀😀')).toBeNull()
  })

  it('accepts 24 Chinese characters (72 bytes) and rejects 25 (75 bytes)', () => {
    expect(passwordProblem('密码'.repeat(12))).toBeNull()
    expect(passwordProblem('密码'.repeat(12) + '密')).toBe('long')
  })

  it('uses the same byte ceiling as bcrypt', () => {
    expect(PASSWORD_MAX_BYTES).toBe(72)
    expect(passwordProblem('x'.repeat(PASSWORD_MIN_CHARS))).toBeNull()
    expect(passwordProblem('x'.repeat(PASSWORD_MAX_BYTES))).toBeNull()
    expect(passwordProblem('x'.repeat(PASSWORD_MAX_BYTES + 1))).toBe('long')
  })
})
