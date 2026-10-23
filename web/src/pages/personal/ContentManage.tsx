import { useEffect, useState, useRef, useCallback } from 'preact/hooks'
import { useI18n } from '../../core/i18n'
import { useNav } from '../../core/router'
import { useApp } from '../../core/app'
import { listPostsPaged, updatePost, errMessage } from '../../core/api'
import type { Post, PostStatus } from '../../core/types'
import { fmtDay } from '../../core/format'
import { useQuery } from '../../core/useQuery'
import { Pager, PAGE_SIZE } from './Pager'

export function ContentManage() {
  const { t } = useI18n()
  const { pushToast } = useApp()
  const [, navigate] = useNav()
  const [page, setPage] = useState(1)
  const { data, loading, error, reload, setData } = useQuery(
    () => listPostsPaged({ page, size: PAGE_SIZE }),
    [page],
  )
  // Optimistic visibility switch + undo (spec 22). Undo holds the previous
  // status so the user can roll back within a short window; at most one
  // pending undo is kept, and flipping again clears it.
  const [busyId, setBusyId] = useState<string | null>(null)
  const undoRef = useRef<{ id: string; prev: PostStatus } | null>(null)
  const undoTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [undoAvailable, setUndoAvailable] = useState(false)

  // Clear the undo affordance on unmount.
  useEffect(() => () => { if (undoTimer.current) clearTimeout(undoTimer.current) }, [])

  const clearUndo = useCallback(() => {
    undoRef.current = null
    setUndoAvailable(false)
    if (undoTimer.current) {
      clearTimeout(undoTimer.current)
      undoTimer.current = null
    }
  }, [])

  const undo = useCallback(() => {
    const u = undoRef.current
    if (!u) return
    clearUndo()
    setBusyId(u.id)
    updatePost(u.id, { status: u.prev })
      .then((p) => {
        setData((d) =>
          d
            ? { ...d, items: d.items.map((it) => (it.id === p.id ? p : it)) }
            : d,
        )
        pushToast('info', t('list.visibilityUpdated'))
      })
      .catch((e) => pushToast('error', errMessage(e, t('list.visibilityFailed'))))
      .finally(() => setBusyId(null))
  }, [clearUndo, pushToast, t])

  const toggleVisibility = useCallback(
    (post: Post) => {
      // Only the two author-controlled visibilities can be toggled. The
      // review/ban states (pending/rejected/banned) are owned by the
      // moderation workflow and have no author-side toggle.
      if (post.status === 'pending' || post.status === 'rejected' || post.status === 'banned') return
      const next: 'published' | 'private' = post.status === 'private' ? 'published' : 'private'
      const prev = post.status
      setBusyId(post.id)
      // Optimistic UI: flip the row immediately for snappy feedback.
      setData((d) =>
        d
          ? { ...d, items: d.items.map((it) => (it.id === post.id ? { ...it, status: next } : it)) }
          : d,
      )
      // Remember the previous state for the undo affordance.
      undoRef.current = { id: post.id, prev }
      setUndoAvailable(true)
      if (undoTimer.current) clearTimeout(undoTimer.current)
      undoTimer.current = setTimeout(clearUndo, 6000)

      updatePost(post.id, { status: next })
        .then((p) => {
          setData((d) =>
            d
              ? { ...d, items: d.items.map((it) => (it.id === p.id ? p : it)) }
              : d,
          )
        })
        .catch((e) => {
          // Roll back to the previous state on failure.
          setData((d) =>
            d
              ? { ...d, items: d.items.map((it) => (it.id === post.id ? { ...it, status: prev } : it)) }
              : d,
          )
          clearUndo()
          pushToast('error', errMessage(e, t('list.visibilityFailed')))
        })
        .finally(() => setBusyId(null))
    },
    [clearUndo, pushToast, t],
  )

  const pages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1

  const statusLabel = (post: Post) => {
    if (post.status === 'private') return t('list.private')
    if (post.status === 'pending') return t('admin.statusLabel.pending')
    if (post.status === 'rejected') return t('admin.statusLabel.rejected')
    if (post.status === 'banned') return t('admin.statusLabel.banned')
    if (post.status === 'draft') return t('admin.statusLabel.draft')
    return t('list.published')
  }
  // The visibility toggle only applies to the two author-controlled states.
  const canToggleVisibility = (post: Post) =>
    post.status === 'published' || post.status === 'private'

  // Resubmit a rejected post back into the review queue, closing the author
  // side of the review loop. The backend only allows the author of a rejected
  // post to move it to "pending" and clears the stale reviewer note.
  const resubmit = useCallback((post: Post) => {
    setBusyId(post.id)
    updatePost(post.id, { status: 'pending' })
      .then(() => {
        pushToast('success', t('center.resubmitted'))
        setData((d) =>
          d
            ? {
                ...d,
                items: d.items.map((it) =>
                  it.id === post.id ? { ...it, status: 'pending' as const, review_note: '' } : it,
                ),
              }
            : d,
        )
      })
      .catch((e) => pushToast('error', errMessage(e, t('center.resubmitFailed'))))
      .finally(() => setBusyId(null))
  }, [pushToast, t])

  return (
    <div class="center-section">
      <section class="center-posts">
        <h2 class="center-subtitle">{t('center.contentManage')}</h2>
        {loading && <p class="muted center-loading">{t('center.loading')}</p>}
        {error && (
          <div class="center-error">
            <p class="muted">{t('center.loadError')}</p>
            <button class="btn btn-sm" onClick={reload}>{t('center.retry')}</button>
          </div>
        )}
        {!loading && !error && data && data.items.length === 0 && (
          <p class="muted center-empty">{t('center.manageEmpty')}</p>
        )}
        {!loading && !error && data && data.items.length > 0 && (
          <div class="post-list">
            {data.items.map((post) => (
              <div key={post.id} class="post-row post-row--manage">
                <div class="post-row-main">
                  <span class="post-row-title">{post.title}</span>
                  <span class="post-row-meta">
                    {fmtDay(post.created_at)} · {post.view_count} {t('list.views')}
                  </span>
                  {/* The reviewer's note, visible only to the author, so they
                      know what to fix before resubmitting a rejected post. */}
                  {post.status === 'rejected' && post.review_note && (
                    <div class="post-row-note">
                      <strong>{t('reviewNote')}</strong> {post.review_note}
                    </div>
                  )}
                </div>
                <div class="post-row-side">
                  {/* Visibility tag: public = green, private = orange. */}
                  <span class={`badge badge--${post.status}`}>{statusLabel(post)}</span>
                  {post.status === 'rejected' ? (
                    <button
                      class="btn btn-primary btn-sm"
                      disabled={busyId === post.id}
                      onClick={() => resubmit(post)}
                    >
                      {t('editor.resubmit')}
                    </button>
                  ) : (
                    <button
                      class="btn btn-ghost btn-sm pe-visibility-toggle"
                      disabled={busyId === post.id || !canToggleVisibility(post)}
                      onClick={() => toggleVisibility(post)}
                      title={
                        !canToggleVisibility(post)
                          ? statusLabel(post)
                          : post.status === 'private'
                            ? t('list.setPublic')
                            : t('list.setPrivate')
                      }
                    >
                      {canToggleVisibility(post)
                        ? post.status === 'private'
                          ? t('list.setPublic')
                          : t('list.setPrivate')
                        : statusLabel(post)}
                    </button>
                  )}
                  <button class="btn btn-sm" onClick={() => navigate(`/edit/${post.id}`)}>
                    {t('center.edit')}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {data && data.total > 0 && (
          <Pager
            page={page}
            pages={pages}
            loading={loading}
            onPrev={() => setPage((p) => Math.max(1, p - 1))}
            onNext={() => setPage((p) => Math.min(pages, p + 1))}
          />
        )}

        {undoAvailable && (
          <div class="center-undo">
            <span class="muted">{t('list.visibilityUpdated')}</span>
            <button class="btn btn-ghost btn-sm" onClick={undo}>
              {t('list.visibilityUndo')}
            </button>
          </div>
        )}
      </section>
    </div>
  )
}
