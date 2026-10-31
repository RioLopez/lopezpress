import { describe, it, expect } from 'vitest'
import { escapeHTML, highlight, extractTOC, isSafeHref, isSafeSrc, isSafeSrcset, plainText, excerpt } from './md'

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

// The list card used to render the whole article and slice 120 characters off
// the result. These cases pin the visible text it is supposed to keep, because
// a regex pass that quietly drops a paragraph is indistinguishable from a post
// whose body is short.
describe('plainText', () => {
  it('drops heading marks, emphasis and inline code markers', () => {
    expect(plainText('## 标题\n\n正文有**粗体**和*斜体*还有`代码`')).toBe(
      '标题 正文有粗体和斜体还有代码',
    )
  })

  it('keeps image alt text and link labels but not their URLs', () => {
    expect(plainText('看图 ![示意图](/uploads/a.jpg) 和 [链接](https://example.com/x)')).toBe(
      '看图 示意图 和 链接',
    )
  })

  it('removes fenced code blocks entirely', () => {
    expect(plainText('前\n\n```go\nfunc main() {}\n```\n\n后')).toBe('前 后')
  })

  it('removes an unterminated fence instead of leaking the rest', () => {
    expect(plainText('前\n```\n未闭合的代码')).toBe('前 未闭合的代码')
  })

  it('drops table separator rows and keeps cell text apart', () => {
    expect(plainText('| 名称 | 值 |\n| --- | --- |\n| 甲 | 1 |')).toBe('名称 值 甲 1')
  })

  it('drops list bullets and lets a rule be a rule', () => {
    expect(plainText('- 第一项\n- 第二项\n\n- - -\n\n结尾')).toBe('第一项 第二项 结尾')
  })

  it('strips tags but keeps their text, and leaves a bare less-than alone', () => {
    expect(plainText('a < b 且 <span class="x">文字</span>')).toBe('a < b 且 文字')
  })

  it('decodes entities once, never recursively', () => {
    expect(plainText('&amp;lt; 与 &amp;')).toBe('&lt; 与 &')
  })

  it('leaves snake_case identifiers intact', () => {
    expect(plainText('use snake_case_name here')).toBe('use snake_case_name here')
  })
})

describe('excerpt', () => {
  it('returns empty for empty input', () => {
    expect(excerpt('')).toBe('')
    expect(excerpt(null)).toBe('')
    expect(excerpt(undefined)).toBe('')
  })

  it('keeps short text as-is, without an ellipsis', () => {
    expect(excerpt('短文本', 120)).toBe('短文本')
  })

  it('cuts by code point so an emoji is never sliced in half', () => {
    const out = excerpt('😀'.repeat(200), 10)
    expect(out).toBe('😀'.repeat(10) + '…')
    expect(out).not.toContain('\ufffd')
  })

  it('appends an ellipsis only when it actually truncated', () => {
    expect(excerpt('a'.repeat(200), 10)).toBe('aaaaaaaaaa…')
    expect(excerpt('a'.repeat(10), 10)).toBe('aaaaaaaaaa')
  })
})
