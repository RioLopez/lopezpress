import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { getAuthor, errStatus } from '../core/api'
import { useNav } from '../core/router'
import type { AuthorView } from '../core/types'

// Public author profile (spec 15): display name, bio, avatar, and a list of
// the author's published posts. Reachable at /u/:username.
export function AuthorPage({ username }: { username: string }) {
  const { t, locale } = useI18n()
  const { pushToast } = useApp()
  const [, navigate] = useNav()
  const [data, setData] = useState<AuthorView | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)

  useEffect(() => {
    setLoading(true)
    setNotFound(false)
    getAuthor(username)
      .then(setData)
      .catch((e) => {
        if (errStatus(e) === 404) setNotFound(true)
        else pushToast('error', t('admin.failed'))
      })
      .finally(() => setLoading(false))
  }, [username, pushToast, t])

  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'

  if (loading) {
    return (
      <div class="author-page">
        <p class="muted">{t('common.loading')}</p>
      </div>
    )
  }

  if (notFound || !data) {
    return (
      <div class="author-page">
        <p class="muted">{t('post.notFound')}</p>
        <button class="btn btn-ghost" onClick={() => navigate('/')}>{t('post.backToList')}</button>
      </div>
    )
  }

  const { user, posts } = data

  return (
    <div class="author-page">
      <header class="author-head">
        {user.avatar ? (
          <img class="author-avatar" src={user.avatar} alt={user.display_name} />
        ) : (
          <span class="author-avatar author-avatar--default">
            {(user.display_name || user.username).slice(0, 1).toUpperCase()}
          </span>
        )}
        <div class="author-meta">
          <h1 class="author-name">{user.display_name || user.username}</h1>
          {user.display_name && <p class="muted">@{user.username}</p>}
          {user.bio && <p class="author-bio">{user.bio}</p>}
        </div>
      </header>

      <section class="author-posts">
        <h3>{t('author.posts')}</h3>
        {posts.length === 0 ? (
          <p class="muted">{t('author.none')}</p>
        ) : (
          <ul class="post-list">
            {posts.map((p) => (
              <li
                key={p.id}
                class="post-item"
                onClick={() => navigate(`/read/${p.id}`)}
              >
                <div class="post-item-head">
                  <span class="post-title">{p.title}</span>
                </div>
                <div class="post-meta">
                  <span>{new Date(p.created_at).toLocaleDateString(localeTag)}</span>
                  <span>{p.view_count} {t('post.views')}</span>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
