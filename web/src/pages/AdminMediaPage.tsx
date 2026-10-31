import { useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { useQuery } from '../core/useQuery'
import { fmtBytes, fmtDate } from '../core/format'
import {
  listUploads,
  deleteUpload,
  purgeUpload,
  restoreUpload,
  uploadImage,
  errMessage,
  errStatus,
} from '../core/api'
import type { UploadFileInfo } from '../core/types'

const PAGE_SIZE = 24

type StatusFilter = 'live' | 'deleted' | 'all'

// Admin media library.
//
// This is the site owner's asset view, not a gallery for readers: every file
// with its owner, size, geometry, source IP and — the part that actually
// matters when deleting — how many articles still reference it.
//
// Two delete verbs, deliberately distinct:
//   - Withdraw (soft): invisible and un-reusable, file stays, published
//     articles keep rendering. Reversible.
//   - Delete permanently (hard): removes the bytes. Allowed even when articles
//     still embed the image, because moderating a bad picture must not require
//     the consent of everyone who used it — but the count is shown first.
export function AdminMediaPage() {
  const { t } = useI18n()
  const { pushToast } = useApp()

  const [status, setStatus] = useState<StatusFilter>('live')
  const [kind, setKind] = useState('')
  const [page, setPage] = useState(1)
  const [detail, setDetail] = useState<UploadFileInfo | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null)

  const { data, loading, reload } = useQuery(
    () => listUploads({ status, kind: kind || undefined, page, size: PAGE_SIZE }),
    [status, kind, page],
    { onError: (e) => pushToast('error', errMessage(e, t('media.deleteFailed'))) },
  )

  const after = async (msg: string) => {
    pushToast('info', msg)
    await reload()
  }

  // One file at a time on purpose. The server allows 20 uploads a minute per
  // account (internal/services/upload_limits.go); firing fifty in parallel
  // would spend the whole allowance on 429s and give the operator no idea what
  // happened. A 429 here means "wait", not "no" — the limiter sits outside the
  // handler, so the file was never received — and waiting out the refill is
  // what makes importing an existing library possible unattended.
  const uploadOne = async (file: File) => {
    for (let attempt = 0; ; attempt++) {
      try {
        return await uploadImage(file, 'post')
      } catch (e) {
        if (errStatus(e) === 429 && attempt < 5) {
          await new Promise((r) => setTimeout(r, 3200))
          continue
        }
        throw e
      }
    }
  }

  const onUploadMany = async (files: File[]) => {
    if (files.length === 0) return
    setUploading(true)
    setProgress({ done: 0, total: files.length })
    let added = 0
    let reused = 0
    let failed = 0
    for (let i = 0; i < files.length; i++) {
      try {
        const r = await uploadOne(files[i])
        if (r.deduped) reused++
        else added++
      } catch {
        failed++
      }
      setProgress({ done: i + 1, total: files.length })
    }
    setUploading(false)
    setProgress(null)
    // One summary instead of one toast per file: a batch of fifty would
    // otherwise bury the page in notifications.
    const parts: string[] = []
    if (added) parts.push(t('media.batchAdded', { n: String(added) }))
    if (reused) parts.push(t('media.batchReused', { n: String(reused) }))
    if (failed) parts.push(t('media.batchFailed', { n: String(failed) }))
    pushToast(failed ? 'error' : 'info', parts.join(' · '))
    await reload()
  }

  const withdraw = async (item: UploadFileInfo) => {
    if (!confirm(t('media.withdrawConfirm'))) return
    setBusy(item.name)
    try {
      await deleteUpload(item.name)
      setDetail(null)
      await after(t('media.deleted'))
    } catch (e) {
      pushToast('error', errMessage(e, t('media.deleteFailed')))
    } finally {
      setBusy(null)
    }
  }

  const purge = async (item: UploadFileInfo) => {
    const warn = item.ref_count > 0
      ? t('media.purgeWithRefs', { n: String(item.ref_count) })
      : t('media.purgeConfirm')
    if (!confirm(warn)) return
    setBusy(item.name)
    try {
      await purgeUpload(item.name)
      setDetail(null)
      await after(t('media.deleted'))
    } catch (e) {
      pushToast('error', errMessage(e, t('media.deleteFailed')))
    } finally {
      setBusy(null)
    }
  }

  const restore = async (item: UploadFileInfo) => {
    setBusy(item.name)
    try {
      await restoreUpload(item.name)
      setDetail(null)
      await after(t('media.restored'))
    } catch (e) {
      pushToast('error', errMessage(e, t('media.deleteFailed')))
    } finally {
      setBusy(null)
    }
  }

  const stats = data?.stats
  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.size)) : 1

  return (
    <div class="admin-page">
      <div class="media-head">
        <div>
          <h1>{t('media.title')}</h1>
          <p class="muted">{t('media.sub')}</p>
        </div>
        <label class="btn btn-primary btn-sm">
          {uploading
            ? progress
              ? `${progress.done}/${progress.total}`
              : t('common.loading')
            : t('media.upload')}
          <input
            type="file"
            accept="image/*"
            multiple
            hidden
            disabled={uploading}
            onChange={(e) => {
              const input = e.target as HTMLInputElement
              const files = Array.from(input.files || [])
              if (files.length) onUploadMany(files)
              input.value = ''
            }}
          />
        </label>
      </div>

      {stats && data && (
        <div class="media-stats">
          <span>{t('media.siteUsed', {
            used: fmtBytes(stats.total_bytes),
            quota: fmtBytes(data.site_quota_bytes),
          })}</span>
          {stats.deleted_count > 0 && (
            <span class="media-stat-warn">
              {t('media.deletedUsed', {
                size: fmtBytes(stats.deleted_bytes),
                n: String(stats.deleted_count),
              })}
            </span>
          )}
          {stats.orphan_count > 0 && (
            <span class="muted">{t('media.orphans', { n: String(stats.orphan_count) })}</span>
          )}
        </div>
      )}

      <div class="media-filters">
        <select
          value={status}
          onChange={(e) => {
            setStatus((e.target as HTMLSelectElement).value as StatusFilter)
            setPage(1)
          }}
        >
          <option value="live">{t('media.statusLive')}</option>
          <option value="deleted">{t('media.statusDeleted')}</option>
          <option value="all">{t('media.statusAll')}</option>
        </select>
        <select
          value={kind}
          onChange={(e) => {
            setKind((e.target as HTMLSelectElement).value)
            setPage(1)
          }}
        >
          <option value="">{t('media.kindAll')}</option>
          <option value="post">{t('media.kindPost')}</option>
          <option value="avatar">{t('media.kindAvatar')}</option>
          <option value="site">{t('media.kindSite')}</option>
        </select>
      </div>

      {loading && <p class="muted">{t('media.loading')}</p>}
      {!loading && data && data.items.length === 0 && <p class="muted">{t('media.empty')}</p>}

      {data && data.items.length > 0 && (
        <div class="image-grid media-grid">
          {data.items.map((it) => (
            <div class={`image-card${it.deleted ? ' image-card--deleted' : ''}`} key={it.name}>
              <button class="image-card-thumb" onClick={() => setDetail(it)} title={t('media.detail')}>
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
                    <button class="btn btn-ghost btn-sm" disabled={busy === it.name} onClick={() => restore(it)}>
                      {t('media.restore')}
                    </button>
                    <button class="btn btn-ghost btn-sm" disabled={busy === it.name} onClick={() => purge(it)}>
                      {t('media.purge')}
                    </button>
                  </>
                ) : (
                  <>
                    <button class="btn btn-ghost btn-sm" disabled={busy === it.name} onClick={() => withdraw(it)}>
                      {t('media.withdraw')}
                    </button>
                    <button class="btn btn-ghost btn-sm" disabled={busy === it.name} onClick={() => purge(it)}>
                      {t('media.purge')}
                    </button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {data && data.total > data.size && (
        <div class="media-pager">
          <button class="btn btn-ghost btn-sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
            {t('media.prev')}
          </button>
          <span class="muted">{page} / {totalPages}</span>
          <button
            class="btn btn-ghost btn-sm"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            {t('media.next')}
          </button>
        </div>
      )}

      {detail && (
        <div class="modal-backdrop" onClick={() => setDetail(null)}>
          <div class="modal media-detail" onClick={(e) => e.stopPropagation()}>
            <img src={detail.url} alt="" class="media-detail-img" />
            <dl class="media-detail-list">
              <dt>{t('media.uploaded')}</dt>
              <dd>{fmtDate(detail.created_at)}</dd>
              <dt>{t('media.owner')}</dt>
              <dd>{detail.owner_name || t('media.ownerLegacy')}</dd>
              <dt>{t('media.dimensions')}</dt>
              <dd>
                {detail.width > 0 ? `${detail.width} × ${detail.height}` : '—'}
                {' · '}
                {fmtBytes(detail.size)}
              </dd>
              <dt>{t('media.refs')}</dt>
              <dd>{detail.ref_count > 0 ? detail.ref_count : t('media.refsNone')}</dd>
              <dt>{t('media.originalName')}</dt>
              <dd class="media-mono">{detail.original_name || '—'}</dd>
              <dt>{t('media.sourceIp')}</dt>
              <dd class="media-mono">{detail.ip || '—'}</dd>
              <dt>{t('media.sha256')}</dt>
              <dd class="media-mono media-hash">{detail.sha256 || '—'}</dd>
            </dl>
            <div class="modal-foot">
              <button class="btn btn-ghost" onClick={() => setDetail(null)}>
                {t('media.close')}
              </button>
              {detail.deleted ? (
                <button class="btn btn-primary" disabled={busy === detail.name} onClick={() => restore(detail)}>
                  {t('media.restore')}
                </button>
              ) : (
                <button class="btn btn-primary" disabled={busy === detail.name} onClick={() => withdraw(detail)}>
                  {t('media.withdraw')}
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
