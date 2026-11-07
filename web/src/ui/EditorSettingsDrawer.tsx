import { useEffect, useMemo, useRef, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { orderCategories, categoryOptionLabel } from '../core/taxonomy'
import { normalizeTagInput, suggestTags } from '../core/tags'
import type { Category, PostStatus, Tag } from '../core/types'

// The publish options that used to live in a fixed third column. They are a
// drawer now: the canvas owns the full width at every viewport, which is what
// keeps the tag box from being clipped at 1440 (the fixed 320px column plus the
// admin nav left a few dozen pixels for the field).
//
// Wide screens get a slide-over; below 1024 the panel covers the whole viewport
// (see .pe-drawer in index.css) because a 320px column on a 768 screen would
// repeat the very layout the drawer is meant to replace.

export interface EditorSettingsDrawerProps {
  open: boolean
  onClose: () => void
  slug: string
  onSlug: (v: string) => void
  category: number | ''
  onCategory: (v: number | '') => void
  categories: Category[]
  tags: string[]
  onTags: (v: string[]) => void
  allTags: Tag[]
  status: PostStatus
  onStatus: (v: PostStatus) => void
  wasPublished: boolean
  isRootUser: boolean
  canPin: boolean
  pinned: boolean
  onPinned: (v: boolean) => void
  cover: string
  onCover: (v: string) => void
  coverBusy: boolean
  onUploadCover: (f: File) => void
  onPickCoverFromBody: () => void
}

export function EditorSettingsDrawer(props: EditorSettingsDrawerProps) {
  const { t } = useI18n()
  const {
    open, onClose, slug, onSlug, category, onCategory, categories,
    tags, onTags, allTags, status, onStatus, wasPublished,
    isRootUser, canPin, pinned, onPinned, cover, onCover, coverBusy,
    onUploadCover, onPickCoverFromBody,
  } = props

  const [tagInput, setTagInput] = useState('')
  const [tagOpen, setTagOpen] = useState(false)
  const [tagActive, setTagActive] = useState(0)
  const tagBoxRef = useRef<HTMLDivElement>(null)
  const slugRef = useRef<HTMLInputElement>(null)

  // Focus the first field on open so keyboard users land inside the panel
  // instead of tabbing through the whole canvas behind it.
  useEffect(() => {
    if (open) slugRef.current?.focus()
  }, [open])

  // Escape closes. Registered only while open so the shortcut cannot fire
  // from a screen where there is nothing to close.
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onClose])

  // Close the suggestion list on an outside click, the same reason the old
  // sidebar did (blur timers raced with typing).
  useEffect(() => {
    if (!tagOpen) return
    const onDown = (e: MouseEvent) => {
      if (tagBoxRef.current && !tagBoxRef.current.contains(e.target as Node)) setTagOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [tagOpen])

  const tagSuggestions = useMemo(
    () => (tagOpen ? suggestTags(allTags, tagInput, tags) : []),
    [tagOpen, allTags, tagInput, tags],
  )

  const addTag = (raw: string) => {
    const name = normalizeTagInput(raw.replace(/,/g, ' '))
    if (!name) return
    // Reuse the stored spelling when one exists: attaching "go" to the "Go"
    // tag is what keeps one concept from splitting into three.
    const existing = allTags.find((x) => x.name.toLowerCase() === name.toLowerCase())
    const final = existing ? existing.name : name
    onTags(tags.some((x) => x.toLowerCase() === final.toLowerCase()) ? tags : [...tags, final])
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
        // Close the dropdown without touching what was typed, and do not let
        // the drawer's own Escape handler close the panel in the same press.
        if (tagOpen) { e.preventDefault(); e.stopPropagation(); setTagOpen(false) }
        return
      case 'Enter':
      case 'Tab':
        e.preventDefault()
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
          onTags(tags.slice(0, -1))
        }
        return
      default:
        return
    }
  }

  if (!open) return null

  const wantsPublic = status === 'published' || status === 'pending'

  return (
    <div class="pe-drawer-root">
      <div class="pe-backdrop" onClick={onClose} />
      <aside class="pe-drawer" role="dialog" aria-modal="true" aria-label={t('editor.settingsTitle')}>
        <div class="pe-drawer-head">
          <h3>{t('editor.settingsTitle')}</h3>
          <button class="btn btn-ghost btn-sm pe-drawer-close" onClick={onClose} aria-label={t('editor.settingsClose')}>
            ×
          </button>
        </div>

        <div class="pe-drawer-body">
          <h4 class="pe-group">{t('editor.settingsGeneral')}</h4>

          <div class="pe-field">
            <label>{t('editor.slug')}</label>
            <input
              ref={slugRef}
              type="text"
              value={slug}
              placeholder={t('editor.slugPlaceholder')}
              onInput={(e) => onSlug((e.target as HTMLInputElement).value)}
            />
            <p class="pe-hint">{t('editor.slugHint')}</p>
          </div>

          <div class="pe-field">
            <label>{t('editor.category')}</label>
            <select
              value={String(category)}
              onChange={(e) =>
                onCategory((e.target as HTMLSelectElement).value === '' ? '' : Number((e.target as HTMLSelectElement).value))
              }
            >
              <option value="">{t('editor.allCategories')}</option>
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
                    onClick={() => onTags(tags.filter((x) => x !== tg))}
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
            <p class="pe-hint">{t('editor.tagHint')}</p>
          </div>

          <div class="pe-field">
            <label>{t('editor.cover')}</label>
            {cover ? (
              <div class="pe-cover-preview">
                <img src={cover} alt="cover" />
                <button class="btn btn-ghost btn-sm" onClick={() => onCover('')}>
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
                      if (f) onUploadCover(f)
                    }}
                  />
                </label>
                <button class="btn btn-ghost btn-sm" onClick={onPickCoverFromBody}>
                  {t('editor.coverFromBody')}
                </button>
              </div>
            )}
          </div>

          <h4 class="pe-group">{t('editor.settingsAdvanced')}</h4>

          <div class="pe-field">
            <label>{t('editor.visibility')}</label>
            {status === 'rejected' ? (
              // The full reviewer note stays on the canvas above the editor;
              // repeating it here would be the same paragraph twice on one
              // screen. This line only explains why the radios are gone.
              <p class="pe-pinned-hint">{t('editor.visibilityReviewHint')}</p>
            ) : (
              <div class="pe-visibility" role="radiogroup" aria-label={t('editor.visibility')}>
                <label class={`pe-visibility-opt ${wantsPublic ? 'is-active' : ''}`}>
                  <input
                    type="radio"
                    name="pe-visibility"
                    checked={wantsPublic}
                    onChange={() => onStatus('published')}
                  />
                  <span class="pe-visibility-dot pe-visibility-dot--public"></span>
                  <span>{t('editor.visibilityPublic')}</span>
                </label>
                <label class={`pe-visibility-opt ${status === 'private' ? 'is-active' : ''}`}>
                  <input
                    type="radio"
                    name="pe-visibility"
                    checked={status === 'private'}
                    onChange={() => onStatus('private')}
                  />
                  <span class="pe-visibility-dot pe-visibility-dot--private"></span>
                  <span>{t('editor.visibilityPrivate')}</span>
                </label>
              </div>
            )}
            <p class="pe-pinned-hint">
              {status === 'private' ? t('editor.visibilityPrivateHint') : t('editor.visibilityPublicHint')}
            </p>
            {!isRootUser && status !== 'private' && (
              <p class="pe-pinned-hint pe-review-hint">
                {status === 'pending'
                  ? t('editor.pendingHint')
                  : wasPublished ? t('editor.rereviewHint') : t('editor.visibilityReviewHint')}
              </p>
            )}
          </div>

          {canPin && (
            <div class="pe-field">
              <label class="pe-pinned">
                <input
                  type="checkbox"
                  checked={pinned}
                  onChange={(e) => onPinned((e.target as HTMLInputElement).checked)}
                />
                <span>{t('editor.pinnedLabel')}</span>
              </label>
              <p class="pe-pinned-hint">{t('editor.pinnedHint')}</p>
            </div>
          )}
        </div>

        <div class="pe-drawer-foot">
          <button class="btn btn-primary" onClick={onClose}>
            {t('editor.settingsDone')}
          </button>
        </div>
      </aside>
    </div>
  )
}
