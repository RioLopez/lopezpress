import { useI18n } from '../../../core/i18n'

export type MediaView = 'grid' | 'list'
export type StatusFilter = 'live' | 'deleted' | 'all'
export type RefFilter = '' | 'used' | 'unused'

type Props = {
  status: StatusFilter
  kind: string
  refStatus: RefFilter
  view: MediaView
  pageCount: number
  selectedCount: number
  allSelected: boolean
  onStatus: (v: StatusFilter) => void
  onKind: (v: string) => void
  onRefStatus: (v: RefFilter) => void
  onView: (v: MediaView) => void
  onToggleAll: () => void
}

// Filter and view controls. Every setter resets paging in the page component,
// so the toolbar itself only reports the chosen value.
export function MediaToolbar(p: Props) {
  const { t } = useI18n()
  return (
    <div class="media-filters">
      <select value={p.status} onChange={(e) => p.onStatus((e.target as HTMLSelectElement).value as StatusFilter)}>
        <option value="live">{t('media.statusLive')}</option>
        <option value="deleted">{t('media.statusDeleted')}</option>
        <option value="all">{t('media.statusAll')}</option>
      </select>
      <select value={p.kind} onChange={(e) => p.onKind((e.target as HTMLSelectElement).value)}>
        <option value="">{t('media.kindAll')}</option>
        <option value="post">{t('media.kindPost')}</option>
        <option value="avatar">{t('media.kindAvatar')}</option>
        <option value="site">{t('media.kindSite')}</option>
      </select>
      <select value={p.refStatus} onChange={(e) => p.onRefStatus((e.target as HTMLSelectElement).value as RefFilter)}>
        <option value="">{t('media.refsFilterAll')}</option>
        <option value="used">{t('media.refsUsed')}</option>
        <option value="unused">{t('media.refsUnused')}</option>
      </select>
      <div class="media-view-toggle">
        <button class={`btn btn-ghost btn-sm${p.view === 'grid' ? ' is-active' : ''}`} onClick={() => p.onView('grid')}>
          {t('media.viewGrid')}
        </button>
        <button class={`btn btn-ghost btn-sm${p.view === 'list' ? ' is-active' : ''}`} onClick={() => p.onView('list')}>
          {t('media.viewList')}
        </button>
      </div>
      {p.pageCount > 0 && (
        <label class="media-selectall">
          <input type="checkbox" checked={p.allSelected} onChange={p.onToggleAll} />
          {t('media.selectAll')}
        </label>
      )}
      {p.selectedCount > 0 && (
        <span class="muted">{t('media.selectedCount', { n: String(p.selectedCount) })}</span>
      )}
    </div>
  )
}
