import { useI18n } from '../../../core/i18n'
import { fmtBytes, fmtDate } from '../../../core/format'
import type { UploadFileInfo } from '../../../core/types'

type Props = {
  item: UploadFileInfo
  busy: string | null
  onClose: () => void
  onWithdraw: (item: UploadFileInfo) => void
  onRestore: (item: UploadFileInfo) => void
  onRetryThumb: (item: UploadFileInfo) => void
}

// Single-file detail sheet: everything an operator needs before removing an
// asset, plus the thumbnail retry when the render failed.
export function MediaDetail(p: Props) {
  const { t } = useI18n()
  const d = p.item
  return (
    <div class="modal-backdrop" onClick={p.onClose}>
      <div class="modal media-detail" onClick={(e) => e.stopPropagation()}>
        <img src={d.url} alt="" class="media-detail-img" />
        <dl class="media-detail-list">
          <dt>{t('media.uploaded')}</dt>
          <dd>{fmtDate(d.created_at)}</dd>
          <dt>{t('media.owner')}</dt>
          <dd>{d.owner_name || t('media.ownerLegacy')}</dd>
          <dt>{t('media.dimensions')}</dt>
          <dd>
            {d.width > 0 ? `${d.width} × ${d.height}` : '—'}
            {' · '}
            {fmtBytes(d.size)}
          </dd>
          <dt>{t('media.refs')}</dt>
          <dd>{d.ref_count > 0 ? d.ref_count : t('media.refsNone')}</dd>
          <dt>{t('media.originalName')}</dt>
          <dd class="media-mono">{d.original_name || '—'}</dd>
          <dt>{t('media.sourceIp')}</dt>
          <dd class="media-mono">{d.ip || '—'}</dd>
          <dt>{t('media.sha256')}</dt>
          <dd class="media-mono media-hash">{d.sha256 || '—'}</dd>
        </dl>
        <div class="modal-foot">
          <button class="btn btn-ghost" onClick={p.onClose}>
            {t('media.close')}
          </button>
          {!d.deleted && d.thumb_status === 'failed' && (
            <button class="btn btn-ghost" disabled={p.busy === d.name} onClick={() => p.onRetryThumb(d)}>
              {t('media.thumbRetry')}
            </button>
          )}
          {d.deleted ? (
            <button class="btn btn-primary" disabled={p.busy === d.name} onClick={() => p.onRestore(d)}>
              {t('media.restore')}
            </button>
          ) : (
            <button class="btn btn-primary" disabled={p.busy === d.name} onClick={() => p.onWithdraw(d)}>
              {t('media.withdraw')}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
