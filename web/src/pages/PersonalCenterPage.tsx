import { useEffect, useState, useRef, useCallback } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { useNav } from '../core/router'
import { useApp } from '../core/app'
import { listPostsPaged, myStats, updatePost, changePassword, meLoginHistory, logoutAll, ApiError } from '../core/api'
import type { PostPage, Post, UserPublic, UserStats, LoginLog } from '../core/types'
import { fmtDay } from '../core/format'

// Personal center (spec 20). Current user's own page with a left sidebar
// (PC) / top tabs (mobile, reserved) and three sections: My Homepage,
// Content, Security. PC-first; mobile degrades via CSS but the structure is
// already in place.

type Tab = 'home' | 'manage' | 'security'
const PAGE_SIZE = 10


// Partially mask an email: keep first + last char of the local part, hide the
// middle (spec 20 §六.1).
const maskEmail = (email: string) => {
  const [name, domain] = email.split('@')
  if (!domain) return email
  if (name.length <= 2) return `***@${domain}`
  return `${name[0]}***${name[name.length - 1]}@${domain}`
}

function Pager({ page, pages, loading, onPrev, onNext }: {
  page: number
  pages: number
  loading: boolean
  onPrev: () => void
  onNext: () => void
}) {
  const { t } = useI18n()
  return (
    <div class="pager">
      <button class="btn btn-ghost btn-sm" disabled={page <= 1 || loading} onClick={onPrev}>
        {t('list.prevPage')}
      </button>
      <span class="pager-info">{t('list.pageOf', { page: String(page), total: String(pages) })}</span>
      <button class="btn btn-ghost btn-sm" disabled={page >= pages || loading} onClick={onNext}>
        {t('list.nextPage')}
      </button>
    </div>
  )
}

function MyHomepage({ user }: { user: UserPublic | null }) {
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [stats, setStats] = useState<UserStats | null>(null)
  const [sort, setSort] = useState<'created_at' | 'views'>('created_at')
  const [order, setOrder] = useState<'asc' | 'desc'>('desc')
  const [page, setPage] = useState(1)
  const [data, setData] = useState<PostPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  // Bumped to force a fresh reload (used by the retry button and after a
  // resubmit) without depending on `page`, which may already be at its value.
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    myStats().then(setStats).catch(() => {})
  }, [])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(false)
    listPostsPaged({ page, size: PAGE_SIZE, sort, order })
      .then((d) => { if (!cancelled) setData(d) })
      .catch(() => { if (!cancelled) setError(true) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [page, sort, order, reloadKey])

  const pages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1
  const initial = ((user?.display_name || user?.username || '?')[0] || '?').toUpperCase()

  return (
    <div class="center-section">
      {/* Profile banner (spec 20 §四.1) */}
      <section class="center-banner">
        <div class="center-banner-row">
          <div class="center-banner-id">
            {user?.avatar ? (
              <img class="center-banner-avatar" src={user.avatar} alt={user.username} />
            ) : (
              <span class="center-banner-avatar center-banner-avatar--fallback">{initial}</span>
            )}
            <span class="center-banner-name">{user?.display_name || user?.username}</span>
          </div>
          <div class="center-banner-stats">
            <div class="center-stat">
              <div class="center-stat-num">{stats ? stats.views_total : '–'}</div>
              <div class="center-stat-label">{t('center.bannerViews')}</div>
            </div>
            <div class="center-stat">
              <div class="center-stat-num">{stats ? stats.posts_total : '–'}</div>
              <div class="center-stat-label">{t('center.bannerPosts')}</div>
            </div>
          </div>
        </div>
        <div class="center-banner-meta">
          <span>{t('center.registeredAt')}：{fmtDay(user?.created_at)}</span>
        </div>
        <div class="center-banner-bio">
          <span class="center-banner-bio-text">{user?.bio || '—'}</span>
          <button class="btn btn-sm" onClick={() => navigate('/settings')}>
            {t('center.editProfile')}
          </button>
        </div>
      </section>

      {/* Article list with sort + pagination (spec 20 §四.2) */}
      <section class="center-posts">
        <div class="sortbar">
          <span class="sortbar-label">{t('center.sortBy')}</span>
          <select
            class="select"
            value={sort}
            onChange={(e) => { setSort((e.currentTarget as HTMLSelectElement).value as 'created_at' | 'views'); setPage(1) }}
          >
            <option value="created_at">{t('center.sortTime')}</option>
            <option value="views">{t('center.sortViews')}</option>
          </select>
          <select
            class="select"
            value={order}
            onChange={(e) => { setOrder((e.currentTarget as HTMLSelectElement).value as 'asc' | 'desc'); setPage(1) }}
          >
            <option value="desc">{t('center.orderDesc')}</option>
            <option value="asc">{t('center.orderAsc')}</option>
          </select>
        </div>

        {loading && <p class="muted center-loading">{t('center.loading')}</p>}
        {error && (
          <div class="center-error">
            <p class="muted">{t('center.loadError')}</p>
            <button class="btn btn-sm" onClick={() => setReloadKey((k) => k + 1)}>{t('center.retry')}</button>
          </div>
        )}
        {!loading && !error && data && data.items.length === 0 && (
          <p class="muted center-empty">{t('center.empty')}</p>
        )}
        {!loading && !error && data && data.items.length > 0 && (
          <div class="post-list">
            {data.items.map((post) => (
              <button
                key={post.id}
                class="post-row"
                onClick={() => navigate(`/read/${post.id}`)}
              >
                <span class="post-row-title">{post.title}</span>
                <span class="post-row-meta">
                  {fmtDay(post.created_at)} · {post.view_count} {t('list.views')}
                </span>
              </button>
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
      </section>
    </div>
  )
}

function ContentManage() {
  const { t } = useI18n()
  const { pushToast } = useApp()
  const [, navigate] = useNav()
  const [page, setPage] = useState(1)
  const [data, setData] = useState<PostPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  // Bumped to force a fresh reload (retry button / after a resubmit) without
  // depending on `page`, which may already be at its current value.
  const [reloadKey, setReloadKey] = useState(0)
  // Optimistic visibility switch + undo (spec 22). Undo holds the previous
  // status so the user can roll back within a short window; at most one
  // pending undo is kept, and flipping again clears it.
  const [busyId, setBusyId] = useState<string | null>(null)
  const undoRef = useRef<{ id: string; prev: 'published' | 'private' } | null>(null)
  const undoTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [undoAvailable, setUndoAvailable] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(false)
    // Own posts, newest first; management view shows visibility + edit.
    listPostsPaged({ page, size: PAGE_SIZE })
      .then((d) => { if (!cancelled) setData(d) })
      .catch(() => { if (!cancelled) setError(true) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [page, reloadKey])

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
      .catch((e) => pushToast('error', e instanceof ApiError ? e.message : t('list.visibilityFailed')))
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
          pushToast('error', e instanceof ApiError ? e.message : t('list.visibilityFailed'))
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
      .catch((e) => pushToast('error', e instanceof ApiError ? e.message : t('center.resubmitFailed')))
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
            <button class="btn btn-sm" onClick={() => setReloadKey((k) => k + 1)}>{t('center.retry')}</button>
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

function SecurityCenter({ user }: { user: UserPublic | null }) {
  const { t } = useI18n()
  const { logout } = useAuth()
  const { pushToast } = useApp()
  const [, navigate] = useNav()
  const [show, setShow] = useState(false)

  // Change-password form state.
  const [cur, setCur] = useState('')
  const [npw, setNpw] = useState('')
  const [cfm, setCfm] = useState('')
  const [pwBusy, setPwBusy] = useState(false)
  const [pwErr, setPwErr] = useState('')

  // Login history (self-service).
  const [logs, setLogs] = useState<LoginLog[] | null>(null)
  const [logsErr, setLogsErr] = useState(false)

  useEffect(() => {
    meLoginHistory(20)
      .then(setLogs)
      .catch(() => setLogsErr(true))
  }, [])

  if (!user) return <p class="muted center-empty">{t('center.securityEmpty')}</p>

  const submitPw = (e: Event) => {
    e.preventDefault()
    setPwErr('')
    if (npw.length < 8) {
      setPwErr(t('center.secPwTooShort'))
      return
    }
    if (npw !== cfm) {
      setPwErr(t('center.secPwMismatch'))
      return
    }
    setPwBusy(true)
    changePassword({ current_password: cur, new_password: npw })
      .then(() => {
        pushToast('success', t('center.secPwChanged'))
        setCur('')
        setNpw('')
        setCfm('')
      })
      .catch((e) =>
        setPwErr(
          e instanceof ApiError
            ? e.status === 401
              ? t('center.secPwWrong')
              : e.message
            : t('center.secPwWrong'),
        ),
      )
      .finally(() => setPwBusy(false))
  }

  const doLogoutAll = () => {
    if (!confirm(t('center.secLogoutAllConfirm'))) return
    logoutAll()
      .then(() => {
        logout()
        pushToast('info', t('center.secLogoutAllDone'))
        navigate('/login')
      })
      .catch(() => {
        // Even if the server call fails, clear the local session.
        logout()
        navigate('/login')
      })
  }

  const fmtTime = (s: string) => {
    const d = new Date(s)
    return isNaN(d.getTime()) ? s : d.toLocaleString()
  }

  return (
    <div class="center-section">
      {/* Account basics (email masked) */}
      <section class="center-security">
        <div class="info-card">
          <div class="info-row">
            <span class="info-label">{t('center.account')}</span>
            <span class="info-value">{show ? user.email : maskEmail(user.email)}</span>
          </div>
          <div class="info-row">
            <span class="info-label">{t('center.username')}</span>
            <span class="info-value">{user.username}</span>
          </div>
          <button class="btn btn-sm btn-ghost" onClick={() => setShow((s) => !s)}>
            {show ? t('center.hide') : t('center.show')}
          </button>
        </div>
        <p class="muted center-note">{t('center.securityNote')}</p>
      </section>

      {/* Change password */}
      <section class="sec-card">
        <h2 class="center-subtitle">{t('center.secChangePw')}</h2>
        <form class="sec-form" onSubmit={submitPw}>
          <label class="sec-field">
            <span>{t('center.secCurrentPw')}</span>
            <input
              type="password"
              value={cur}
              autocomplete="current-password"
              onInput={(e) => setCur((e.currentTarget as HTMLInputElement).value)}
            />
          </label>
          <label class="sec-field">
            <span>{t('center.secNewPw')}</span>
            <input
              type="password"
              value={npw}
              autocomplete="new-password"
              onInput={(e) => setNpw((e.currentTarget as HTMLInputElement).value)}
            />
          </label>
          <label class="sec-field">
            <span>{t('center.secConfirmPw')}</span>
            <input
              type="password"
              value={cfm}
              autocomplete="new-password"
              onInput={(e) => setCfm((e.currentTarget as HTMLInputElement).value)}
            />
          </label>
          {pwErr && <p class="sec-error">{pwErr}</p>}
          <button class="btn btn-primary" type="submit" disabled={pwBusy}>
            {t('center.secPwBtn')}
          </button>
        </form>
      </section>

      {/* Login history */}
      <section class="sec-card">
        <h2 class="center-subtitle">{t('center.secLoginHistory')}</h2>
        {logs === null && <p class="muted center-loading">{t('center.loading')}</p>}
        {logsErr && <p class="muted">{t('center.loadError')}</p>}
        {logs && logs.length === 0 && <p class="muted center-empty">{t('center.secNoLogins')}</p>}
        {logs && logs.length > 0 && (
          <ul class="login-list">
            {logs.map((l) => (
              <li class="login-row" key={l.id}>
                <span class={`badge ${l.success ? 'badge--ok' : 'badge--fail'}`}>
                  {l.success ? t('center.secLoginOk') : t('center.secLoginFail')}
                </span>
                <div class="login-meta">
                  <div>
                    {t('center.secIp')}: {l.ip || '—'}
                  </div>
                  <div class="muted login-ua">
                    {t('center.secDevice')}: {l.ua || '—'} · {fmtTime(l.created_at)}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* Sign out everywhere */}
      <section class="sec-card">
        <button class="btn btn-danger" onClick={doLogoutAll}>
          {t('center.secLogoutAll')}
        </button>
      </section>
    </div>
  )
}

export function PersonalCenterPage() {
  const { user } = useAuth()
  const { t } = useI18n()
  const [tab, setTab] = useState<Tab>('home')

  return (
    <div class="center-layout">
      <aside class="center-sidebar">
        <div class="center-nav">
          <button
            class={`center-nav-link ${tab === 'home' ? 'center-nav-link--active' : ''}`}
            onClick={() => setTab('home')}
          >
            {t('center.myHome')}
          </button>
          <button
            class={`center-nav-link ${tab === 'manage' ? 'center-nav-link--active' : ''}`}
            onClick={() => setTab('manage')}
          >
            {t('center.contentManage')}
          </button>
          <button
            class={`center-nav-link ${tab === 'security' ? 'center-nav-link--active' : ''}`}
            onClick={() => setTab('security')}
          >
            {t('center.security')}
          </button>
        </div>
      </aside>
      <main class="center-main">
        <h1 class="center-title">{t('center.title')}</h1>
        {tab === 'home' && <MyHomepage user={user} />}
        {tab === 'manage' && <ContentManage />}
        {tab === 'security' && <SecurityCenter user={user} />}
      </main>
    </div>
  )
}
