import { useEffect, useRef, useState } from 'preact/hooks'
import type Vditor from 'vditor'
import { isSafeHref } from '../core/md'

// The Markdown editor used by the post composer.
//
// Vditor owns the editing surface; everything around it (title, cover, tags,
// visibility, drafts, submit) stays in PostEditor, because that shell carries
// the domain rules and rewriting it is how bug-per-fix ratios explode.
//
// Four things here are deliberate and easy to break:
//
// 1. The library is imported dynamically. Vditor plus its Markdown engine is
//    about a megabyte of gzipped JavaScript, and readers must never pay for
//    it - only the author, and only once they open the composer.
//
// 2. `cdn` points at /vditor, which the Vite plugin in vite.config.ts copies
//    out of node_modules. The default is unpkg, which would make the editor
//    depend on a third-party CDN being reachable from the author's network,
//    and fail to boot at all when it is not.
//
// 3. `cache` is disabled. Vditor keeps its own copy of the draft in
//    localStorage; PostEditor already owns a draft format with a TTL and a
//    restore prompt (the `echo.draft.*` keys). Two writers for one document
//    means the stale one eventually wins silently, so the library's is off.
//
// 4. Layout changes between "split" and "instant rendering" rebuild the
//    instance. Vditor 4.x exposes getCurrentMode() but no setMode(), so the
//    only way across that boundary is a fresh mount. Content is carried over
//    through valueRef, so nothing is lost except the undo stack - which is
//    the right trade for a mode switch.

/** What the composer offers; mapped onto Vditor's own layouts. */
export type EditorMode = 'split' | 'ir' | 'source'

export interface VditorEditorProps {
  value: string
  onChange: (value: string) => void
  mode: EditorMode
  /** Vditor's locale id. */
  lang: 'zh_CN' | 'en_US'
  placeholder?: string
  /** Upload one pasted or dropped image and report where it landed. */
  onUploadImage: (file: File) => Promise<{ url: string; name: string }>
  /** Surface an upload failure; the editor itself cannot explain it. */
  onUploadError?: (message: string) => void
}

// Both "split" and "source" are the sv layout; they differ only in whether the
// preview pane is shown, which setPreviewMode controls without rebuilding.
const LAYOUT: Record<EditorMode, 'sv' | 'ir'> = {
  split: 'sv',
  ir: 'ir',
  source: 'sv',
}

// Toolbar limited to text authoring. Every renderer NOT listed here (mermaid,// echarts, mathjax, katex, graphviz, markmap, abcjs, wavedrom, plantuml) is a
// runtime download that only happens if its button or an options flag asks for
// it - leaving them out is what keeps the shipped assets at 4.7 MB instead of
// the package's full 24 MB.
const TOOLBAR = [
  'headings',
  'bold',
  'italic',
  'strike',
  '|',
  'list',
  'ordered-list',
  'check',
  '|',
  'quote',
  'line',
  'code',
  'inline-code',
  '|',
  'link',
  'table',
  'upload',
  '|',
  'undo',
  'redo',
  '|',
  'fullscreen',
]

// Links inside the editor's own preview would otherwise bypass the URL policy
// in core/md.ts, which is what the reader-facing renderer enforces. A post is
// later rendered by a different engine than the one that produced the HTML
// here, so the two have to agree on what counts as a safe target.
//
// DOMParser is inert (no script runs, no image loads) and the string is only
// rebuilt when something actually changed, so the common case costs one scan
// and no re-serialisation.
function stripUnsafeLinks(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  let changed = false
  doc.querySelectorAll('a[href]').forEach((a) => {
    if (!isSafeHref(a.getAttribute('href') ?? '')) {
      a.removeAttribute('href')
      changed = true
    }
  })
  return changed ? doc.body.innerHTML : html
}

// Upload toasts live here rather than in the composer's i18n bundle because
// they belong to the editor surface, and the composer would have to thread a
// second callback through just to render two words.
const TIPS: Record<'zh_CN' | 'en_US', { uploading: string; failed: string }> = {
  zh_CN: { uploading: '图片上传中…', failed: '图片上传失败' },
  en_US: { uploading: 'Uploading image…', failed: 'Image upload failed' },
}

// `![alt](url)` with the alt taken from the original file name, minus the
// extension and minus `[` `]` `\`: a `]` would close the alt text early and
// the rest of the line would then be parsed as body text.
function imageMarkdown(file: File, url: string): string {
  const base = (file.name || '')
    .replace(/\.[^.]+$/, '')
    .replace(/[[\]\\]/g, '')
    .trim()
  return `![${base || 'image'}](${url})`
}

export function VditorEditor(props: VditorEditorProps) {
  const hostRef = useRef<HTMLDivElement>(null)
  const vditorRef = useRef<Vditor | null>(null)
  // Last value exchanged with Vditor. Comparing against this (instead of
  // against props.value) is what stops a feedback loop: input -> onChange ->
  // new prop -> setValue -> input -> ...
  const valueRef = useRef(props.value)
  // Callbacks are read through a ref so the editor is built once and never has
  // to be torn down just because the parent re-rendered with new closures.
  const propsRef = useRef(props)
  propsRef.current = props
  const [ready, setReady] = useState(false)
  // Bumped to force a rebuild when the layout has to change (see note 4).
  const [generation, setGeneration] = useState(0)

  useEffect(() => {
    const host = hostRef.current
    if (!host) return
    let disposed = false
    const initialMode = propsRef.current.mode
    setReady(false)

    void (async () => {
      // Both requests start together: the stylesheet and the library are
      // useless without the other, so sequencing them only delays paint.
      const [mod] = await Promise.all([import('vditor'), import('vditor/dist/index.css')])
      if (disposed) return

      const VditorCtor = mod.default
      const vditor = new VditorCtor(host, {
        cdn: '/vditor',
        lang: propsRef.current.lang,
        icon: 'material',
        theme: 'classic',
        mode: LAYOUT[initialMode],
        value: valueRef.current,
        placeholder: propsRef.current.placeholder ?? '',
        height: '100%',
        minHeight: 360,
        toolbar: TOOLBAR,
        // The composer prints its own word count and read time below the
        // editor (.pe-statline); the library's counter would be a second one.
        counter: { enable: false },
        outline: { enable: false, position: 'left' },
        cache: { enable: false },
        preview: {
          theme: { current: 'ant-design' },
          hljs: { style: 'github', lineNumber: false },
          // Deliberately no `math` / `mermaid` / `markdown` config: each one
          // triggers a runtime download of a renderer this blog does not ship.
          transform: stripUnsafeLinks,
        },
        upload: {
          accept: 'image/png,image/jpeg,image/gif,image/webp',
          // Without this Vditor keeps only the first file of a multi-file drop
          // (uploadFiles computes filesMax = multiple ? files.length : 1).
          multiple: true,
          handler: async (files: File[]) => {
            const v = vditorRef.current
            // null is the typed "insert nothing" answer. Everything below
            // inserts through insertMD instead, because whatever this handler
            // returns as a string is shown as a tip, not inserted.
            if (!v) return null
            const tips = TIPS[propsRef.current.lang]
            for (let i = 0; i < files.length; i++) {
              const file = files[i]
              if (files.length > 1) v.tip(`${i + 1}/${files.length}`, 30000)
              else v.tip(tips.uploading, 30000)
              try {
                const r = await propsRef.current.onUploadImage(file)
                // insertMD, not insertValue, and never a returned string:
                //  - a string RETURN VALUE is read as a tip message and nothing
                //    is inserted (uploadFiles(), `typeof isValidate === "string"`);
                //  - insertValue takes HTML in ir mode, so markdown text would
                //    land in the document as literal `![alt](url)`.
                // insertMD routes through lute per mode: Md2VditorIRDOM for ir,
                // processPaste for sv. That is the only path that works in both.
                v.insertMD(`\n${imageMarkdown(file, r.url)}\n`)
                // insertMD does not fire the `input` callback, so the parent's
                // body state would stay stale and the image would vanish on
                // save. Push the whole document back by hand.
                const next = v.getValue()
                valueRef.current = next
                propsRef.current.onChange(next)
              } catch (e) {
                // Insert nothing for the rest of the batch: a partial result
                // would leave the post referencing images that never landed.
                v.tip(tips.failed, 4000)
                propsRef.current.onUploadError?.(e instanceof Error ? e.message : String(e))
                return null
              }
            }
            return null
          },
        },
        input: (value: string) => {
          valueRef.current = value
          propsRef.current.onChange(value)
        },
        after: () => {
          if (disposed) return
          // sv starts with both panes; "source" is the same layout with the
          // preview hidden.
          if (initialMode === 'source') vditor.setPreviewMode('editor')
          setReady(true)
        },
      })
      vditorRef.current = vditor
    })()

    return () => {
      disposed = true
      vditorRef.current?.destroy()
      vditorRef.current = null
    }
  }, [generation])

  useEffect(() => {
    const v = vditorRef.current
    if (!v || !ready) return

    const target = LAYOUT[props.mode]
    if (v.getCurrentMode() !== target) {
      // Crossing sv <-> ir needs a new instance; content survives via valueRef.
      setGeneration((g) => g + 1)
      return
    }
    // Same layout underneath: split and source differ only by the preview
    // pane, which is a cheap toggle.
    if (target === 'sv') v.setPreviewMode(props.mode === 'source' ? 'editor' : 'both')
  }, [props.mode, ready])

  // External changes (restoring a draft, or the parent resetting the form)
  // have to be pushed in by hand: Vditor is a controlled component in one
  // direction only. Comparing against valueRef keeps the echo of the user's
  // own typing from rewriting the document under the caret.
  useEffect(() => {
    const v = vditorRef.current
    if (!v || !ready) return
    if (props.value !== valueRef.current) {
      valueRef.current = props.value
      v.setValue(props.value)
    }
  }, [props.value, ready])

  return <div class={`vd-host ${props.mode === 'source' ? 'pe-source' : ''}`} ref={hostRef} />
}
