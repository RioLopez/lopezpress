import { useI18n } from '../../../core/i18n'
import { fmtBytes } from '../../../core/format'
import type { UploadFileInfo } from '../../../core/types'

type Props = {
  items: UploadFileInfo[]
  busy: string | null
  selected: Set<string>
  onToggle: (name: string) => void
  onOpen: (item: UploadFileInfo) => void
  onWithdraw: (item: UploadFileInfo) => void
  onPurge: (item: UploadFileInfo) => void
  onRestore: (item: UploadFileInfo) => void
  onRetryThumb: (item: UploadFileInfo) => void
}

// Grid of library images with the per-file actions. Each card shows size and
// reference count because those are the two facts that decide whether it is
// safe to remove; the thumbnail doubles as the detail trigger.
export function MediaGrid(p: Props) {
  const { t } = useI18n()
  return (
    <div class="image-grid media-grid">
      {p.items.map((it) => (
        <div class={`image-card${it.deleted ? ' image-card--deleted' : ''}`} key={it.name}>
          <label class="media-check" onClick={(e) => e.stopPropagation()}>
            <input
              type="checkbox"
              checked={p.selected.has(it.name)}
              onChange={() => p.onToggle(it.name)}
              aria-label={it.original_name || it.name}
            />
          </label>
          <button class="image-card-thumb" onClick={() => p.onOpen(it)} title={t('media.detail')}>
            <img src={it.thumb_url || it.url} alt={it.original_name || it.name} loading="lazy" />
          </button>
          <div class="image-card-meta">
            <span>{fmtBytes(it.size)}</span>
            <span title={t('media.refs')}>
              {it.ref_count > 0 ? `↩ ${it.ref_count}` : '—'}
            </span>
          </div>
          <div class="image-card-actions">
            {it.deleted ? (
              <>
                <button class="btn btn-ghost btn-sm" disabled={p.busy === it.name} onClick={() => p.onRestore(it)}>
                  {t('media.restore')}
                </button>
                <button class="btn btn-ghost btn-sm" disabled={p.busy === it.name} onClick={() => p.onPurge(it)}>
                  {t('media.purge')}
                </button>
              </>
            ) : (
              <>
                <button class="btn btn-ghost btn-sm" disabled={p.busy === it.name} onClick={() => p.onWithdraw(it)}>
                  {t('media.withdraw')}
                </button>
                <button class="btn btn-ghost btn-sm" disabled={p.busy === it.name} onClick={() => p.onPurge(it)}>
                  {t('media.purge')}
                </button>
                {it.thumb_status === 'failed' && (
                  <button class="btn btn-ghost btn-sm" disabled={p.busy === it.name} onClick={() => p.onRetryThumb(it)}>
                    {t('media.thumbRetry')}
                  </button>
                )}
              </>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}
