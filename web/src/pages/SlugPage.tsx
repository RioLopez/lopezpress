import { useEffect, useState } from 'preact/hooks'
import { getPostBySlug } from '../core/api'
import { useI18n } from '../core/i18n'
import { ReadPage } from './ReadPage'
import { NotFoundPage } from './NotFoundPage'

// Spec 29: `/p/{slug}` resolves the permanent link to a post id, then hands
// off to the existing ReadPage. Keeping the resolution here means ReadPage
// stays single-purpose (id-based) and the UUID route keeps working unchanged.
export function SlugPage({ slug }: { slug: string }) {
  const { t } = useI18n()
  const [id, setId] = useState<string | null>(null)
  const [missing, setMissing] = useState(false)

  useEffect(() => {
    let alive = true
    setId(null)
    setMissing(false)
    getPostBySlug(slug)
      .then((p) => {
        if (alive) setId(p.id)
      })
      .catch(() => {
        // 404 is the normal answer for an unpublished or unknown slug; the
        // same page as an unknown route keeps the two indistinguishable.
        if (alive) setMissing(true)
      })
    return () => {
      alive = false
    }
  }, [slug])

  if (missing) return <NotFoundPage />
  if (!id) return <div class="read-page"><p class="muted">{t('common.loading')}</p></div>
  return <ReadPage id={id} />
}
