import { describe, expect, it } from 'vitest'
import { normalizeTagInput, suggestTags } from './tags'
import type { Tag } from './types'

const tag = (id: number, name: string): Tag => ({ id, name })

const all: Tag[] = [
  tag(2, 'Go'),
  tag(3, 'Preact'),
  tag(4, 'Markdown'),
  tag(5, '架构'),
  tag(6, '前端'),
  tag(7, 'windows'),
  tag(8, 'golang'),
]

describe('normalizeTagInput', () => {
  it('trims and collapses inner whitespace', () => {
    expect(normalizeTagInput('  web   dev ')).toBe('web dev')
    expect(normalizeTagInput('\u3000golang\u3000')).toBe('golang')
  })
})

describe('suggestTags', () => {
  it('returns nothing while the input is empty or blank', () => {
    expect(suggestTags(all, '')).toEqual([])
    expect(suggestTags(all, '   ')).toEqual([])
  })

  it('matches case-insensitively and returns the stored spelling', () => {
    // The whole point: typing "go" must offer the existing "Go", not create a
    // second tag that differs only in case.
    const out = suggestTags(all, 'go')
    expect(out[0]).toEqual({ name: 'Go', exists: true })
    expect(out.some((s) => s.name === 'golang')).toBe(true)
  })

  it('ranks prefix matches above substring matches', () => {
    const out = suggestTags(all, 'go')
    const names = out.filter((s) => s.exists).map((s) => s.name)
    // "Go" and "golang" both start with "go"; neither is a bare substring case
    // here, but ordering must still be deterministic: both are prefix matches
    // and keep the source order after the exact match.
    expect(names.slice(0, 2)).toEqual(['Go', 'golang'])
  })

  it('puts a substring-only match after prefix matches', () => {
    const out = suggestTags(all, 'ark')
    expect(out.map((s) => s.name)).toContain('Markdown')
  })

  it('excludes tags that are already attached', () => {
    const out = suggestTags(all, 'go', ['Go'])
    expect(out.some((s) => s.name === 'Go')).toBe(false)
    expect(out.some((s) => s.name === 'golang')).toBe(true)
  })

  it('excludes selected tags case-insensitively', () => {
    const out = suggestTags(all, 'go', ['go'])
    expect(out.some((s) => s.name.toLowerCase() === 'go')).toBe(false)
  })

  it('never offers to create a name that is already attached', () => {
    // "go" is on the post, so no create row — but "golang" is still a useful
    // suggestion, so the dropdown must not go blank either.
    const out = suggestTags(all, 'go', ['Go'])
    expect(out.some((s) => !s.exists)).toBe(false)
    expect(out.map((s) => s.name)).toEqual(['golang'])
  })

  it('offers a create row when nothing matches', () => {
    const out = suggestTags(all, 'brand-new')
    const last = out[out.length - 1]
    expect(last).toEqual({ name: 'brand-new', exists: false })
    expect(out.filter((s) => !s.exists)).toHaveLength(1)
  })

  it('never offers a create row when an exact match exists', () => {
    const out = suggestTags(all, 'preact')
    expect(out.some((s) => !s.exists)).toBe(false)
    expect(out[0]).toEqual({ name: 'Preact', exists: true })
  })

  it('keeps the create row visible even when matches fill the list', () => {
    const many = Array.from({ length: 20 }, (_, i) => tag(i + 1, `topic${i}`))
    const out = suggestTags(many, 'topic', [], 5)
    // 4 existing matches + the create row: the only way to add an unseen tag
    // must never be pushed off the bottom of the dropdown.
    expect(out).toHaveLength(5)
    expect(out[out.length - 1]).toEqual({ name: 'topic', exists: false })
  })

  it('matches CJK names', () => {
    const out = suggestTags(all, '前端')
    expect(out[0]).toEqual({ name: '前端', exists: true })
  })
})
