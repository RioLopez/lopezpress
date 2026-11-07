import { useEffect, useRef, useState, useCallback } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { wordCount } from '../core/md'
import { errMessage, listCategories, listTags, uploadImage } from '../core/api'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { isAdmin, isRoot } from '../core/roles'
import type { EditorValues, PostStatus, Category, Tag } from '../core/types'
import { VditorEditor, type EditorMode } from './VditorEditor'
import { EditorSettingsDrawer } from './EditorSettingsDrawer'

const DRAFT_TTL_DAYS = 30
const DRAFT_TTL_MS = DRAFT_TTL_DAYS * 24 * 60 * 60 * 1000
const AUTOSAVE_MS = 2500

const draftKey = (id?: string) => `echo.draft${id ? '.' + id : ''}`

// A post has only two visibilities. The editor's `status` state and
// any persisted browser draft are constrained to this union — draft and
// scheduled no longer exist.
// Only root may publish directly; every other role's post enters the
// pending review queue server-side no matter which visibility is chosen here.
type Visibility = PostStatus

interface Draft {
  title: string
  slug: string
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

// Formatting commands are Vditor's toolbar now. The hand-rolled button list
// that used to live here inserted markdown at a textarea's caret, which meant
// it also had to own caret restoration, selection handling and IME quirks -
// all of it replaced by the editor's own `toolbar` option.

export interface PostEditorProps {
  id?: string
  initialTitle?: string
  initialSlug?: string
  initialBody?: string
  initialStatus?: PostStatus
  initialReviewNote?: string
  initialCategory?: number
  initialTags?: string[]
  initialPinned?: boolean
  initialCover?: string
  publishing?: boolean
  onCancel: () => void
  onSubmit: (v: EditorValues) => void
}

export function PostEditor({
  id,
  initialTitle = '',
  initialSlug = '',
  initialBody = '',
  initialStatus = 'published',
  initialReviewNote = '',
  initialCategory,
  initialTags,
  initialPinned,
  initialCover = '',
  publishing = false,
  onCancel,
  onSubmit,
}: PostEditorProps) {
  const { t, locale } = useI18n()
  const { pushToast } = useApp()
  const { user } = useAuth()
 // Only root publishes straight to "published"; everyone else's
  // post enters the review queue server-side.
  const isRootUser = isRoot(user?.role)
  // Pinning floats a post to the top of every public list for every reader,
  // so it is an editorial decision: the server refuses it from a plain member
  // and the control is hidden here to match (no button that 403s).
  const canPin = isAdmin(user?.role)
  const [title, setTitle] = useState(initialTitle)
  // Empty means "derive from the title" on create and "leave the permalink
  // alone" on edit - the server already draws that distinction.
  const [slug, setSlug] = useState(initialSlug)
 // Default to public ("published") when no initial status supplied.
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
  // The site's existing tags, handed to the settings drawer for autocomplete.
  const [allTags, setAllTags] = useState<Tag[]>([])
  const [coverBusy, setCoverBusy] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const settingsBtnRef = useRef<HTMLButtonElement>(null)

  const bodyRef = useRef(initialBody)
  const autosaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const initialDraft = useRef<Draft | null>(readDraft(id))
  const [showRestore, setShowRestore] = useState(!!initialDraft.current)
  const [restoreDecided, setRestoreDecided] = useState(false)

  const valuesRef = useRef({ title, slug, status, category, tags, pinned, cover })
  valuesRef.current = { title, slug, status, category, tags, pinned, cover }

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
      slug: valuesRef.current.slug,
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

  // Image ingestion for the editor. Vditor hands over the pasted or dropped
  // files and inserts whatever markdown this returns, so the upload endpoint
  // and the dedupe message stay in one place.
  const uploadForEditor = useCallback(
    async (file: File) => {
      const r = await uploadImage(file)
      // Tell the author when the library already had these exact bytes.
      // Nothing new was stored, and saying so avoids "why is my gallery full
      // of the same picture".
      if (r.deduped) pushToast('info', t('imgUpload.deduped'))
      return { url: r.url, name: r.name }
    },
    [pushToast, t],
  )

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
      setSlug(d.slug || '')
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

  // Tag and slug edits are browser-draft material too, so they schedule an
  // autosave the same way the title does.
  const updateTags = (next: string[]) => {
    setTags(next)
    scheduleAutosave()
  }
  const updateSlug = (next: string) => {
    setSlug(next)
    scheduleAutosave()
  }

  const closeSettings = () => {
    setSettingsOpen(false)
    settingsBtnRef.current?.focus()
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
    } catch (e) {
      // Swallowing this left the button flashing "上传中…" and then nothing at
      // all — no cover, no explanation, and no way to tell a rejected file
      // from a server problem.
      pushToast('error', errMessage(e, t('imgUpload.failed')))
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
      // Empty is meaningful on both routes (create: derive from title,
      // edit: keep the current permalink), so it is sent as-is rather than
      // dropped the way an absent field would be.
      slug: slug.trim(),
      body: bodyRef.current,
      status: effectiveStatus,
      category_id: category !== '' ? Number(category) : undefined,
      // The tag set is always sent, empty included: the API reads an absent
      // `tags` as "leave them alone" and `[]` as "remove every tag", so
      // `tags.length ? tags : undefined` made deleting the last tag a no-op.
      tags,
      pinned,
      // Same rule for the cover: "" is the instruction to remove it.
      cover,
    })
  }

  const wc = wordCount(body)
  const readMin = wc ? Math.max(1, Math.round(wc / 300)) : 0
  // The button has to say what will actually happen. For a non-root author,
  // "发布" was a lie: the post goes to the review queue, not to readers. And a
  // private save is not a publish at all. Root is the reviewer, so for root
  // both words stay as they are.
  //
  // A caller-supplied label used to override all of this through
  // `saveLabel || saveBtnLabel` — and since both call sites always passed one,
  // every branch below was dead code: a non-root author saw "发布" on a button
  // that only filed the article for review.
  const wasPublished = initialStatus === 'published'
  const saveBtnLabel = status === 'rejected'
    ? t('editor.resubmit')
    : isRootUser
      ? (id ? t('editor.publishUpdate') : t('editor.publish'))
      : status === 'private'
        ? t('editor.savePrivate')
        : wasPublished ? t('editor.publishUpdateRequeue') : t('editor.submitReview')
  const busyLabel = status === 'private' ? t('editor.saving') : t('editor.publishing')

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

      {/* Row 1: page title (left) + page-level actions (right). Page-level ops
          live here, not in the editor toolbar. */}
      <div class="pe-topbar">
        <h1 class="pe-heading">{t('editor.heading')}</h1>
        <div class="pe-topbar-actions">
          <button
            ref={settingsBtnRef}
            class={`btn btn-ghost pe-settingsbtn ${settingsOpen ? 'is-active' : ''}`}
            onClick={() => setSettingsOpen(true)}
            aria-haspopup="dialog"
            aria-expanded={settingsOpen}
          >
            {t('editor.settings')}
          </button>
          <button
            class={`btn btn-primary pe-savebtn ${publishing ? 'pe-publishing' : ''}`}
            disabled={publishing}
            onClick={() => doSubmit()}
          >
            {publishing ? busyLabel : saveBtnLabel}
          </button>
        </div>
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

      {/* Mode switch only. Vditor renders its own formatting toolbar inside
          the editing area, and a second one up here would be a duplicate
          source of truth for the same commands. */}
      <div class="pe-toolbar">
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

      {/* A rejected post keeps its reviewer note on the canvas: that text is
          the reason the author is editing at all, and hiding it behind a
          drawer would make the banner something they have to go look for. */}
      {status === 'rejected' && (
        <div class="pe-rejected-banner pe-rejected-banner--canvas">
          <p class="pe-rejected-title">{t('editor.rejectedBanner')}</p>
          {reviewNote && (
            <p class="pe-rejected-note">
              <strong>{t('reviewNote')}</strong> {reviewNote}
            </p>
          )}
        </div>
      )}

      <div class="pe-canvas">
        <VditorEditor
          value={body}
          onChange={setBodyValue}
          mode={mode}
          lang={locale === 'zh' ? 'zh_CN' : 'en_US'}
          placeholder={t('editor.bodyPlaceholder')}
          onUploadImage={uploadForEditor}
          onUploadError={(m) => pushToast('error', m)}
        />

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

      <EditorSettingsDrawer
        open={settingsOpen}
        onClose={closeSettings}
        slug={slug}
        onSlug={updateSlug}
        category={category}
        onCategory={setCategory}
        categories={categories}
        tags={tags}
        onTags={updateTags}
        allTags={allTags}
        status={status}
        onStatus={(s) => { setStatus(s); scheduleAutosave() }}
        wasPublished={wasPublished}
        isRootUser={isRootUser}
        canPin={canPin}
        pinned={pinned}
        onPinned={setPinned}
        cover={cover}
        onCover={(v) => { setCover(v); scheduleAutosave() }}
        coverBusy={coverBusy}
        onUploadCover={uploadCover}
        onPickCoverFromBody={pickCoverFromBody}
      />
    </div>
  )
}
