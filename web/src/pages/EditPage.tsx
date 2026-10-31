import { useEffect, useState } from 'preact/hooks'
import { PostEditor } from '../ui/PostEditor'
import { RevisionList } from '../ui/RevisionList'
import { useApp } from '../core/app'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { getPost, updatePost, errMessage, errStatus } from '../core/api'
import { useNav } from '../core/router'
import { isRoot } from '../core/roles'
import type { Post } from '../core/types'

export function EditPage({ id }: { id: string }) {
  const { pushToast } = useApp()
  const { user } = useAuth()
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

  // R5-05: editing text is author-or-root only (the backend enforces it). The
  // list page already hides the link for an admin looking at someone else's
  // post, but /edit/{id} is a plain URL — without this check an admin lands in
  // a fully rendered editor and only discovers the refusal when the save comes
  // back 403, which reads like a bug rather than a permission rule.
  if (user && post.author?.id !== user.id && !isRoot(user.role)) {
    return (
      <div class="card">
        <p class="muted">{t('editor.notYourPost')}</p>
        <button class="btn" onClick={() => navigate(`/read/${id}`)}>
          {t('common.back')}
        </button>
      </div>
    )
  }

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
      publishing={busy}
      onCancel={() => navigate(`/read/${id}`)}
      onSubmit={async (v) => {
        setBusy(true)
        try {
          const saved = await updatePost(id, v)
          // The post is now persisted on the server; drop the local draft
          // so the next time we open it we see the saved version, not a
          // stale autosave.
          localStorage.removeItem(`echo.draft.${id}`)
          // Rewriting a live article sends it back to the queue, and the
          // server answers with that status. Telling the author "已保存" would
          // leave them wondering why the article vanished from the site.
          pushToast('success', saved.status === 'pending' ? t('editor.requeued') : t('editor.saved'))
          navigate(`/read/${id}`)
        } catch (e) {
          // 409 is not in the shared status→text table because it is not
          // mechanical: the same status also carries "username already in
          // use". Here the meaning is unambiguous — the row moved under us
          // (a reviewer approved or banned it mid-edit) — and the server's
          // English sentence is the whole explanation, so say it in the
          // reader's language.
          pushToast(
            'error',
            errStatus(e) === 409 ? t('editor.conflictReload') : errMessage(e, t('editor.saveFailed')),
          )
          setBusy(false)
        }
      }}
    />
    <RevisionList postId={id} onRestored={() => navigate(`/read/${id}`)} />
    </>
  )
}
