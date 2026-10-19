import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { getDashboard, ApiError } from '../core/api'
import { useNav } from '../core/router'
import type { DashboardData } from '../core/types'
import { fmtDate } from '../core/format'

export function AdminDashboardPage() {
  const { t, locale } = useI18n()
  const { pushToast } = useApp()
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [, navigate] = useNav()

  useEffect(() => {
    setLoading(true)
    getDashboard()
      .then(setData)
      .catch((e) => pushToast('error', e instanceof ApiError ? e.message : t('admin.failed')))
      .finally(() => setLoading(false))
  }, [pushToast, t])


  return (
    <div class="admin-page">
      <h1>{t('admin.dashboardTitle')}</h1>
      <p class="muted">{t('admin.dashboardSub')}</p>

      {loading ? (
        <p class="muted">{t('common.loading')}</p>
      ) : (
        data && (
          <>
            <div class="dash-cards">
              <div class="dash-card">
                <div class="dash-card-num">{data.posts_total}</div>
                <div class="dash-card-label">{t('admin.statPosts')}</div>
              </div>
              <div class="dash-card">
                <div class="dash-card-num">{data.views_total}</div>
                <div class="dash-card-label">{t('admin.statViews')}</div>
              </div>
            </div>

            <div class="dash-cols">
              <section class="dash-col">
                <h3>{t('admin.recentPosts')}</h3>
                {data.recent_posts.length === 0 ? (
                  <p class="muted">—</p>
                ) : (
                  <ul class="dash-list">
                    {data.recent_posts.map((p) => (
                      <li key={p.id} class="dash-list-item" onClick={() => navigate(`/read/${p.id}`)}>
                        <span class="dash-list-title">{p.title}</span>
                        <span class="dash-list-date">{new Date(p.created_at).toLocaleDateString()}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section class="dash-col">
                <h3>{t('admin.recentLogins')}</h3>
                {data.recent_logins.length === 0 ? (
                  <p class="muted">{t('admin.noLogs')}</p>
                ) : (
                  <ul class="dash-list">
                    {data.recent_logins.map((l) => (
                      <li key={l.id} class="dash-list-item">
                        <span class={`post-status post-status-${l.success ? 'published' : 'draft'}`}>
                          {l.success ? t('admin.loginOk') : t('admin.loginFail')}
                        </span>
                        <span class="dash-list-title">{l.ip}</span>
                        <span class="dash-list-date">{fmtDate(l.created_at, locale)}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </div>
          </>
        )
      )}
    </div>
  )
}
