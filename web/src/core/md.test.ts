import { describe, it, expect } from 'vitest'
import { escapeHTML, highlight, extractTOC, isSafeHref, isSafeSrc, isSafeSrcset } from './md'

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

  // R2-01 / R3-01 / R5-03: the empty-query branch used to return `text`
  // verbatim, so a post title carrying raw HTML executed on the anonymous
  // home page (and a body whose entities decoded back into tags did the same
  // in the excerpt). Every branch now escapes.
  it('escapes even with no query — the stored-XSS gadget (R2-01/R3-01)', () => {
    expect(highlight('<img src=x onerror="alert(1)">', '')).toBe(
      '&lt;img src=x onerror=&quot;alert(1)&quot;&gt;',
    )
  })

  it('escapes a query that itself contains HTML', () => {
    // The query is escaped before it becomes a regex, so "<b>" matches the
    // escaped "&lt;b&gt;" run and the inserted <mark> never contains raw
    // attacker-controlled characters.
    expect(highlight('say <b>hi</b>', '<b>')).toBe(
      'say <mark class="hl">&lt;b&gt;</mark>hi&lt;/b&gt;',
    )
  })
})

describe('isSafeHref', () => {
  it('accepts http, https, mailto and relative paths', () => {
    expect(isSafeHref('https://example.com/x')).toBe(true)
    expect(isSafeHref('http://example.com')).toBe(true)
    expect(isSafeHref('mailto:a@b.c')).toBe(true)
    expect(isSafeHref('/about')).toBe(true)
    expect(isSafeHref('./x')).toBe(true)
    expect(isSafeHref('#anchor')).toBe(true)
    expect(isSafeHref('page.html')).toBe(true)
    expect(isSafeHref('')).toBe(true)
  })

  // R5-02: the old regex treated a leading "/" as same-origin, so a
  // protocol-relative URL pointed at the attacker's host.
  it('rejects protocol-relative URLs', () => {
    expect(isSafeHref('//evil.example/x')).toBe(false)
    expect(isSafeHref('///evil.example')).toBe(false)
    expect(isSafeHref('\\\\evil.example')).toBe(false)
  })

  it('rejects javascript: and data: in any casing', () => {
    expect(isSafeHref('javascript:alert(1)')).toBe(false)
    expect(isSafeHref('JaVaScRiPt:alert(1)')).toBe(false)
    expect(isSafeHref('  javascript:alert(1)')).toBe(false)
    expect(isSafeHref('data:text/html,<script>')).toBe(false)
  })
})

describe('isSafeSrc', () => {
  it('accepts http(s) and same-origin relative paths', () => {
    expect(isSafeSrc('https://cdn.example/a.png')).toBe(true)
    expect(isSafeSrc('/uploads/a.png')).toBe(true)
    expect(isSafeSrc('a.png')).toBe(true)
    expect(isSafeSrc('')).toBe(true)
  })

  // R5-02: the old check was `!src.startsWith('data:')`, which a capital
  // "DATA:" walked straight through.
  it('rejects data:, blob: and javascript: in any casing', () => {
    expect(isSafeSrc('data:image/svg+xml,<svg/>')).toBe(false)
    expect(isSafeSrc('DATA:image/svg+xml,<svg/>')).toBe(false)
    expect(isSafeSrc('Data:image/png;base64,xx')).toBe(false)
    expect(isSafeSrc('blob:https://x/uuid')).toBe(false)
    expect(isSafeSrc('javascript:alert(1)')).toBe(false)
  })

  it('rejects protocol-relative sources', () => {
    expect(isSafeSrc('//evil.example/x.png')).toBe(false)
  })
})

describe('isSafeSrcset', () => {
  it('accepts a list of same-origin / http(s) candidates', () => {
    expect(isSafeSrcset('a.png 1x, b.png 2x')).toBe(true)
    expect(isSafeSrcset('//ok.example/a.png 100w')).toBe(false)
  })

  // R3-09 / R5-02: srcset was not checked at all, so one unsafe candidate in
  // a list of safe ones was enough to make the browser fetch it.
  it('rejects when ANY candidate is unsafe', () => {
    expect(isSafeSrcset('a.png 1x, //evil.example/b.png 2x')).toBe(false)
    expect(isSafeSrcset('a.png 1x, DATA:image/svg+xml,x 2x')).toBe(false)
  })

  it('treats an empty value as safe', () => {
    expect(isSafeSrcset('')).toBe(true)
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
