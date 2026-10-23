import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../../core/i18n'
import { useNav } from '../../core/router'
import { myStats, listPostsPaged } from '../../core/api'
import type { UserPublic, UserStats } from '../../core/types'
import { fmtDay } from '../../core/format'
import { useQuery } from '../../core/useQuery'
import { Pager, PAGE_SIZE } from './Pager'

export function MyHomepage({ user }: { user: UserPublic | null }) {
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [stats, setStats] = useState<UserStats | null>(null)
  const [sort, setSort] = useState<'created_at' | 'views'>('created_at')
  const [order, setOrder] = useState<'asc' | 'desc'>('desc')
  const [page, setPage] = useState(1)

  useEffect(() => {
    myStats().then(setStats).catch(() => {})
  }, [])

  const { data, loading, error, reload } = useQuery(
    () => listPostsPaged({ page, size: PAGE_SIZE, sort, order }),
    [page, sort, order],
  )

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
            <button class="btn btn-sm" onClick={reload}>{t('center.retry')}</button>
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
