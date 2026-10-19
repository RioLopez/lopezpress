import { useEffect, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { getPost, deletePost, getPostPrevNext, ApiError } from '../core/api'
import type { Post, PostNav, FontSize } from '../core/types'
import { useAuth } from '../core/auth'
import { renderMD, wordCount, extractTOC } from '../core/md'
import { useNav } from '../core/router'

const FONT_KEY = 'echo.readFont'

// Rough reading speed (words per minute) used to estimate reading time.
const WPM = 200

function detectFont(): FontSize {
  const saved = localStorage.getItem(FONT_KEY) as FontSize | null
  if (saved === 'small' || saved === 'medium' || saved === 'large' || saved === 'xlarge') return saved
  return 'medium'
}

export function ReadPage({ id }: { id: string }) {
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const { user } = useAuth()
  const [, navigate] = useNav()
  const [post, setPost] = useState<Post | null>(null)
  const [nav, setNav] = useState<PostNav | null>(null)
  const [loading, setLoading] = useState(true)
  const [fontSize, setFontSizeState] = useState<FontSize>(detectFont)
  const [progress, setProgress] = useState(0)
  // TOC: table of contents extracted from h2/h3 headings. Collapsible.
  const [tocOpen, setTocOpen] = useState(
    () => !window.matchMedia('(max-width: 768px)').matches,
  )
  // Scroll-spy: id of the heading currently crossing the top of the viewport.
  const [activeHeading, setActiveHeading] = useState<string | null>(null)
  // Back-to-top: shows once the page scrolled past this many px.
  const [showTop, setShowTop] = useState(false)
  // Lightbox: src of enlarged image, or null when closed.
  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null)

  const setFontSize = (f: FontSize) => {
    localStorage.setItem(FONT_KEY, f)
    setFontSizeState(f)
  }

  // Reading progress + scroll-spy + back-to-top, all driven by the same scroll
  // event to avoid duplicate listeners. Scroll-spy is throttled via rAF mark.
  useEffect(() => {
    if (!post) return
    let ticking = false
    const onScroll = () => {
      const el = document.documentElement
      const max = el.scrollHeight - el.clientHeight
      setProgress(max > 0 ? Math.min(100, Math.max(0, (el.scrollTop / max) * 100)) : 0)
      setShowTop(el.scrollTop > 400)
      if (ticking) return
      ticking = true
      requestAnimationFrame(() => {
        ticking = false
        // Find the last heading whose top has crossed above the sticky offset.
        const heads = Array.from(
          document.querySelectorAll<HTMLElement>('.read-body.md-body h2, .read-body.md-body h3'),
        )
        let current: string | null = null
        // ~72px sticky header offset + small buffer.
        const threshold = 80
        for (const h of heads) {
          if (h.getBoundingClientRect().top <= threshold) current = h.id
          else break
        }
        setActiveHeading(current)
      })
    }
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      window.removeEventListener('scroll', onScroll)
      ticking = false
    }
  }, [post])

  // Middle-click image lightbox
  useEffect(() => {
    if (!post) return
    const imgs = Array.from(
      document.querySelectorAll<HTMLImageElement>('.read-body.md-body img'),
    )
    imgs.forEach((img, i) => {
      if (i > 0 && !img.hasAttribute('loading')) img.loading = 'lazy'
      const src = img.getAttribute('src')
      if (!src || img.classList.contains('md-img-broken')) return
      img.classList.add('read-zoomable')
      img.addEventListener('click', () => setLightboxSrc(src))
    })
    return () => {
      imgs.forEach((img) => img.classList.remove('read-zoomable'))
    }
  }, [post])

  // Close lightbox with Escape; prevent body scroll while open.
  useEffect(() => {
    if (!lightboxSrc) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setLightboxSrc(null)
    }
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = previous
      window.removeEventListener('keydown', onKey)
    }
  }, [lightboxSrc])

  useEffect(() => {
    setLoading(true)
    getPost(id)
      .then((p) => {
        setPost(p)
        getPostPrevNext(id).then(setNav).catch(() => {})
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          pushToast('error', t('post.notFound'))
        } else {
          pushToast('error', t('post.loadFailed'))
        }
        navigate('/')
      })
      .finally(() => setLoading(false))
  }, [id])

  // Inject a copy control onto every code block in the article. The buttons
  // are appended imperatively so the sanitized Markdown body can stay pure.
  useEffect(() => {
    if (!post) return
    const blocks = Array.from(document.querySelectorAll<HTMLPreElement>('.read-body.md-body pre'))
    blocks.forEach((pre) => {
      if (pre.closest('.code-copy-holder')) return
      const holder = document.createElement('span')
      holder.className = 'code-copy-holder'
      pre.parentNode?.insertBefore(holder, pre)
      holder.appendChild(pre)
      const btn = document.createElement('button')
      btn.type = 'button'
      btn.className = 'code-copy'
      btn.title = t('read.copyCode')
      btn.textContent = t('read.copyCode')
      holder.appendChild(btn)
      const codeEl = pre.querySelector('code')
      const raw = codeEl ? codeEl.textContent || '' : pre.textContent || ''
      const revert = () => {
        btn.textContent = t('read.copyCode')
        btn.classList.remove('code-copy--done')
      }
      btn.addEventListener('click', async () => {
        try {
          if (navigator.clipboard?.writeText) {
            await navigator.clipboard.writeText(raw)
          } else {
            const ta = document.createElement('textarea')
            ta.value = raw
            ta.style.position = 'fixed'
            ta.style.opacity = '0'
            document.body.appendChild(ta)
            ta.select()
            document.execCommand('copy')
            document.body.removeChild(ta)
          }
          btn.textContent = t('read.codeCopied')
          btn.classList.add('code-copy--done')
        } catch {
          pushToast('error', t('admin.failed'))
        }
        window.setTimeout(revert, 1500)
      })
    })
    return () => {
      Array.from(document.querySelectorAll('.code-copy-holder')).forEach((h) => h.remove())
    }
  }, [post, t, pushToast])

  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'

  if (loading) return <div class="read-page"><p class="muted">{t('common.loading')}</p></div>
  if (!post) return <div class="read-page"><p class="muted">{t('post.notFound')}</p></div>

  const canEdit = user && (user.id === post.author_id || user.role === 'admin' || user.role === 'root')
  const canDelete = canEdit

  const remove = async () => {
    if (!(await confirmDialog(t('post.deleteConfirm', { t: post.title })))) return
    try {
      await deletePost(id)
      pushToast('success', t('post.deleted'))
      navigate('/')
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('post.deleteConfirm', { t: post.title }))
    }
  }

  const words = wordCount(post.body || '')
  const readMins = Math.max(1, Math.round(words / WPM))
  const toc = post ? extractTOC(post.body || '') : []

  const fontOrder: FontSize[] = ['small', 'medium', 'large', 'xlarge']
  const fontLabel = (f: FontSize) =>
    f === 'small' ? t('read.fontSmall') : f === 'medium' ? t('read.fontMedium') : f === 'large' ? t('read.fontLarge') : t('read.fontXlarge')

  const jumpToAnchor = (id: string) => {
    const el = document.getElementById(id)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  return (
    <article class={`read-page read-page--${fontSize}`}>
      <div class="read-progress" style={{ width: `${progress}%` }} aria-hidden="true" />
      <header class="read-head">
        <div class="read-badges">
          {post.pinned && <span class="badge-pinned">{t('post.pinned')}</span>}
          {post.status !== 'published' && (
            <span class={`post-status post-status-${post.status}`}>
              {t(`admin.statusLabel.${post.status}`)}
            </span>
          )}
        </div>
        <h1>{post.title}</h1>
        <p class="read-meta">
          {t('list.author')}:{' '}
          <a
            class="read-author"
            onClick={() => post.author?.username && navigate(`/u/${post.author.username}`)}
          >
            {post.author?.username || post.author_id}
          </a>
          <span> &middot; {new Date(post.created_at).toLocaleString(localeTag)}</span>
          {post.updated_at !== post.created_at && (
            <span> &middot; {t('post.updated')} {new Date(post.updated_at).toLocaleString(localeTag)}</span>
          )}
          <span> &middot; {words} {t('post.words')}</span>
          <span> &middot; {post.view_count} {t('post.views')}</span>
          <span> &middot; {t('read.read', { n: String(readMins) })}</span>
        </p>
        {(post.category?.name || (post.tags?.length || 0) > 0) && (
          <div class="read-chips">
            {post.category?.name && <span class="post-cat-chip">{post.category.name}</span>}
            {post.tags?.map((tg) => (
              <span key={tg.id} class="post-tag-chip">{tg.name}</span>
            ))}
          </div>
        )}
        <div class="read-controls">
          <div class="font-controls">
            <span class="font-controls-label">{t('read.font')}</span>
            {fontOrder.map((f) => (
              <button
                key={f}
                class={`font-btn font-btn--${f} ${fontSize === f ? 'font-btn--active' : ''}`}
                title={fontLabel(f)}
                onClick={() => setFontSize(f)}
              >
                A<span class="font-btn-sub">{fontLabel(f)}</span>
              </button>
            ))}
            <button class="font-btn font-btn--reset" title={t('read.reset')} onClick={() => setFontSize('medium')}>
              {t('read.reset')}
            </button>
          </div>
        </div>
        {canEdit && (
          <div class="read-actions">
            <button class="btn btn-ghost btn-sm" onClick={() => navigate(`/edit/${id}`)}>
              {t('list.edit')}
            </button>
            {canDelete && (
              <button class="btn btn-danger btn-sm" onClick={remove}>{t('list.del')}</button>
            )}
          </div>
        )}
      </header>
      {toc.length > 0 && (
        <nav
          class={`read-toc ${tocOpen ? 'read-toc--open' : ''}`}
          aria-label={t('read.tocTitle')}
        >
          <button class="read-toc-toggle" onClick={() => setTocOpen((o) => !o)}>
            <span>{t('read.tocTitle')}</span>
            <span class="read-toc-caret" aria-hidden="true">
              ▼ {tocOpen ? t('read.tocClose') : t('read.tocOpen')}
            </span>
          </button>
          {tocOpen && (
            <ul class="read-toc-list">
              {toc.map((item, i) => (
                <li
                  key={`${item.id}-${i}`}
                  class={`read-toc-item read-toc-item--h${item.depth} ${
                    activeHeading === item.id ? 'read-toc-item--active' : ''
                  }`}
                >
                  <a
                    href={`#${item.id}`}
                    onClick={(e) => {
                      e.preventDefault()
                      jumpToAnchor(item.id)
                    }}
                  >
                    {item.text}
                  </a>
                </li>
              ))}
            </ul>
          )}
        </nav>
      )}
      <div
        class="read-body md-body"
        dangerouslySetInnerHTML={{ __html: renderMD(post.body || '') }}
      />
      <div class="read-footer">
        <div class="read-nav">
          <button
            class="btn btn-ghost read-nav-btn"
            disabled={!nav?.prev}
            onClick={() => nav?.prev && navigate(`/read/${nav.prev!.id}`)}
          >
            {nav?.prev ? `← ${nav.prev.title}` : t('post.prevPost')}
          </button>
          <button
            class="btn btn-ghost read-nav-btn"
            disabled={!nav?.next}
            onClick={() => nav?.next && navigate(`/read/${nav.next!.id}`)}
          >
            {nav?.next ? `${nav.next.title} →` : t('post.nextPost')}
          </button>
        </div>
        <button class="btn btn-ghost" onClick={() => navigate('/')}>{t('post.backToList')}</button>
      </div>
      {showTop && (
        <button
          class="btn btn-ghost back-to-top"
          aria-label={t('read.backToTop')}
          onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
        >
          ↑
        </button>
      )}
      {lightboxSrc && (
        <div class="lightbox" onClick={() => setLightboxSrc(null)} role="dialog" aria-label={t('read.lightboxClose')}>
          <img src={lightboxSrc} alt="" />
        </div>
      )}
    </article>
  )
}
