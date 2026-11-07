import { useI18n } from '../../../core/i18n'
import { fmtBytes, fmtDate } from '../../../core/format'
import { useQuery } from '../../../core/useQuery'
import { listUploadRefs } from '../../../core/api'
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
  // Fetched per open: the ledger can change between visits, and a stale list
  // is worse than one extra request behind a click.
  const { data: refs, loading: refsLoading } = useQuery(
    () => listUploadRefs(d.name),
    [d.name],
  )
  const kindLabel: Record<string, string> = {
    body: t('media.refKindBody'),
    avatar: t('media.refKindAvatar'),
    site: t('media.refKindSite'),
  }
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
        <div class="media-refs">
          <h4>{t('media.refsTitle')}</h4>
          {refsLoading && <p class="muted">{t('common.loading')}</p>}
          {!refsLoading && refs && refs.length > 0 && (
            <ul>
              {refs.map((r, i) => (
                <li key={`${r.kind}-${r.post_id}-${r.user_id}-${i}`}>
                  <span class="media-ref-kind">{kindLabel[r.kind] || r.kind}</span>
                  {r.post_title && <span>{r.post_title}</span>}
                  {r.user_name && <span class="muted"> · {r.user_name}</span>}
                </li>
              ))}
            </ul>
          )}
          {!refsLoading && refs && refs.length === 0 && <p class="muted">{t('media.refsEmpty')}</p>}
        </div>
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
