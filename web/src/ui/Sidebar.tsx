import { useEffect, useState, useCallback } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { listCategories, listTags, getRecentPosts } from '../core/api'
import { useNav } from '../core/router'
import { orderCategories } from '../core/taxonomy'
import type { Category, Tag, Post } from '../core/types'

interface SidebarProps {
  // Category NAME (see ListPage): the sidebar highlights by name so it agrees
  // with the URL and with the filter dropdown.
  activeCategory: string | null
  activeTag: string | null
  onCategorySelect: (name: string | null) => void
  onTagSelect: (name: string | null) => void
}

export function Sidebar({ activeCategory, activeTag, onCategorySelect, onTagSelect }: SidebarProps) {
  const { t, locale } = useI18n()
  const [, navigate] = useNav()
  const [categories, setCategories] = useState<Category[]>([])
  const [tags, setTags] = useState<Tag[]>([])
  const [recent, setRecent] = useState<Post[]>([])

  const load = useCallback(() => {
    Promise.all([listCategories(), listTags(), getRecentPosts()])
      .then(([cats, tgs, rpts]) => {
        setCategories(cats)
        setTags(tgs)
        setRecent(rpts)
      })
      .catch(() => {
        // Sidebar data is non-critical: any failure (including the global
        // 401 already handled by api.ts) is swallowed so the layout still
        // renders. No-op by design.
      })
  }, [])

  useEffect(() => { load() }, [load])

  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'
  const linkBase = 'sidebar-link'

  // Spec 27: categories are a two-level tree. The flattening lives in
  // core/taxonomy so the sidebar, the list filter and the card breadcrumbs all
  // agree on the order.
  const ordered = orderCategories(categories)

  return (
    <aside class="sidebar">
      <section class="sidebar-section">
        <h3 class="sidebar-title">{t('sidebar.categories')}</h3>
        <ul class="sidebar-list">
          <li>
            <a
              href="#"
              class={`${linkBase} ${activeCategory === null ? 'sidebar-link--active' : ''}`}
              onClick={(e) => { e.preventDefault(); onCategorySelect(null) }}
            >{t('sidebar.allCategories')}</a>
          </li>
          {ordered.map(({ cat: c, depth }) => (
            // The class carries the hierarchy: parents get a heavier weight,
            // children get a real indent plus a guide line. Before this the
            // only difference was opacity .9, which nobody could see, so a
            // child category read as a sibling.
            <li key={c.id} class={depth === 1 ? 'sidebar-item--child' : 'sidebar-item--parent'}>
              <a
                href="#"
                class={`${linkBase} ${depth === 1 ? 'sidebar-link--child' : ''} ${activeCategory === c.name ? 'sidebar-link--active' : ''}`}
                onClick={(e) => { e.preventDefault(); onCategorySelect(c.name) }}
              >{c.name}</a>
            </li>
          ))}
        </ul>
      </section>

      <section class="sidebar-section">
        <h3 class="sidebar-title">{t('sidebar.tags')}</h3>
        <div class="tag-cloud">
          {tags.length === 0 && <span class="muted">—</span>}
          {tags.map((tg) => (
            <button
              key={tg.id}
              type="button"
              class={`tag-pill ${activeTag === tg.name ? 'tag-pill--active' : ''}`}
              onClick={() => onTagSelect(activeTag === tg.name ? null : tg.name)}
            >
              {tg.name}
            </button>
          ))}
        </div>
      </section>

      <section class="sidebar-section">
        <h3 class="sidebar-title">{t('sidebar.recent')}</h3>
        <ul class="recent-list">
          {recent.length === 0 && <li class="muted">—</li>}
          {recent.map((p) => (
            <li key={p.id} class="recent-item" onClick={() => navigate(`/read/${p.id}`)}>
              <span class="recent-title">{p.title}</span>
              <span class="recent-date">
                {new Date(p.created_at).toLocaleDateString(localeTag)}
              </span>
            </li>
          ))}
        </ul>
      </section>
    </aside>
  )
}
