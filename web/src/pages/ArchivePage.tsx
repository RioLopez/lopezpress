import { useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { useNav } from '../core/router'
import { listArchivePaged, errStatus } from '../core/api'
import type { Post } from '../core/types'
import { Pagination } from '../ui/Pagination'
import { useQuery } from '../core/useQuery'

// The archive is paginated now. Before this it fetched EVERY
// published post in one response (body stripped, but three relations
// preloaded) — fine for a dozen posts, unusable at a thousand on 1 vCPU.

const ARCHIVE_PAGE_SIZE = 50

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
  const { pushToast } = useApp()
  const { t, locale } = useI18n()
  const [, navigate] = useNav()
  const [page, setPage] = useState(1)

  const { data, loading, error } = useQuery(
    () => listArchivePaged({ page, size: ARCHIVE_PAGE_SIZE }),
    [page],
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

  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'
  const items = data?.items ?? []
  const groups = groupByMonth(items)
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / (data?.page_size || ARCHIVE_PAGE_SIZE)))

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
        ) : total === 0 ? (
          <div class="empty-state">
            <div class="empty-icon">
              <div class="empty-icon-doc">
                <div class="empty-icon-line" />
                <div class="empty-icon-line empty-icon-line--short" />
                <div class="empty-icon-line empty-icon-line--short" />
              </div>
            </div>
            <p class="empty-text">{t('archive.empty')}</p>
          </div>
        ) : (
          <div class="archive-groups">
            <p class="list-count">{t('archive.total', { n: String(total) })}</p>
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
            <Pagination page={page} totalPages={totalPages} onPage={setPage} />
          </div>
        )}
      </div>
    </div>
  )
}
