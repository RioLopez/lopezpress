import { useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { useNav } from '../core/router'
import { listArchive, ApiError } from '../core/api'
import type { Post } from '../core/types'

// Group posts by month ("YYYY-MM", derived from created_at). Keeps the group
// order stable by preserving the already-descending backend order.
function groupByMonth(posts: Post[]): { month: string; items: Post[] }[] {
  const groups: { month: string; items: Post[] }[] = []
  for (const p of posts) {
    const month = p.created_at.slice(0, 7)
    const last = groups[groups.length - 1]
    if (last && last.month === month) last.items.push(p)
    else groups.push({ month, items: [p] })
  }
  return groups
}

export function ArchivePage() {
  const { user } = useAuth()
  const { pushToast } = useApp()
  const { t, locale } = useI18n()
  const [, navigate] = useNav()
  const [groups, setGroups] = useState<{ month: string; items: Post[] }[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    let active = true
    listArchive()
      .then((posts) => {
        if (!active) return
        setGroups(groupByMonth(posts))
        setError(false)
      })
      .catch((e) => {
        if (!active) return
        if (e instanceof ApiError && e.status === 401) {
          navigate('/login')
          return
        }
        setError(true)
        pushToast('error', t('post.loadFailed'))
      })
      .finally(() => active && setLoading(false))
    return () => {
      active = false
    }
  }, [pushToast, t, navigate])

  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'
  const totalPosts = groups.reduce((n, g) => n + g.items.length, 0)

  return (
    <div class="archive-page">
      <div class="archive-main">
        <div class="list-header">
          <h1>{t('archive.title')}</h1>
        </div>

        {loading ? (
          <p class="muted">{t('common.loading')}</p>
        ) : error ? (
          <p class="muted">{t('archive.loadFailed')}</p>
        ) : groups.length === 0 ? (
          <div class="empty-state">
            <div class="empty-icon">
              <div class="empty-icon-doc">
                <div class="empty-icon-line" />
                <div class="empty-icon-line empty-icon-line--short" />
                <div class="empty-icon-line empty-icon-line--short" />
              </div>
            </div>
            <p class="empty-text">{user ? t('archive.empty') : t('archive.emptyLoggedOut')}</p>
          </div>
        ) : (
          <div class="archive-groups">
            <p class="list-count">{t('archive.total', { n: String(totalPosts) })}</p>
            {groups.map((g) => (
              <section key={g.month} class="archive-group">
                <h2 class="archive-group-title">{g.month}</h2>
                <ul class="archive-list">
                  {g.items.map((p) => (
                    <li key={p.id} class="archive-item" onClick={() => navigate(`/read/${p.id}`)}>
                      <span class="archive-item-date">
                        {new Date(p.created_at).toLocaleDateString(localeTag, { day: 'numeric' })}
                      </span>
                      <span class="archive-item-title">{p.title}</span>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}