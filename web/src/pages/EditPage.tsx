import { useEffect, useState } from 'preact/hooks'
import { PostEditor } from '../ui/PostEditor'
import { RevisionList } from '../ui/RevisionList'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { getPost, updatePost, errMessage, errStatus } from '../core/api'
import { useNav } from '../core/router'
import type { Post } from '../core/types'

export function EditPage({ id }: { id: string }) {
  const { pushToast } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [post, setPost] = useState<Post | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    getPost(id)
      .then(setPost)
      .catch((e) => {
        if (errStatus(e) === 404) {
          pushToast('error', t('post.notFound'))
        } else {
          pushToast('error', t('post.loadFailed'))
        }
        navigate('/')
      })
      .finally(() => setLoading(false))
  }, [id])

  if (loading) return <p class="muted">{t('common.loading')}</p>
  if (!post) return <p class="muted">{t('post.notFound')}</p>

  return (
    <>
    <PostEditor
      id={id}
      initialTitle={post.title}
      initialBody={post.body}
      initialStatus={post.status}
      initialReviewNote={post.review_note}
      initialCategory={post.category_id}
      initialTags={post.tags?.map((tg) => tg.name)}
      initialPinned={post.pinned}
      initialCover={post.cover}
      saveLabel={t('editor.save')}
      publishing={busy}
      onCancel={() => navigate(`/read/${id}`)}
      onSubmit={async (v) => {
        setBusy(true)
        try {
          await updatePost(id, v)
          // The post is now persisted on the server; drop the local draft
          // so the next time we open it we see the saved version, not a
          // stale autosave.
          localStorage.removeItem(`echo.draft.${id}`)
          pushToast('success', t('editor.saved'))
          navigate(`/read/${id}`)
        } catch (e) {
          pushToast('error', errMessage(e, t('editor.saveFailed')))
          setBusy(false)
        }
      }}
    />
    <RevisionList postId={id} onRestored={() => navigate(`/read/${id}`)} />
    </>
  )
}
