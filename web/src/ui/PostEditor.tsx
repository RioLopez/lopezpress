import { useEffect, useRef, useState, useCallback, useMemo } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { renderMD, wordCount } from '../core/md'
import { listCategories, listTags, uploadImage } from '../core/api'
import { useAuth } from '../core/auth'
import { isAdmin, isRoot } from '../core/roles'
import { orderCategories, categoryOptionLabel } from '../core/taxonomy'
import { normalizeTagInput, suggestTags } from '../core/tags'
import type { EditorValues, PostStatus, Category, Tag } from '../core/types'

type EditorMode = 'split' | 'ir' | 'source'
const DRAFT_TTL_DAYS = 30
const DRAFT_TTL_MS = DRAFT_TTL_DAYS * 24 * 60 * 60 * 1000
const AUTOSAVE_MS = 2500

const draftKey = (id?: string) => `echo.draft${id ? '.' + id : ''}`

// Spec 22: a post has only two visibilities. The editor's `status` state and
// any persisted browser draft are constrained to this union — draft and
// scheduled no longer exist.
// Spec 24: only root may publish directly; every other role's post enters the
// pending review queue server-side no matter which visibility is chosen here.
type Visibility = PostStatus

interface Draft {
  title: string
  body: string
  status: Visibility
  category: number | ''
  tags: string[]
  pinned: boolean
  cover: string
  ts: number
}

function readDraft(id?: string): Draft | null {
  try {
    const raw = localStorage.getItem(draftKey(id))
    if (!raw) return null
    const d = JSON.parse(raw)
    if (!d || typeof d !== 'object') return null
    if (d.ts && Date.now() - d.ts > DRAFT_TTL_MS) {
      localStorage.removeItem(draftKey(id))
      return null
    }
    return d as Draft
  } catch {
    return null
  }
}

function pruneOldDrafts() {
  const now = Date.now()
  for (let i = localStorage.length - 1; i >= 0; i--) {
    const key = localStorage.key(i)
    if (!key || !key.startsWith('echo.draft.')) continue
    try {
      const d = JSON.parse(localStorage.getItem(key) || '')
      if (d.ts && now - d.ts > DRAFT_TTL_MS) localStorage.removeItem(key)
    } catch {
      /* skip */
    }
  }
}

// Toolbar: wrap / insert markdown at the caret. Reused for split + source.
const TOOLBAR: { label: string; prefix: string; suffix: string; key: string }[] = [
  { label: 'H1', prefix: '\n# ', suffix: '\n', key: 'editor.h1' },
  { label: 'H2', prefix: '\n## ', suffix: '\n', key: 'editor.h2' },
  { label: 'H3', prefix: '\n### ', suffix: '\n', key: 'editor.h3' },
  { label: '**B**', prefix: '**', suffix: '**', key: 'editor.bold' },
  { label: '*I*', prefix: '*', suffix: '*', key: 'editor.italic' },
  { label: '`c`', prefix: '`', suffix: '`', key: 'editor.code' },
  { label: '> ', prefix: '\n> ', suffix: '\n', key: 'editor.blockquote' },
  { label: '- ', prefix: '\n- ', suffix: '\n', key: 'editor.list' },
  { label: '[]', prefix: '[', suffix: '](url)', key: 'editor.link' },
  { label: '![]', prefix: '![', suffix: '](url)', key: 'editor.image' },
]

export interface PostEditorProps {
  id?: string
  initialTitle?: string
  initialBody?: string
  initialStatus?: PostStatus
  initialReviewNote?: string
  initialCategory?: number
  initialTags?: string[]
  initialPinned?: boolean
  initialCover?: string
  saveLabel: string
  publishing?: boolean
  onCancel: () => void
  onSubmit: (v: EditorValues) => void
}

export function PostEditor({
  id,
  initialTitle = '',
  initialBody = '',
  initialStatus = 'published',
  initialReviewNote = '',
  initialCategory,
  initialTags,
  initialPinned,
  initialCover = '',
  saveLabel,
  publishing = false,
  onCancel,
  onSubmit,
}: PostEditorProps) {
  const { t } = useI18n()
  const { user } = useAuth()
  // Spec 24: only root publishes straight to "published"; everyone else's
  // post enters the review queue server-side.
  const isRootUser = isRoot(user?.role)
  // Pinning floats a post to the top of every public list for every reader,
  // so it is an editorial decision: the server refuses it from a plain member
  // and the control is hidden here to match (no button that 403s).
  const canPin = isAdmin(user?.role)
  const [title, setTitle] = useState(initialTitle)
  // Spec 22: default to public ("published") when no initial status supplied.
  // Any legacy draft/scheduled value is coerced back to public so a stale
  // record can never produce a hidden or scheduled post.
  const [status, setStatus] = useState<PostStatus>(
    initialStatus === 'private' || initialStatus === 'rejected' || initialStatus === 'pending'
      ? initialStatus
      : 'published',
  )
  // The reviewer's note for a rejected post — shown to the author so they
  // know what to fix before resubmitting (closes the review loop).
  const [reviewNote] = useState(initialReviewNote)
  const [category, setCategory] = useState<number | ''>(initialCategory ?? '')
  const [tags, setTags] = useState<string[]>((initialTags || []).filter(Boolean))
  const [pinned, setPinned] = useState(!!initialPinned && canPin)
  const [cover, setCover] = useState(initialCover)
  const [mode, setMode] = useState<EditorMode>('split')
  const [body, setBody] = useState(initialBody)
  const [categories, setCategories] = useState<Category[]>([])
  const [savedAt, setSavedAt] = useState<number | null>(null)
  // Tag picker state: the site's existing tags (for autocomplete), what the
  // author is typing, whether the dropdown is open, and which row is
  // highlighted for Enter/arrow keys.
  const [allTags, setAllTags] = useState<Tag[]>([])
  const [tagInput, setTagInput] = useState('')
  const [tagOpen, setTagOpen] = useState(false)
  const [tagActive, setTagActive] = useState(0)
  const [coverBusy, setCoverBusy] = useState(false)
  const [splitPct, setSplitPct] = useState(50)
  const [dragging, setDragging] = useState(false)

  const editorHost = useRef<HTMLDivElement>(null)
  const taRef = useRef<HTMLTextAreaElement>(null)
  const previewRef = useRef<HTMLDivElement>(null)
  const bodyRef = useRef(initialBody)
  const autosaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const initialDraft = useRef<Draft | null>(readDraft(id))
  const [showRestore, setShowRestore] = useState(!!initialDraft.current)
  const [restoreDecided, setRestoreDecided] = useState(false)

  const valuesRef = useRef({ title, status, category, tags, pinned, cover })
  valuesRef.current = { title, status, category, tags, pinned, cover }

  useEffect(() => {
    pruneOldDrafts()
    listCategories().then(setCategories).catch(() => {})
    // Tag autocomplete data. Failure is non-fatal: the author can still type
    // a tag by hand and the server will resolve or create it.
    listTags().then(setAllTags).catch(() => {})
  }, [])

  const saveDraft = useCallback(() => {
    if (id === undefined && !bodyRef.current && !valuesRef.current.title) return
    const d: Draft = {
      title: valuesRef.current.title,
      body: bodyRef.current,
      status: valuesRef.current.status,
      category: valuesRef.current.category,
      tags: valuesRef.current.tags,
      pinned: valuesRef.current.pinned,
      cover: valuesRef.current.cover,
      ts: Date.now(),
    }
    try {
      localStorage.setItem(draftKey(id), JSON.stringify(d))
      setSavedAt(d.ts)
    } catch {
      /* quota */
    }
  }, [id])

  const scheduleAutosave = useCallback(() => {
    if (showRestore && !restoreDecided) return
    if (autosaveTimer.current) clearTimeout(autosaveTimer.current)
    autosaveTimer.current = setTimeout(saveDraft, AUTOSAVE_MS)
  }, [showRestore, restoreDecided, saveDraft])

  // Cleanup timer on unmount.
  useEffect(() => () => { if (autosaveTimer.current) clearTimeout(autosaveTimer.current) }, [])

  const setBodyValue = useCallback(
    (val: string) => {
      bodyRef.current = val
      setBody(val)
      scheduleAutosave()
    },
    [scheduleAutosave],
  )

  const insertAtCursor = (prefix: string, suffix = '') => {
    const ta = taRef.current
    if (!ta) return
    const start = ta.selectionStart
    const end = ta.selectionEnd
    const selected = bodyRef.current.slice(start, end)
    const replacement = prefix + selected + suffix
    const next = bodyRef.current.slice(0, start) + replacement + bodyRef.current.slice(end)
    setBodyValue(next)
    requestAnimationFrame(() => {
      ta.focus()
      if (selected) {
        const from = start + prefix.length
        ta.setSelectionRange(from, from + selected.length)
      } else {
        const pos = start + prefix.length
        ta.setSelectionRange(pos, pos)
      }
    })
  }

  const insertImageMarkdown = (url: string) => insertAtCursor(`![image](${url})`)

  // Paste / drop image upload (split + source modes).
  const handleFiles = async (files: FileList | File[]) => {
    const imgs = Array.from(files).filter((f) => f.type.startsWith('image/'))
    for (const f of imgs) {
      try {
        const r = await uploadImage(f)
        insertImageMarkdown(r.url)
      } catch {
        /* ignore */
      }
    }
  }

  const onPaste = (e: ClipboardEvent) => {
    if (mode === 'ir') return
    const items = e.clipboardData?.items
    if (!items) return
    const hasImage = Array.from(items).some((it) => it.type.startsWith('image/'))
    if (!hasImage) return
    e.preventDefault()
    const files: File[] = []
    for (const it of Array.from(items)) {
      if (it.type.startsWith('image/')) {
        const f = it.getAsFile()
        if (f) files.push(f)
      }
    }
    handleFiles(files)
  }

  const onDrop = (e: DragEvent) => {
    if (mode === 'ir') return
    if (e.dataTransfer?.files?.length) {
      e.preventDefault()
      handleFiles(e.dataTransfer.files)
    }
    setDragging(false)
  }

  // Split: two-way scroll sync.
  const onTaScroll = () => {
    const ta = taRef.current
    const pv = previewRef.current
    if (!ta || !pv) return
    const ratio = ta.scrollTop / (ta.scrollHeight - ta.clientHeight || 1)
    pv.scrollTop = ratio * (pv.scrollHeight - pv.clientHeight)
  }
  const onPreviewScroll = () => {
    const ta = taRef.current
    const pv = previewRef.current
    if (!ta || !pv) return
    const ratio = pv.scrollTop / (pv.scrollHeight - pv.clientHeight || 1)
    ta.scrollTop = ratio * (ta.scrollHeight - ta.clientHeight)
  }

  // Resizable divider (split mode).
  useEffect(() => {
    if (!dragging) return
    const onMove = (e: MouseEvent) => {
      const host = editorHost.current
      if (!host) return
      const rect = host.getBoundingClientRect()
      const pct = ((e.clientX - rect.left) / rect.width) * 100
      setSplitPct(Math.min(80, Math.max(20, pct)))
    }
    const onUp = () => setDragging(false)
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }, [dragging])

  // Inline formatting shortcuts; yield to IME composition.
  const onKeyDown = (e: KeyboardEvent) => {
    if ((e as any).isComposing) return
    if (!(e.ctrlKey || e.metaKey) || e.shiftKey) return
    const k = e.key.toLowerCase()
    if (k === 'b') {
      e.preventDefault()
      insertAtCursor('**', '**')
    } else if (k === 'i') {
      e.preventDefault()
      insertAtCursor('*', '*')
    }
  }

  // Document-level shortcuts: Ctrl/Cmd+S = save (browser draft when creating,
  // explicit save when editing), Ctrl/Cmd+Enter = submit with current visibility.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return
      if ((e as any).isComposing) return
      if (e.key === 's' && !e.shiftKey) {
        e.preventDefault()
        saveDraft()
      } else if (e.key === 'Enter') {
        e.preventDefault()
        doSubmit()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  const restoreDraft = () => {
    const d = initialDraft.current
    if (d) {
      setTitle(d.title || '')
      // Coerce any legacy draft/scheduled value to public.
      setStatus(d.status === 'private' ? 'private' : 'published')
      setCategory(d.category ?? '')
      setTags((d.tags || []).filter(Boolean))
      setPinned(!!d.pinned && canPin)
      setCover(d.cover || '')
      setBodyValue(d.body || '')
    }
    setShowRestore(false)
    setRestoreDecided(true)
  }
  const discardDraft = () => {
    localStorage.removeItem(draftKey(id))
    initialDraft.current = null
    setShowRestore(false)
    setRestoreDecided(true)
  }

  // --- tag picker -----------------------------------------------------------
  // The dropdown mirrors the site's existing tags so an author never has to
  // remember whether the site spells it "Go" or "go". Matching is delegated to
  // core/tags (pure + unit-tested); this component only handles focus and keys.
  const tagSuggestions = useMemo(
    () => (tagOpen ? suggestTags(allTags, tagInput, tags) : []),
    [tagOpen, allTags, tagInput, tags],
  )

  // Close the dropdown on an outside click rather than on input blur. A blur
  // timer raced with typing (the timer fired just after the next keystroke
  // reopened the list, so the list vanished while the author was still typing)
  // and it also fired when the click landed on an option. A document-level
  // mousedown scoped to the box has neither problem.
  const tagBoxRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!tagOpen) return
    const onDown = (e: MouseEvent) => {
      if (tagBoxRef.current && !tagBoxRef.current.contains(e.target as Node)) setTagOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [tagOpen])

  const addTag = (raw: string) => {
    const name = normalizeTagInput(raw.replace(/,/g, ' '))
    if (!name) return
    // Reuse the stored spelling when one exists: attaching "go" to the "Go"
    // tag is what keeps one concept from splitting into three.
    const existing = allTags.find((t) => t.name.toLowerCase() === name.toLowerCase())
    const final = existing ? existing.name : name
    setTags((prev) =>
      prev.some((t) => t.toLowerCase() === final.toLowerCase()) ? prev : [...prev, final],
    )
    setTagInput('')
    setTagActive(0)
    setTagOpen(false)
  }

  const pickSuggestion = (index: number) => {
    const picked = tagSuggestions[index]
    if (picked) addTag(picked.name)
    else addTag(tagInput)
  }

  const onTagKey = (e: KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        if (!tagOpen && tagInput.trim()) { setTagOpen(true); setTagActive(0); return }
        setTagActive((i) => Math.min(i + 1, Math.max(0, tagSuggestions.length - 1)))
        return
      case 'ArrowUp':
        e.preventDefault()
        setTagActive((i) => Math.max(0, i - 1))
        return
      case 'Escape':
        // Close the dropdown without touching what was typed.
        if (tagOpen) { e.preventDefault(); setTagOpen(false) }
        return
      case 'Enter':
      case 'Tab':
        e.preventDefault()
        // Enter takes the highlighted suggestion (a create-row is part of the
        // suggestion list, so "type a new name + Enter" still creates it).
        if (tagOpen && tagSuggestions.length) pickSuggestion(tagActive)
        else addTag(tagInput)
        return
      case ',':
        e.preventDefault()
        addTag(tagInput)
        return
      case ' ':
        // Space only commits when it cannot be part of a name: multi-word tags
        // like "web dev" are legitimate, so a space mid-typing stays literal.
        if (tagOpen && tagSuggestions.length) {
          e.preventDefault()
          pickSuggestion(tagActive)
        }
        return
      case 'Backspace':
        if (!tagInput && tags.length) {
          e.preventDefault()
          setTags(tags.slice(0, -1))
        }
        return
      default:
        return
    }
  }

  const pickCoverFromBody = () => {
    const m = bodyRef.current.match(/!\[[^\]]*\]\((https?:\/\/[^)\s]+|[^)\s]+)\)/)
    if (m) setCover(m[1])
  }
  const uploadCover = async (file: File) => {
    setCoverBusy(true)
    try {
      const r = await uploadImage(file)
      setCover(r.url)
    } catch {
      /* ignore */
    } finally {
      setCoverBusy(false)
    }
  }

  const doSubmit = () => {
    if (!title.trim()) {
      document.querySelector<HTMLInputElement>('.pe-title')?.focus()
      return
    }
    // A rejected post is resubmitted to the review queue (status "pending"),
    // never republished by its author. Non-rejected posts keep the chosen
    // visibility (private, or published straight to the queue for non-root).
    const effectiveStatus: PostStatus = status === 'rejected' ? 'pending' : status
    onSubmit({
      title: title.trim(),
      body: bodyRef.current,
      status: effectiveStatus,
      category_id: category !== '' ? Number(category) : undefined,
      tags: tags.length ? tags : undefined,
      pinned,
      cover: cover || undefined,
    })
  }

  const wc = wordCount(body)
  const readMin = wc ? Math.max(1, Math.round(wc / 300)) : 0
  // New post -> "发布" (publish); existing post -> "更新" (update). No
  // "save draft" variant: drafts were removed in spec 22.
  const saveBtnLabel = status === 'rejected' ? t('editor.resubmit') : id ? t('editor.publishUpdate') : t('editor.publish')
  const showToolbar = mode !== 'ir'

  return (
    <div class="post-editor">
      {showRestore && (
        <div class="pe-restore">
          <span>{t('editor.restoreDraft')}</span>
          <button class="btn btn-primary btn-sm" onClick={restoreDraft}>
            {t('editor.restoreDraftConfirm')}
          </button>
          <button class="btn btn-ghost btn-sm" onClick={discardDraft}>
            {t('editor.restoreDraftDiscard')}
          </button>
        </div>
      )}

      {/* Row 1: page title (left) + publish action (right). Page-level ops live here,
          not in the editor toolbar. */}
      <div class="pe-topbar">
        <h1 class="pe-heading">{t('editor.heading')}</h1>
        <button
          class={`btn btn-primary pe-savebtn ${publishing ? 'pe-publishing' : ''}`}
          disabled={publishing}
          onClick={() => doSubmit()}
        >
          {publishing
            ? t('editor.publishing')
            : status === 'rejected'
              ? t('editor.resubmit')
              : saveLabel || saveBtnLabel}
        </button>
      </div>

      {/* Row 2: full-width post title input. */}
      <input
        class="pe-title"
        value={title}
        placeholder={t('editor.titlePlaceholder')}
        onInput={(e) => {
          setTitle((e.target as HTMLInputElement).value)
          scheduleAutosave()
        }}
      />

      {/* Row 3: editor toolbar (formatting, left) + mode switch (right). */}
      <div class="pe-toolbar">
        {showToolbar && (
          <div class="pe-tools">
            {TOOLBAR.map((b) => (
              <button
                key={b.label}
                class="tool-btn"
                title={t(b.key)}
                onClick={() => insertAtCursor(b.prefix, b.suffix)}
              >
                {b.label}
              </button>
            ))}
          </div>
        )}
        <div class="pe-modes" role="group" aria-label="editor mode">
          <button class={`pe-mode ${mode === 'split' ? 'pe-mode--active' : ''}`} onClick={() => setMode('split')}>
            {t('editor.modeSplit')}
          </button>
          <button class={`pe-mode ${mode === 'ir' ? 'pe-mode--active' : ''}`} onClick={() => setMode('ir')}>
            {t('editor.modeIR')}
          </button>
          <button class={`pe-mode ${mode === 'source' ? 'pe-mode--active' : ''}`} onClick={() => setMode('source')}>
            {t('editor.modeSource')}
          </button>
        </div>
      </div>

      <div class="pe-grid">
        <div
          class={`pe-editor ${dragging ? 'pe-dragging' : ''}`}
          ref={editorHost}
          onDragOver={(e) => {
            if (mode === 'ir') return
            e.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
        >
          {mode === 'split' && (
            <div class="pe-split" style={`grid-template-columns: ${splitPct}% 1fr`}>
              <textarea
                ref={taRef}
                class="pe-ta"
                value={body}
                placeholder={t('editor.bodyPlaceholder')}
                onInput={(e) => setBodyValue((e.target as HTMLTextAreaElement).value)}
                onScroll={onTaScroll}
                onKeyDown={onKeyDown}
                onPaste={onPaste}
              />
              <div class="pe-preview" ref={previewRef} onScroll={onPreviewScroll}>
                {body ? (
                  <div class="md-body" dangerouslySetInnerHTML={{ __html: renderMD(body) }} />
                ) : (
                  <p class="muted">{t('post.emptyBody')}</p>
                )}
              </div>
              <div class="pe-divider" style={`left:${splitPct}%`} onMouseDown={() => setDragging(true)} title={t('editor.dragDivider')} />
            </div>
          )}

          {mode === 'source' && (
            <textarea
              ref={taRef}
              class="pe-ta pe-ta--full"
              value={body}
              placeholder={t('editor.bodyPlaceholder')}
              onInput={(e) => setBodyValue((e.target as HTMLTextAreaElement).value)}
              onKeyDown={onKeyDown}
              onPaste={onPaste}
            />
          )}

          {mode === 'ir' && (
            <div class="pe-ir">
              {body ? (
                <div class="md-body" dangerouslySetInnerHTML={{ __html: renderMD(body) }} />
              ) : (
                <p class="muted">{t('post.emptyBody')}</p>
              )}
              <p class="pe-pinned-hint">{t('editor.modeIR')}：实时预览（只读）。编辑请切到「分屏预览」或「纯源码」。</p>
            </div>
          )}

          <div class="pe-statline">
            <span>{t('editor.stats', { n: String(wc) })}</span>
            {readMin > 0 && <span>· {t('editor.readTime', { n: String(readMin) })}</span>}
            {savedAt && (
              <span class="pe-saved">{t('editor.savedAt', { time: new Date(savedAt).toLocaleTimeString() })}</span>
            )}
            <span class="pe-statline-cancel">
              <button class="btn btn-ghost btn-sm" onClick={onCancel}>
                {t('editor.cancel')}
              </button>
            </span>
          </div>
        </div>

        <aside class="pe-meta">
          <h3>{t('editor.metaPanel')}</h3>

          <div class="pe-field">
            <label>{t('editor.category')}</label>
            <select
              value={String(category)}
              onChange={(e) =>
                setCategory((e.target as HTMLSelectElement).value === '' ? '' : Number((e.target as HTMLSelectElement).value))
              }
            >
              <option value="">{t('editor.allCategories')}</option>
              {/* Same indented tree as the public filter: filing a post must
                  show which node is a parent and which is a leaf. */}
              {orderCategories(categories).map((node) => (
                <option key={node.cat.id} value={String(node.cat.id)}>
                  {categoryOptionLabel(node)}
                </option>
              ))}
            </select>
          </div>

          <div class="pe-field">
            <label>{t('editor.tags')}</label>
            <div
              ref={tagBoxRef}
              class={`pe-tags-input ${tagOpen && tagSuggestions.length ? 'pe-tags-input--open' : ''}`}
            >
              {tags.map((tg) => (
                <span class="pe-tag" key={tg}>
                  {tg}
                  <button
                    type="button"
                    aria-label={`${t('editor.tagRemove')} ${tg}`}
                    onClick={() => setTags(tags.filter((x) => x !== tg))}
                  >
                    ×
                  </button>
                </span>
              ))}
              <input
                class="pe-tag-input"
                value={tagInput}
                placeholder={tags.length ? '' : t('editor.tagPlaceholder')}
                onInput={(e) => {
                  setTagInput((e.target as HTMLInputElement).value)
                  setTagOpen(true)
                  setTagActive(0)
                }}
                onFocus={() => { if (tagInput.trim()) setTagOpen(true) }}
                onKeyDown={onTagKey}
              />
              {tagOpen && tagSuggestions.length > 0 && (
                <ul class="pe-tag-menu">
                  {tagSuggestions.map((s, i) => (
                    <li
                      key={`${s.exists ? 'e' : 'n'}:${s.name}`}
                      class={[
                        'pe-tag-option',
                        i === tagActive ? 'pe-tag-option--active' : '',
                        s.exists ? '' : 'pe-tag-option--create',
                      ].join(' ')}
                      // mousedown (not click) so the input never loses focus
                      // before the choice is committed.
                      onMouseDown={(e) => { e.preventDefault(); pickSuggestion(i) }}
                      onMouseEnter={() => setTagActive(i)}
                    >
                      {s.exists ? (
                        <span class="pe-tag-option-name">{s.name}</span>
                      ) : (
                        <>
                          <span class="pe-tag-option-plus">+</span>
                          <span class="pe-tag-option-label">{t('editor.tagCreate')}</span>
                          <span class="pe-tag-option-name">{s.name}</span>
                        </>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <p class="pe-tags-hint">{t('editor.tagHint')}</p>
          </div>

          <div class="pe-field">
            <label>{t('editor.visibility')}</label>
            {status === 'rejected' ? (
              // A rejected post is in review-limbo: the author sees the
              // reviewer's note, revises, then resubmits from the top bar.
              // The visibility radios are hidden because the post cannot be
              // made public or private until it clears review again.
              <div class="pe-rejected-banner">
                <p class="pe-rejected-title">{t('editor.rejectedBanner')}</p>
                {reviewNote && (
                  <p class="pe-rejected-note">
                    <strong>{t('reviewNote')}</strong> {reviewNote}
                  </p>
                )}
                <p class="pe-pinned-hint pe-review-hint">{t('editor.visibilityReviewHint')}</p>
              </div>
            ) : (
              <div class="pe-visibility" role="radiogroup" aria-label={t('editor.visibility')}>
                <label class={`pe-visibility-opt ${status === 'published' ? 'is-active' : ''}`}>
                  <input
                    type="radio"
                    name="pe-visibility"
                    checked={status === 'published'}
                    onChange={() => setStatus('published')}
                  />
                  <span class="pe-visibility-dot pe-visibility-dot--public"></span>
                  <span>{t('editor.visibilityPublic')}</span>
                </label>
                <label class={`pe-visibility-opt ${status === 'private' ? 'is-active' : ''}`}>
                  <input
                    type="radio"
                    name="pe-visibility"
                    checked={status === 'private'}
                    onChange={() => setStatus('private')}
                  />
                  <span class="pe-visibility-dot pe-visibility-dot--private"></span>
                  <span>{t('editor.visibilityPrivate')}</span>
                </label>
              </div>
            )}
            <p class="pe-pinned-hint">
              {status === 'private' ? t('editor.visibilityPrivateHint') : t('editor.visibilityPublicHint')}
            </p>
            {/* Spec 24: non-root posts always queue for review, so surface
                that up front instead of letting the author wonder why the
                post never went public. */}
            {!isRootUser && (
              <p class="pe-pinned-hint pe-review-hint">{t('editor.visibilityReviewHint')}</p>
            )}
          </div>

          {canPin && (
            <div class="pe-field">
              <label class="pe-pinned">
                <input
                  type="checkbox"
                  checked={pinned}
                  onChange={(e) => setPinned((e.target as HTMLInputElement).checked)}
                />
                <span>{t('editor.pinnedLabel')}</span>
              </label>
              <p class="pe-pinned-hint">{t('editor.pinnedHint')}</p>
            </div>
          )}

          <div class="pe-field">
            <label>{t('editor.cover')}</label>
            {cover ? (
              <div class="pe-cover-preview">
                <img src={cover} alt="cover" />
                <button class="btn btn-ghost btn-sm" onClick={() => setCover('')}>
                  {t('editor.coverClear')}
                </button>
              </div>
            ) : (
              <div class="pe-cover-actions">
                <label class="btn btn-ghost btn-sm" style="cursor:pointer">
                  {coverBusy ? t('editor.publishing') : t('editor.coverUpload')}
                  <input
                    type="file"
                    accept="image/*"
                    style="display:none"
                    onChange={(e) => {
                      const f = (e.target as HTMLInputElement).files?.[0]
                      if (f) uploadCover(f)
                    }}
                  />
                </label>
                <button class="btn btn-ghost btn-sm" onClick={pickCoverFromBody}>
                  {t('editor.coverFromBody')}
                </button>
              </div>
            )}
          </div>
        </aside>
      </div>
    </div>
  )
}
