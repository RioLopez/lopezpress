import { marked } from 'marked'

marked.setOptions({ gfm: true, breaks: true })

const SAFE_SRC = (src: string): boolean =>
  !!src && !src.startsWith('data:') && !src.startsWith('javascript:')

const SAFE_HREF = (href: string): boolean => {
  const re = new RegExp('^(https?:|mailto:|/|#|\\./)', 'i')
  return re.test(href)
}

// XSS defense-in-depth: elements removed outright from rendered Markdown.
const DROP_TAGS = new Set([
  'script', 'iframe', 'object', 'embed', 'svg', 'math', 'link', 'style',
  'meta', 'base', 'frame', 'frameset', 'applet', 'form',
])
// Attributes always stripped (event handlers handled separately via on*).
const DROP_ATTRS = new Set(['srcdoc', 'formaction', 'xlink:href', 'http-equiv', 'style', 'background', 'dynsrc', 'lowsrc'])

export function stripHTML(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  return doc.body.textContent || ''
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
// `query` is matched case-insensitively. If query is empty or absent,
// returns `text` unchanged. The input is HTML-escaped first, so callers
// may safely inject the result into an element with textContent-free
// innerHTML (the surrounding content is caller-controlled & escaped).
export function highlight(text: string, query: string): string {
  const q = query.trim()
  if (!q || !text) return text
  const escapedText = escapeHTML(text)
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
      if (n.startsWith('on') || DROP_ATTRS.has(n)) el.removeAttribute(attr.name)
    }
  }
  for (const i of Array.from(doc.body.querySelectorAll('img'))) {
    const v = i.getAttribute('src') || ''
    if (!SAFE_SRC(v)) {
      i.setAttribute('src', '')
      i.classList.add('md-img-broken')
    }
  }
  for (const a of Array.from(doc.body.querySelectorAll('a'))) {
    const v = a.getAttribute('href') || ''
    if (!SAFE_HREF(v)) a.setAttribute('href', '')
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
