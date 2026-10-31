import { useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { MyHomepage } from './personal/MyHomepage'
import { ContentManage } from './personal/ContentManage'
import { SecurityCenter } from './personal/SecurityCenter'

// Personal center. Current user's own page with a left sidebar
// (PC) / top tabs (mobile, reserved) and three sections: My Homepage,
// Content, Security. Each section lives in pages/personal/ to keep this
// shell small; PC-first, mobile degrades via CSS.

type Tab = 'home' | 'manage' | 'security'

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
