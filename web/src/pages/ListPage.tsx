import { useEffect, useState, useCallback, useRef } from 'preact/hooks'
import { useDebounce } from '../core/debounce'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listPostsPaged, deletePost, listCategories, ApiError } from '../core/api'
import type { PostPage, PostStatus, Category } from '../core/types'
import { renderMD, stripHTML, highlight } from '../core/md'
import { Sidebar } from '../ui/Sidebar'
import { useNav } from '../core/router'

const PAGE_SIZE = 20

export function ListPage() {
  const { user } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const [, navigate] = useNav()
  const [pageData, setPageData] = useState<PostPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [status, setStatus] = useState<PostStatus | ''>('')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [activeCategory, setActiveCategory] = useState<number | null>(null)
  const [activeTag, setActiveTag] = useState<string | null>(null)
  const [categories, setCategories] = useState<Category[]>([])
  const [error, setError] = useState(false)
  const sentinelRef = useRef<HTMLDivElement | null>(null)
  const hasMoreRef = useRef(false)
  const debouncedSearch = useDebounce(search, 300)

  // Load one page. `append` keeps previously loaded items (infinite scroll);
  // a fresh load (page reset / filter change) replaces the list.
  const load = useCallback(
    async (
      q: number,
      s: PostStatus | '',
      srch: string,
      cat: number | null,
      tag: string | null,
      append: boolean,
    ) => {
      if (append) setLoadingMore(true)
      else setLoading(true)
      setError(false)
      try {
        const res = await listPostsPaged({
          status: s || undefined,
          q: srch || undefined,
          page: q,
          size: PAGE_SIZE,
          // Explicit null check: a valid category id could be falsy (0),
          // which `||` would silently drop and disable the filter.
          category: cat == null ? undefined : cat,
          tag: tag == null ? undefined : tag,
        })
        setPageData((prev) =>
          append && prev ? { ...prev, items: [...prev.items, ...res.items] } : res,
        )
        setPage(q)
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) {
          navigate('/login')
          return
        }
        setError(true)
        pushToast('error', t('post.loadFailed'))
      } finally {
        setLoading(false)
        setLoadingMore(false)
      }
    },
    [pushToast, t, navigate],
  )

  const loadCategories = useCallback(() => {
    listCategories().then(setCategories).catch(() => {})
  }, [])

  useEffect(() => {
    loadCategories()
  }, [loadCategories])

  // Reset to the first page whenever a filter changes.
  useEffect(() => {
    setPage(1)
  }, [debouncedSearch, status, activeCategory, activeTag])

  // Single data-fetch effect: fresh load on page 1 / back, append on forward.
  useEffect(() => {
    load(page, status, debouncedSearch, activeCategory, activeTag, page > 1)
  }, [page, status, debouncedSearch, activeCategory, activeTag, load])

  const loadNext = useCallback(() => {
    if (loading || loadingMore || error) return
    if (!hasMoreRef.current) return
    setPage((p) => p + 1)
  }, [loading, loadingMore, error])

  // Infinite scroll: observe the bottom sentinel and load more when it scrolls
  // into view (with a 200px lookahead margin).
  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && hasMoreRef.current) loadNext()
      },
      { rootMargin: '200px' },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [loadNext])

  const remove = async (id: string) => {
    if (!(await confirmDialog(t('list.deleteConfirm')))) return
    try {
      await deletePost(id)
      pushToast('success', t('post.deleted'))
      await load(page, status, debouncedSearch, activeCategory, activeTag, false)
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('post.deleteConfirm'))
    }
  }

  const onCategorySelect = (id: number | null) => {
    setActiveCategory(id)
    setPage(1)
  }

  const onTagSelect = (name: string | null) => {
    setActiveTag(name)
    setPage(1)
  }

  const totalPagesNum = pageData ? Math.max(1, Math.ceil(pageData.total / PAGE_SIZE)) : 1
  const totalItems = pageData?.total ?? 0
  const hasMore = page < totalPagesNum
  hasMoreRef.current = hasMore
  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'

  const items = pageData?.items ?? []

  return (
    <div class="list-page">
      <div class="list-layout">
        <div class="list-main">
          <div class="list-header">
            <h1>{t('list.title')}</h1>
            <button class="btn btn-primary" onClick={() => navigate('/create')}>
              {t('nav.create')}
            </button>
          </div>

          <div class="list-toolbar">
            <input
              placeholder={t('list.searchPlaceholder')}
              value={search}
              onInput={(e) => setSearch((e.target as HTMLInputElement).value)}
            />
            <select
              value={activeCategory !== null ? String(activeCategory) : ''}
              onInput={(e) => {
                const val = (e.target as HTMLSelectElement).value
                onCategorySelect(val ? Number(val) : null)
              }}
            >
              <option value="">{t('list.allCategories')}</option>
              {categories.map((c) => (
                <option key={c.id} value={String(c.id)}>{c.name}</option>
              ))}
            </select>
            <select value={status} onInput={(e) => setStatus((e.target as HTMLSelectElement).value as PostStatus | '')}>
              <option value="">{t('list.allStatus')}</option>
              <option value="published">{t('list.published')}</option>
              <option value="private">{t('list.private')}</option>
            </select>
          </div>

          {loading ? (
            <p class="muted">{t('common.loading')}</p>
          ) : totalItems === 0 ? (
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
          ) : (
            <>
              <p class="list-count">
                {t('list.pageOf', { page: String(page), total: String(totalPagesNum) })}
                &middot; {t('common.count', { from: String((page - 1) * PAGE_SIZE + 1), to: String((page - 1) * PAGE_SIZE + items.length), total: String(totalItems) })}
              </p>
              <ul class="post-list">
                {items.map((p) => (
                  <li key={p.id} class="post-item" onClick={() => navigate(`/read/${p.id}`)}>
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
                    {(p.category?.name || (p.tags?.length || 0) > 0) && (
                      <div class="post-tags">
                        {p.category?.name && <span class="post-cat-chip">{p.category.name}</span>}
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
                      <span class="post-actions">
                        <a href="#" onClick={(e) => { e.preventDefault(); navigate(`/edit/${p.id}`) }}>
                          {t('list.edit')}
                        </a>
                        <a href="#" onClick={(e) => { e.preventDefault(); remove(p.id) }}>
                          {t('list.del')}
                        </a>
                      </span>
                    </div>
                  </li>
                ))}
              </ul>
              {/* Paging is append-style (infinite scroll): a classic
                  prev/next pager would re-append page 1 on "prev" and
                  duplicate items, so progress lives in the count line
                  above and loading continues via the sentinel below. */}
              <div class="lazy-load" ref={sentinelRef}>
                {loadingMore ? (
                  <span class="lazy-load-text">
                    <span class="lazy-load-spinner" />
                    {t('list.loadingMore')}
                  </span>
                ) : hasMore ? (
                  <button class="btn btn-ghost btn-sm lazy-load-btn" onClick={loadNext}>
                    {t('list.loadMore')}
                  </button>
                ) : (
                  <span class="lazy-load-end">{t('list.noMore')}</span>
                )}
              </div>
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
