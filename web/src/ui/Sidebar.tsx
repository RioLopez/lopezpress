import { useEffect, useState, useCallback, useMemo } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { listCategories, listTags, getRecentPosts } from '../core/api'
import { useNav } from '../core/router'
import type { Category, Tag, Post } from '../core/types'

interface SidebarProps {
  activeCategory: number | null
  activeTag: string | null
  onCategorySelect: (id: number | null) => void
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

  // Spec 27: categories are a two-level tree. Flatten to parents-then-children
  // so the sidebar keeps its simple <ul> while showing hierarchy.
  const ordered = useMemo(() => {
    const roots = categories.filter((c) => c.parent_id == null)
    const out: { cat: Category; depth: number }[] = []
    for (const root of roots) {
      out.push({ cat: root, depth: 0 })
      for (const child of categories.filter((c) => c.parent_id === root.id)) {
        out.push({ cat: child, depth: 1 })
      }
    }
    // Orphans (parent row missing) still need to be reachable.
    for (const cat of categories) {
      if (cat.parent_id != null && !categories.some((p) => p.id === cat.parent_id)) {
        out.push({ cat, depth: 0 })
      }
    }
    return out
  }, [categories])

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
            <li key={c.id} class={depth === 1 ? 'sidebar-item--child' : ''}>
              <a
                href="#"
                class={`${linkBase} ${activeCategory === c.id ? 'sidebar-link--active' : ''}`}
                onClick={(e) => { e.preventDefault(); onCategorySelect(c.id) }}
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
