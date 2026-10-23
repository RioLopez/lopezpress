import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { getDashboard, errMessage } from '../core/api'
import { useNav } from '../core/router'
import type { DayViews } from '../core/types'
import { fmtDate } from '../core/format'
import { useQuery } from '../core/useQuery'

// Spec 33: a 30-bar view chart drawn as inline SVG.
//
// No chart library on purpose: the whole point of this project is one small
// bundle. 30 rectangles and a max() is the entire feature, and using CSS
// variables for the fill means it follows the light/dark theme for free.
function ViewsChart({ data, locale }: { data: DayViews[]; locale: string }) {
  if (data.length === 0) return null
  const W = 660
  const H = 110
  const gap = 2
  const barW = (W - gap * (data.length - 1)) / data.length
  const max = Math.max(1, ...data.map((d) => d.views))
  return (
    <svg class="dash-chart-svg" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" role="img" aria-label="views trend">
      {data.map((d, i) => {
        // Keep a 2px floor so a zero day is still visible as a baseline dot
        // rather than an empty gap that looks like missing data.
        const h = d.views === 0 ? 2 : Math.max(3, (d.views / max) * (H - 6))
        return (
          <rect
            key={d.day}
            x={i * (barW + gap)}
            y={H - h}
            width={barW}
            height={h}
            rx={1.5}
            class={d.views > 0 ? 'dash-bar' : 'dash-bar dash-bar--zero'}
          >
            <title>{`${new Date(d.day + 'T00:00:00Z').toLocaleDateString(locale === 'zh' ? 'zh-CN' : 'en-US')} · ${d.views}`}</title>
          </rect>
        )
      })}
    </svg>
  )
}

export function AdminDashboardPage() {
  const { t, locale } = useI18n()
  const { pushToast } = useApp()
  const [, navigate] = useNav()

  const { data, loading } = useQuery(() => getDashboard(), [], {
    onError: (e) => pushToast('error', errMessage(e, t('admin.failed'))),
  })


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
              <div class="dash-card">
                <div class="dash-card-num">{data.pending_total}</div>
                <div class="dash-card-label">{t('admin.statPending')}</div>
              </div>
            </div>

            <section class="dash-block">
              <h3>{t('admin.trendTitle')}</h3>
              {data.view_trend.every((d) => d.views === 0) ? (
                <p class="muted">{t('admin.trendEmpty')}</p>
              ) : (
                <ViewsChart data={data.view_trend} locale={locale} />
              )}
              <p class="dash-hint">{t('admin.trendHint')}</p>
            </section>

            <div class="dash-cols">
              <section class="dash-col">
                <h3>{t('admin.topPosts')}</h3>
                {data.top_posts.length === 0 ? (
                  <p class="muted">{t('admin.topEmpty')}</p>
                ) : (
                  <ul class="dash-list">
                    {data.top_posts.map((p) => (
                      <li key={p.id} class="dash-list-item" onClick={() => navigate(p.slug ? `/p/${p.slug}` : `/read/${p.id}`)}>
                        <span class="dash-list-title">{p.title}</span>
                        <span class="dash-list-date">{p.view_count}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

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
