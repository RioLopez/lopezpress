import { useI18n } from '../../../core/i18n'
import { fmtBytes, fmtDate } from '../../../core/format'
import type { UploadFileInfo } from '../../../core/types'

type Props = {
  items: UploadFileInfo[]
  busy: string | null
  selected: Set<string>
  onToggle: (name: string) => void
  onOpen: (item: UploadFileInfo) => void
}

// Dense table view: the same rows as the grid, for scanning size/date/ref
// count across many files. Actions live in the detail sheet so the row stays
// narrow; the checkbox is the bulk-delete entry point.
export function MediaList(p: Props) {
  const { t } = useI18n()
  const kindLabel: Record<string, string> = {
    post: t('media.kindPost'),
    avatar: t('media.kindAvatar'),
    site: t('media.kindSite'),
  }
  return (
    <div class="media-table-wrap">
      <table class="media-table admin-table--cards">
        <thead>
          <tr>
            <th />
            <th>{t('media.originalName')}</th>
            <th>{t('media.kindAll')}</th>
            <th>{t('media.owner')}</th>
            <th>{t('media.dimensions')}</th>
            <th>{t('media.refs')}</th>
            <th>{t('media.uploaded')}</th>
          </tr>
        </thead>
        <tbody>
          {p.items.map((it) => (
            <tr key={it.name} class={it.deleted ? 'media-row--deleted' : ''}>
              <td class="col-check">
                <input
                  type="checkbox"
                  checked={p.selected.has(it.name)}
                  onChange={() => p.onToggle(it.name)}
                  aria-label={it.original_name || it.name}
                />
              </td>
              <td class="card-title">
                <button class="media-row-open" onClick={() => p.onOpen(it)}>
                  <img src={it.thumb_url || it.url} alt="" loading="lazy" />
                  <span class="media-mono">{it.original_name || it.name}</span>
                </button>
              </td>
              <td data-label={t('media.kindAll')}>{kindLabel[it.kind] || it.kind}</td>
              <td data-label={t('media.owner')}>{it.owner_name || t('media.ownerLegacy')}</td>
              <td data-label={t('media.dimensions')}>
                {it.width > 0 ? `${it.width}×${it.height}` : '—'} · {fmtBytes(it.size)}
              </td>
              <td data-label={t('media.refs')}>{it.ref_count > 0 ? it.ref_count : '—'}</td>
              <td data-label={t('media.uploaded')}>{fmtDate(it.created_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {p.busy && <span class="muted">{t('common.loading')}</span>}
    </div>
  )
}
