import { useI18n } from '../core/i18n'
import { categoryOptionLabel, type CategoryNode } from '../core/taxonomy'
import type { PostStatus } from '../core/types'

interface Props {
  search: string
  onSearchChange: (value: string) => void
  activeCategory: string | null
  onCategorySelect: (value: string | null) => void
  status: PostStatus | ''
  onStatusChange: (value: PostStatus | '') => void
  categoryNodes: CategoryNode[]
}

/**
 * Public list filter bar. Kept out of ListPage so the page stays under the
 * repository's file-size gate while the mobile grid rules live in CSS.
 */
export function ListToolbar(props: Props) {
  const { t } = useI18n()
  return (
    <div class="list-toolbar">
      <input
        class="search-input"
        type="search"
        placeholder={t('list.searchPlaceholder')}
        value={props.search}
        onInput={(e) => props.onSearchChange((e.target as HTMLInputElement).value)}
      />
      <select
        value={props.activeCategory ?? ''}
        onInput={(e) => props.onCategorySelect((e.target as HTMLSelectElement).value || null)}
      >
        <option value="">{t('list.allCategories')}</option>
        {/* Options cannot be styled, so the two-level tree is spelled with an
            indent + └ prefix. Values are names, matching the URL and sidebar. */}
        {props.categoryNodes.map((node) => (
          <option key={node.cat.id} value={node.cat.name}>
            {categoryOptionLabel(node)}
          </option>
        ))}
      </select>
      <select
        value={props.status}
        onInput={(e) => props.onStatusChange((e.target as HTMLSelectElement).value as PostStatus | '')}
      >
        <option value="">{t('list.allStatus')}</option>
        <option value="published">{t('list.published')}</option>
        <option value="private">{t('list.private')}</option>
      </select>
    </div>
  )
}
