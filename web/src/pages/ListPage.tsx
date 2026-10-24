import { useState, useCallback, useMemo } from 'preact/hooks'
import { useDebounce } from '../core/debounce'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listPostsPaged, deletePost, listCategories, errMessage, errStatus } from '../core/api'
import type { Post, PostStatus } from '../core/types'
import { renderMD, stripHTML, highlight } from '../core/md'
import { Sidebar } from '../ui/Sidebar'
import { Pagination } from '../ui/Pagination'
import { useNavSearch } from '../core/router'
import { useQuery } from '../core/useQuery'
import { isAdminOnly, isRoot } from '../core/roles'
import { orderCategories, categoryOptionLabel, categoryLabel } from '../core/taxonomy'

// Spec 24: classic pagination. The page number lives in the URL (?page=N) and
// the page SIZE is decided by the server from site_config.page_size — the
// client deliberately never sends `size`, so one link means one thing for
// everybody and stays shareable.

// The filters that are part of the URL (deep-linkable from the admin
// category/tag pages). Keyword + status stay in component state: pushing a
// URL on every keystroke would trash the history.
interface UrlFilters {
  page: number
  // Category NAME, not id: ?category=PHP reads like ?tag=Go and stays
  // meaningful when the URL is pasted into a chat. The server still accepts a
  // numeric id, so bookmarks made before this change keep resolving.
  category: string | null
  tag: string | null
}

function parseUrl(search: string): UrlFilters {
  const p = new URLSearchParams(search)
  const rawPage = parseInt(p.get('page') ?? '', 10)
  return {
    page: Number.isFinite(rawPage) && rawPage > 0 ? rawPage : 1,
    category: p.get('category') || null,
    tag: p.get('tag') || null,
  }
}

function buildUrl(f: UrlFilters): string {
  const p = new URLSearchParams()
  if (f.page > 1) p.set('page', String(f.page))
  if (f.category) p.set('category', f.category)
  if (f.tag) p.set('tag', f.tag)
  const qs = p.toString()
  return qs ? `/?${qs}` : '/'
}

export function ListPage() {
  const { user } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const [, urlSearch, navigate] = useNavSearch()

  // wouter's useLocation is pathname-only; the query string arrives as a
  // separate value (see useNavSearch). Parsing it off `location` would
  // silently drop every deep-linkable filter.
  const url = parseUrl(urlSearch)
  const page = url.page
  const activeCategory = url.category
  const activeTag = url.tag

  const [status, setStatus] = useState<PostStatus | ''>('')
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebounce(search, 300)

  const { data: pageData, loading, reload } = useQuery(
    () =>
      listPostsPaged({
        status: status || undefined,
        q: debouncedSearch || undefined,
        page,
        // No `size`: the server applies site_config.page_size (spec 24).
        category: activeCategory ?? undefined,
        tag: activeTag == null ? undefined : activeTag,
      }),
    [page, status, debouncedSearch, activeCategory, activeTag],
    {
      onError: (e) => {
        if (errStatus(e) === 401) {
          navigate('/login')
          return
        }
        pushToast('error', t('post.loadFailed'))
      },
    },
  )

  const { data: categoriesData } = useQuery(() => listCategories(), [])
  const categories = categoriesData ?? []
  // One flattening for both the filter dropdown and the card breadcrumbs, so
  // the hierarchy cannot be right in one place and flat in the other.
  const categoryNodes = useMemo(() => orderCategories(categories), [categories])

  // Single navigation helper: every filter change goes through it so the page
  // number is always reset deliberately (spec 24 AC-3.1) instead of by an
  // effect that would also fire on mount and fight a deep link.
  const go = useCallback(
    (next: { page?: number; category?: string | null; tag?: string | null }) => {
      navigate(
        buildUrl({
          page: next.page ?? 1,
          category: next.category !== undefined ? next.category : activeCategory,
          tag: next.tag !== undefined ? next.tag : activeTag,
        }),
      )
    },
    [navigate, activeCategory, activeTag],
  )

  const remove = async (id: string) => {
    if (!(await confirmDialog(t('list.deleteConfirm')))) return
    try {
      await deletePost(id)
      pushToast('success', t('post.deleted'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('post.deleteConfirm')))
    }
  }

  const onCategorySelect = (name: string | null) => go({ category: name })
  const onTagSelect = (name: string | null) => go({ tag: name })

  // Server-decided page size; fall back to 10 only while the first response
  // is still in flight.
  const size = pageData?.page_size || 10
  const totalItems = pageData?.total ?? 0
  const totalPagesNum = Math.max(1, Math.ceil(totalItems / size))
  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'

  const items = pageData?.items ?? []

  // Who may act on a card: mirrors the backend UpdatePost/DeletePost guards
  // (author, or root, or an admin over a plain user's post) plus the banned
  // freeze (only root edits; deletes go through the root-only vault).
  // Without this the links rendered for everyone — anonymous visitors
  // included — and every click died on a 403.
  const canEditPost = (p: Post): boolean => {
    if (!user) return false
    if (p.author?.id === user.id || isRoot(user.role)) return true
    return isAdminOnly(user.role) && p.author?.role === 'user'
  }
  const canDeletePost = (p: Post): boolean =>
    p.status !== 'banned' && canEditPost(p)
  // A deep link may point past the end (post deleted, filters changed). That
  // is not an error state — show the message plus the pager so the reader can
  // get back (spec 24 AC-1.4).
  const hasActiveFilter = debouncedSearch !== '' || status !== '' || activeCategory !== null || activeTag !== null
  const outOfRange = !loading && items.length === 0 && totalItems > 0 && page > totalPagesNum

  return (
    <div class="list-page">
      <div class="list-layout">
        <div class="list-main">
          <div class="list-header">
            <h1>{t('list.title')}</h1>
            {/* Spec 27: anonymous readers must never see an authoring entry. */}
            {user && (
              <button class="btn btn-primary" onClick={() => navigate('/create')}>
                {t('nav.create')}
              </button>
            )}
          </div>

          <div class="list-toolbar">
            <input
              placeholder={t('list.searchPlaceholder')}
              value={search}
              onInput={(e) => {
                setSearch((e.target as HTMLInputElement).value)
                // Spec 24 AC-3.1: a filter change restarts at page 1. Doing it
                // here (not in an effect) keeps a deep link like ?page=3 from
                // being reset on mount.
                if (page !== 1) go({ page: 1 })
              }}
            />
            <select
              value={activeCategory ?? ''}
              onInput={(e) => {
                const val = (e.target as HTMLSelectElement).value
                onCategorySelect(val || null)
              }}
            >
              <option value="">{t('list.allCategories')}</option>
              {/* Options cannot be styled, so the two-level tree is spelled
                  with an indent + └ prefix. Values are names, matching the
                  URL and the sidebar. */}
              {categoryNodes.map((node) => (
                <option key={node.cat.id} value={node.cat.name}>
                  {categoryOptionLabel(node)}
                </option>
              ))}
            </select>
            <select value={status} onInput={(e) => {
              setStatus((e.target as HTMLSelectElement).value as PostStatus | '')
              if (page !== 1) go({ page: 1 })
            }}>
              <option value="">{t('list.allStatus')}</option>
              <option value="published">{t('list.published')}</option>
              <option value="private">{t('list.private')}</option>
            </select>
          </div>

          {loading ? (
            <p class="muted">{t('common.loading')}</p>
          ) : totalItems === 0 ? (
            // A filtered empty result is NOT "you have no posts yet": the old
            // copy told a reader to go write their first post after a search
            // that simply matched nothing (spec 28).
            hasActiveFilter ? (
              <div class="empty-state">
                <p class="empty-text">{t('list.noResults')}</p>
                <button class="btn btn-ghost btn-sm" onClick={() => { setSearch(''); setStatus(''); go({ page: 1, category: null, tag: null }) }}>
                  {t('list.clearFilters')}
                </button>
              </div>
            ) : (
              <div class="empty-state">
                <div class="empty-icon">
                  <div class="empty-icon-doc">
                    <div class="empty-icon-line" />
                    <div class="empty-icon-line empty-icon-line--short" />
                    <div class="empty-icon-line empty-icon-line--short" />
                  </div>
                </div>
                <p class="empty-text">{user ? t('list.empty') : t('list.emptyLoggedOut')}</p>
              </div>
            )
          ) : outOfRange ? (
            <div class="empty-state">
              <p class="empty-text">{t('list.pageOutOfRange')}</p>
              <button class="btn btn-ghost btn-sm" onClick={() => go({ page: 1 })}>
                {t('list.firstPage')}
              </button>
            </div>
          ) : (
            <>
              <p class="list-count">
                {t('list.pageOf', { page: String(page), total: String(totalPagesNum) })}
                &middot; {t('common.count', { from: String((page - 1) * size + 1), to: String((page - 1) * size + items.length), total: String(totalItems) })}
              </p>
              <ul class="post-list">
                {items.map((p) => (
                  <li key={p.id} class="post-item" onClick={() => navigate(`/read/${p.id}`)}>
                    {/* Spec 28: the cover field existed (editor + model) but
                        was never rendered in any list, so choosing a cover
                        had no visible effect. */}
                    {p.cover && (
                      <img class="post-cover-thumb" src={p.cover} alt="" loading="lazy" />
                    )}
                    <div class="post-item-head">
                      <span class="post-title">
                        {p.pinned && <span class="badge-pinned">{t('list.pinned')}</span>}
                        <span
                          class="post-title-text"
                          dangerouslySetInnerHTML={{ __html: highlight(p.title, debouncedSearch) }}
                        />
                      </span>
                      {p.status !== 'published' && (
                        <span class={`post-status post-status-${p.status}`}>
                          {t(`admin.statusLabel.${p.status}`)}
                        </span>
                      )}
                    </div>
                    {(p.category || (p.tags?.length || 0) > 0) && (
                      <div class="post-tags">
                        {/* Breadcrumb ("PHP › php教程") so a child category is
                            readable as a child even out of the sidebar. */}
                        {p.category && (
                          <span class="post-cat-chip">
                            {categoryLabel(categories, p.category.id) || p.category.name}
                          </span>
                        )}
                        {p.tags?.map((tg) => (
                          <span key={tg.id} class="post-tag-chip">{tg.name}</span>
                        ))}
                      </div>
                    )}
                    <p
                      class="post-excerpt"
                      dangerouslySetInnerHTML={{
                        __html: highlight(
                          stripHTML(renderMD(p.body)).slice(0, 120) || t('post.emptyBody'),
                          debouncedSearch,
                        ),
                      }}
                    />
                    <div class="post-meta">
                      <span>{t('list.author')}: {p.author?.username || '—'}</span>
                      <span>{new Date(p.created_at).toLocaleDateString(localeTag)}</span>
                      <span>{p.view_count} {t('list.views')}</span>
                      {(canEditPost(p) || canDeletePost(p)) && (
                        <span class="post-actions">
                          {canEditPost(p) && (
                            <a href="#" onClick={(e) => { e.preventDefault(); navigate(`/edit/${p.id}`) }}>
                              {t('list.edit')}
                            </a>
                          )}
                          {canDeletePost(p) && (
                            <a href="#" onClick={(e) => { e.preventDefault(); remove(p.id) }}>
                              {t('list.del')}
                            </a>
                          )}
                        </span>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
              <Pagination page={page} totalPages={totalPagesNum} onPage={(n) => go({ page: n })} />
            </>
          )}
        </div>

        <Sidebar
          activeCategory={activeCategory}
          activeTag={activeTag}
          onCategorySelect={onCategorySelect}
          onTagSelect={onTagSelect}
        />
      </div>
    </div>
  )
}
