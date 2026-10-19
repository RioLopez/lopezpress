import { useState } from 'preact/hooks'
import { PostEditor } from '../ui/PostEditor'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { createPost, ApiError } from '../core/api'
import { useNav } from '../core/router'

export function CreatePage() {
  const { user } = useAuth()
  const { pushToast } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [busy, setBusy] = useState(false)

  if (!user) {
    return (
      <p class="muted">
        {t('list.emptyLoggedOut')} <a href="#" onClick={(e) => { e.preventDefault(); navigate('/login') }}>{t('auth.loginBtn')}</a>
      </p>
    )
  }

  return (
    <PostEditor
      saveLabel={t('editor.publish')}
      publishing={busy}
      onCancel={() => navigate('/')}
      onSubmit={async (v) => {
        setBusy(true)
        try {
          const p = await createPost({
            title: v.title,
            body: v.body,
            status: v.status,
            category_id: v.category_id,
            tags: v.tags,
            pinned: v.pinned,
            cover: v.cover,
          })
          // Drop the anonymous draft (CreatePage has no id) so the next
          // visit to /create always starts from a clean slate.
          localStorage.removeItem('echo.draft')
          pushToast('success', t('editor.created'))
          navigate(`/read/${p.id}`)
        } catch (e) {
          pushToast('error', e instanceof ApiError ? e.message : t('editor.createFailed'))
          setBusy(false)
        }
      }}
    />
  )
}
