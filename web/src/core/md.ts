import { marked } from 'marked'

marked.setOptions({ gfm: true, breaks: true })

// --- URL policy -----------------------------------------------------------
//
// Prefix matching is not a URL policy. It is case-sensitive ("DATA:" and
// "JaVaScRiPt:" sail straight past `startsWith('data:')`), and it lets a
// protocol-relative URL — "//evil.example/x" — through, because the leading
// "/" looks like a same-origin path. Such a URL inherits the page scheme but
// points at the attacker's host: the browser resolves it, not us.
//
// Both predicates therefore parse the value and inspect the *scheme* (plus
// the leading characters), which is what the browser actually acts on.

// urlScheme returns the lower-cased scheme of a URL, or "" when the value has
// none (a relative path). Per RFC 3986 a scheme is `ALPHA *( ALPHA / DIGIT /
// "+" / "-" / "." )`, so "a b:" is a relative path, not a scheme, and must
// not be treated as one.
function urlScheme(u: string): string {
  const i = u.indexOf(':')
  if (i <= 0) return ''
  const s = u.slice(0, i)
  if (!/^[a-zA-Z][a-zA-Z0-9+.-]*$/.test(s)) return ''
  return s.toLowerCase()
}

// Link targets we are willing to put in an href.
const SAFE_HREF_SCHEMES = new Set(['http', 'https', 'mailto'])

// isSafeHref reports whether a link target stays on this site or on a plain
// http(s) endpoint. Relative paths ("/x", "./x", "#frag", "x") are allowed;
// everything else is not.
export function isSafeHref(href: string): boolean {
  const v = (href || '').trim()
  if (!v) return true
  if (v.startsWith('//')) return false // protocol-relative: attacker's host
  if (v.startsWith('\\')) return false // backslash form browsers also treat as "//"
  const scheme = urlScheme(v)
  if (scheme) return SAFE_HREF_SCHEMES.has(scheme)
  return true
}

// isSafeSrc reports whether a resource URL may be fetched. Images and
// posters are fetched even when nothing executes, and each fetch leaks the
// reader's IP / UA / Referer to whoever hosts the URL, so this list is
// narrower than isSafeHref's on purpose: no mailto, and no scheme-less
// protocol-relative form.
const SAFE_SRC_SCHEMES = new Set(['http', 'https'])

export function isSafeSrc(src: string): boolean {
  const v = (src || '').trim()
  if (!v) return true
  if (v.startsWith('//') || v.startsWith('\\')) return false
  const scheme = urlScheme(v)
  if (scheme) return SAFE_SRC_SCHEMES.has(scheme)
  return true // same-origin relative path
}

// isSafeSrcset checks every candidate of a srcset value. A single unsafe
// candidate is enough: the browser picks whichever it likes, so a whitelist
// applied to the whole string proves nothing.
export function isSafeSrcset(srcset: string): boolean {
  const v = (srcset || '').trim()
  if (!v) return true
  for (const cand of v.split(',')) {
    const url = cand.trim().split(/\s+/)[0]
    if (!url) continue
    if (!isSafeSrc(url)) return false
  }
  return true
}

// XSS defense-in-depth: elements removed outright from rendered Markdown.
const DROP_TAGS = new Set([
  'script', 'iframe', 'object', 'embed', 'svg', 'math', 'link', 'style',
  'meta', 'base', 'frame', 'frameset', 'applet', 'form',
])
// Attributes always stripped (event handlers handled separately via on*).
// srcset and poster are NOT in this set: they are URL-valued, so they are
// policy-checked below and blanked when unsafe rather than dropped, matching
// how src and href are treated.
const DROP_ATTRS = new Set(['srcdoc', 'formaction', 'xlink:href', 'http-equiv', 'style', 'background', 'dynsrc', 'lowsrc'])
// Attributes that carry a resource URL and therefore need the src policy.
const URL_ATTRS: Record<string, (v: string) => boolean> = {
  src: isSafeSrc,
  poster: isSafeSrc,
  srcset: isSafeSrcset,
}

export function stripHTML(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  return doc.body.textContent || ''
}

// --- card excerpts --------------------------------------------------------

// Named entities that markdown authors actually type. Decoding is a single
// pass and deliberately not recursive: "&amp;lt;" means the literal text
// "&lt;", so a second pass would be a bug, not a convenience.
const NAMED_ENTITIES: Record<string, string> = {
  amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ', '#39': "'",
}

// plainText flattens a markdown source into the text a list card needs,
// without parsing it.
//
// The list page used to call stripHTML(renderMD(body)) once per card — a full
// markdown parse, plus a DOMParser document, plus a querySelectorAll('*')
// walk over every attribute — ten times on a full page, and then discarded
// everything past the first 120 characters. A regex pass yields the same
// card-visible text for a fraction of the cost and never touches the DOM.
//
// The results follow renderMD's output rather than the raw source: image alt
// text survives and the URL does not (the RSS excerpt makes the same call),
// link text survives and the target does not, inline code keeps its content,
// fenced code blocks and table rules disappear.
export function plainText(src: string | null | undefined): string {
  if (!src) return ''
  let s = String(src)

  // Fenced blocks first: their contents can contain anything, including lines
  // that look like markdown, and none of it belongs on a card.
  s = s.replace(/^[ \t]*(```+|~~~+)[^\n]*\n[\s\S]*?^[ \t]*\1[ \t]*$/gm, ' ')
  // An unterminated fence leaves its opening line behind; drop that too.
  s = s.replace(/^[ \t]*(?:```+|~~~+)[^\n]*$/gm, ' ')

  // Comments, then tag-shaped text only. Requiring a letter after "<" keeps
  // "a < b" in a sentence while <span>...</span> still contributes its text.
  s = s.replace(/<!--[\s\S]*?-->/g, ' ')
  s = s.replace(/<\/?[a-zA-Z][^>]*>/g, '')

  // Images keep their alt text; links keep their label.
  s = s.replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
  s = s.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
  s = s.replace(/\[([^\]]*)\]\[[^\]]*\]/g, '$1')

  // Table separator rows carry no text at all, and horizontal rules must be
  // consumed before bullets get a chance to read "- - -" as a list item.
  s = s.replace(/^[ \t]*\|?[ \t]*:?-{2,}:?[ \t]*(?:\|[ \t]*:?-{2,}:?[ \t]*)+\|?[ \t]*$/gm, ' ')
  s = s.replace(/^[ \t]{0,3}(?:[-*_][ \t]*){3,}$/gm, ' ')
  s = s.replace(/^[ \t]{0,3}(?:#{1,6}[ \t]+|>[ \t]?|[-*+][ \t]+|\d+[.)][ \t]+)/gm, '')

  // Emphasis markers. Backticks mean nothing outside code and asterisks mean
  // nothing outside emphasis. Underscores are left alone when they sit inside
  // a word: snake_case identifiers are far more common here than _emphasis_.
  s = s.replace(/`+/g, '')
  s = s.replace(/\*\*|__|~~/g, '')
  s = s.replace(/\*/g, '')
  s = s.replace(/(^|[\s(（])_([^_\n]+)_(?=$|[\s)）,.!?，。！？])/g, '$1$2')

  // Pipes become spaces rather than gluing adjacent cells into one word.
  s = s.replace(/\|/g, ' ')

  // Entities decode after tags are gone. Decoding first would turn &lt;b&gt;
  // into a tag that the line above would then delete.
  s = s.replace(/&(amp|lt|gt|quot|apos|nbsp|#39);/g, (m, name) => NAMED_ENTITIES[name] ?? m)

  s = s.replace(/[ \t\u00a0]+/g, ' ')
  s = s.replace(/\s*\n\s*/g, ' ')
  return s.replace(/^[\s.,;:!?，。；：！？、\-–—]+|[\s.,;:!?，。；：！？、\-–—]+$/g, '')
}

// excerpt is plainText capped to `max` characters plus an ellipsis. The cut is
// by code point rather than UTF-16 unit, so an emoji is never sliced in half
// and rendered as a replacement box.
export function excerpt(src: string | null | undefined, max = 120): string {
  const text = plainText(src)
  if (!text) return ''
  const chars = Array.from(text)
  if (chars.length <= max) return text
  return chars.slice(0, max).join('').trimEnd() + '…'
}

// Escape the 5 HTML-unsafe characters so a string can be embedded as text.
export function escapeHTML(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

// Wrap every occurrence of `query` inside `text` in <mark class="hl">.
// `query` is matched case-insensitively. The input is HTML-escaped FIRST, on
// every branch — including the "no query" one, which used to return `text`
// verbatim and made this function an XSS gadget for any caller that feeds the
// result to innerHTML (the list page did exactly that for titles and
// excerpts). Callers may now inject the result into an innerHTML element
// without pre-escaping.
export function highlight(text: string, query: string): string {
  const escapedText = escapeHTML(text || '')
  const q = (query || '').trim()
  if (!q) return escapedText
  const escapedQuery = escapeHTML(q)
  // Build a global, case-insensitive regex from the escaped query.
  // The query is already HTML-escaped, so any special chars are literal.
  const re = new RegExp(escapedQuery.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi')
  return escapedText.replace(re, (m) => `<mark class="hl">${m}</mark>`)
}

// Count words / CJK characters in a Markdown source string.
// CJK: count each character as one word; non-CJK: split on whitespace.
export function wordCount(src: string | null | undefined): number {
  if (!src) return 0
  const text = stripHTML(renderMD(src))
  const re = /[\u4e00-\u9fff\u3400-\u4dbf\uf900-\ufaff]|\S+/g
  let count = 0
  while (re.exec(text) !== null) count++
  return count
}

// --- anchor / TOC helpers (shared between renderMD and extractTOC) --------

// Slugs are deterministic so TOC links and DOM ids always line up.
function slugify(s: string): string {
  return s
    .toLowerCase()
    .replace(/["'<>&|\\`/]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60)
}

// Pure function: given a list of heading texts, returns matching slug ids.
// No module-level mutable state — each call is independent and deterministic,
// so renderMD and extractTOC always produce identical slugs for the same input.
function buildSlugs(texts: string[]): string[] {
  const local = new Set<string>()
  return texts.map((raw) => {
    let base = slugify(raw)
    if (!base) base = 'section'
    let candidate = base
    let n = 2
    while (local.has(candidate)) {
      candidate = `${base}-${n}`
      n++
    }
    local.add(candidate)
    return candidate
  })
}

export interface TOCEntry {
  id: string
  text: string
  depth: 2 | 3
}

// Parse `##` / `###` headings out of the same Markdown source, producing
// the same slugs renderMD assigns to the DOM. Callers get a TOC list that
// can be rendered before the article body.
export function extractTOC(src: string | null | undefined): TOCEntry[] {
  if (!src) return []
  const lines = src.split('\n')
  const headings: { text: string; depth: 2 | 3 }[] = []
  let inFence = false
  for (const line of lines) {
    if (/^\s*(```|~~~)/.test(line)) {
      inFence = !inFence
      continue
    }
    if (inFence) continue
    const m = /^(#{2,3})\s+(.+?)\s*#*\s*$/.exec(line)
    if (!m) continue
    headings.push({ text: m[2].trim(), depth: m[1].length as 2 | 3 })
  }
  const slugs = buildSlugs(headings.map((h) => h.text))
  return headings.map((h, i) => ({
    id: slugs[i],
    text: h.text,
    depth: h.depth,
  }))
}

export function renderMD(src: string | null | undefined): string {
  if (!src) return ''
  let html: unknown
  try {
    html = marked.parse(src)
  } catch {
    return ''
  }
  if (typeof html !== 'string') return ''

  const doc = new DOMParser().parseFromString(html, 'text/html')
  for (const s of Array.from(doc.body.querySelectorAll('script'))) s.remove()
  // Strip dangerous elements + inline event handlers / dangerous attrs that
  // `marked` may pass through from raw HTML in the source.
  for (const el of Array.from(doc.body.querySelectorAll('*'))) {
    if (DROP_TAGS.has(el.tagName.toLowerCase())) {
      el.remove()
      continue
    }
    for (const attr of Array.from(el.attributes)) {
      const n = attr.name.toLowerCase()
      if (n.startsWith('on') || DROP_ATTRS.has(n)) {
        el.removeAttribute(attr.name)
        continue
      }
      // URL-valued attributes (src / poster / srcset) get the scheme policy
      // instead of a blanket drop, so a legitimate relative image survives
      // while an absolute one to an attacker's host is blanked.
      const check = URL_ATTRS[n]
      if (check && !check(attr.value)) {
        el.removeAttribute(attr.name)
      }
    }
  }
  for (const i of Array.from(doc.body.querySelectorAll('img'))) {
    const v = i.getAttribute('src') || ''
    if (!isSafeSrc(v)) {
      i.setAttribute('src', '')
      i.classList.add('md-img-broken')
    }
  }
  for (const a of Array.from(doc.body.querySelectorAll('a'))) {
    const v = a.getAttribute('href') || ''
    if (!isSafeHref(v)) a.setAttribute('href', '')
    // target=_blank hands the opened page a window.opener back-reference
    // (reverse tabnabbing: it can navigate this tab to a look-alike login).
    // Removing the attribute is the only fix that does not depend on also
    // remembering to set rel — so links open in this tab, which is the safe
    // default for a reader-facing article anyway.
    if (a.hasAttribute('target')) a.removeAttribute('target')
  }
  // Wrap tables in a scroll container so wide tables scroll horizontally
  // without breaking the layout; the table itself stays display:table.
  for (const t of Array.from(doc.body.querySelectorAll('table'))) {
    if (t.parentElement?.classList.contains('table-scroll')) continue
    const wrap = doc.createElement('div')
    wrap.className = 'table-scroll'
    t.replaceWith(wrap)
    wrap.appendChild(t)
  }
  // Assign stable anchor ids to h2/h3 so the TOC can target them.
  const heads = Array.from(doc.body.querySelectorAll('h2, h3'))
  const slugs = buildSlugs(heads.map((h) => h.textContent || ''))
  heads.forEach((h, i) => {
    h.id = slugs[i]
  })
  return doc.body.innerHTML
}
