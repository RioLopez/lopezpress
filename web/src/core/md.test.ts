import { describe, it, expect } from 'vitest'
import { escapeHTML, highlight, extractTOC } from './md'

describe('escapeHTML', () => {
  it('escapes all five HTML-unsafe characters', () => {
    expect(escapeHTML(`&<>"'`)).toBe('&amp;&lt;&gt;&quot;&#39;')
  })

  it('leaves plain text untouched', () => {
    expect(escapeHTML('hello world')).toBe('hello world')
  })
})

describe('highlight', () => {
  it('returns text unchanged for empty query or empty text', () => {
    expect(highlight('hello', '')).toBe('hello')
    expect(highlight('', 'x')).toBe('')
  })

  it('wraps matches in <mark class="hl">', () => {
    expect(highlight('hello world', 'world')).toBe(
      'hello <mark class="hl">world</mark>',
    )
  })

  it('matches case-insensitively', () => {
    expect(highlight('Alpha alpha', 'alpha')).toBe(
      '<mark class="hl">Alpha</mark> <mark class="hl">alpha</mark>',
    )
  })

  it('escapes HTML before matching, so input is never injected raw', () => {
    expect(highlight('<b>hi</b>', 'hi')).toBe(
      '&lt;b&gt;<mark class="hl">hi</mark>&lt;/b&gt;',
    )
  })

  it('treats regex specials in the query as literals', () => {
    expect(highlight('a.b', 'a.b')).toBe('<mark class="hl">a.b</mark>')
  })
})

describe('extractTOC', () => {
  it('returns an empty list for missing source', () => {
    expect(extractTOC(undefined)).toEqual([])
    expect(extractTOC('')).toEqual([])
  })

  it('extracts h2/h3 headings with their depth', () => {
    expect(extractTOC('## One\n### Two')).toEqual([
      { id: 'one', text: 'One', depth: 2 },
      { id: 'two', text: 'Two', depth: 3 },
    ])
  })

  it('ignores headings inside fenced code blocks', () => {
    const src = '```\n## not-a-heading\n```\n## real'
    expect(extractTOC(src)).toEqual([{ id: 'real', text: 'real', depth: 2 }])
  })

  it('deduplicates repeated headings with -N suffixes', () => {
    expect(extractTOC('## Intro\n## Intro')).toEqual([
      { id: 'intro', text: 'Intro', depth: 2 },
      { id: 'intro-2', text: 'Intro', depth: 2 },
    ])
  })
})
