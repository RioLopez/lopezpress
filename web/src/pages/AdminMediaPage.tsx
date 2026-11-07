import { useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useApp } from '../core/app'
import { useAuth } from '../core/auth'
import { useQuery } from '../core/useQuery'
import { fmtBytes } from '../core/format'
import {
  listUploads,
  deleteUpload,
  purgeUpload,
  restoreUpload,
  retryUploadThumb,
  rebuildUploadRefs,
  batchDeleteUploads,
  uploadImage,
  errMessage,
  errStatus,
} from '../core/api'
import { isRoot } from '../core/roles'
import type { UploadFileInfo } from '../core/types'
import { MediaHeader } from './admin/media/MediaHeader'
import { MediaToolbar, type MediaView, type StatusFilter, type RefFilter } from './admin/media/MediaToolbar'
import { MediaBulkBar } from './admin/media/MediaBulkBar'
import { MediaGrid } from './admin/media/MediaGrid'
import { MediaList } from './admin/media/MediaList'
import { MediaDetail } from './admin/media/MediaDetail'

const PAGE_SIZE = 24

// Admin media library: the site owner's asset view, with per-file owner,
// geometry and reference count.
//
// Two delete verbs, deliberately distinct:
//   - Withdraw (soft): invisible and un-reusable, file stays; articles using
//     it keep rendering. Reversible.
//   - Delete permanently (hard): removes the bytes even when articles embed
//     the image — moderating a bad picture must not need their consent — but
//     the count is shown first.
export function AdminMediaPage() {
  const { t } = useI18n()
  const { pushToast } = useApp()
  const { user } = useAuth()
  const isRootUser = isRoot(user?.role)

  const [status, setStatus] = useState<StatusFilter>('live')
  const [kind, setKind] = useState('')
  const [refStatus, setRefStatus] = useState<RefFilter>('')
  const [view, setView] = useState<MediaView>('grid')
  const [page, setPage] = useState(1)
  const [detail, setDetail] = useState<UploadFileInfo | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [bulkBusy, setBulkBusy] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [uploading, setUploading] = useState(false)
  const [rebuilding, setRebuilding] = useState(false)
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null)

  const { data, loading, reload } = useQuery(
    () =>
      listUploads({
        status,
        kind: kind || undefined,
        ref_status: refStatus || undefined,
        page,
        size: PAGE_SIZE,
      }),
    [status, kind, refStatus, page],
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

  // The worker retries once by itself; this is the escape hatch for a row that
  // already reached "failed" with a file the operator knows was replaced.
  const retryThumb = async (item: UploadFileInfo) => {
    setBusy(item.name)
    try {
      await retryUploadThumb(item.name)
      await after(t('media.thumbQueued'))
    } catch (e) {
      pushToast('error', errMessage(e, t('media.thumbRetryFailed')))
    } finally {
      setBusy(null)
    }
  }

  // M-9: rebuild every reference edge from posts / avatars / branding. Root
  // only, and deliberately behind a confirm — it rewrites the whole ledger.
  const rebuild = async () => {
    if (!confirm(t('media.rebuildConfirm'))) return
    setRebuilding(true)
    try {
      const r = await rebuildUploadRefs()
      pushToast('info', t('media.rebuildDone', { n: String(r.refs) }))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('media.rebuildFailed')))
    } finally {
      setRebuilding(false)
    }
  }

  const items = data?.items ?? []
  const toggle = (name: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  const allSelected = items.length > 0 && items.every((i) => selected.has(i.name))
  const toggleAll = () =>
    setSelected((prev) => {
      const next = new Set(prev)
      const every = items.every((i) => next.has(i.name))
      items.forEach((i) => (every ? next.delete(i.name) : next.add(i.name)))
      return next
    })

  const bulkDelete = async (hard: boolean) => {
    const names = [...selected]
    if (names.length === 0) return
    const msg = hard
      ? t('media.bulkHardConfirm', { n: String(names.length) })
      : t('media.bulkSoftConfirm', { n: String(names.length) })
    if (!confirm(msg)) return
    setBulkBusy(true)
    try {
      const r = await batchDeleteUploads(names, hard)
      pushToast('info', t('media.bulkDone', { n: String(r.affected) }))
      if (r.skipped.length > 0) {
        pushToast('error', t('media.bulkSkipped', { n: String(r.skipped.length) }))
      }
      setSelected(new Set())
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('media.deleteFailed')))
    } finally {
      setBulkBusy(false)
    }
  }

  const stats = data?.stats
  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.size)) : 1

  return (
    <div class="admin-page">
      <MediaHeader
        uploading={uploading}
        progress={progress}
        isRootUser={isRootUser}
        rebuilding={rebuilding}
        onUploadMany={onUploadMany}
        onRebuild={rebuild}
      />

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

      <MediaToolbar
        status={status}
        kind={kind}
        refStatus={refStatus}
        view={view}
        pageCount={items.length}
        selectedCount={selected.size}
        allSelected={allSelected}
        onStatus={(v) => { setStatus(v); setPage(1) }}
        onKind={(v) => { setKind(v); setPage(1) }}
        onRefStatus={(v) => { setRefStatus(v); setPage(1) }}
        onView={setView}
        onToggleAll={toggleAll}
      />

      <MediaBulkBar
        count={selected.size}
        busy={bulkBusy}
        onSoft={() => bulkDelete(false)}
        onHard={() => bulkDelete(true)}
        onClear={() => setSelected(new Set())}
      />

      {loading && <p class="muted">{t('media.loading')}</p>}
      {!loading && data && items.length === 0 && <p class="muted">{t('media.empty')}</p>}

      {items.length > 0 && view === 'grid' && (
        <MediaGrid
          items={items}
          busy={busy}
          selected={selected}
          onToggle={toggle}
          onOpen={setDetail}
          onWithdraw={withdraw}
          onPurge={purge}
          onRestore={restore}
          onRetryThumb={retryThumb}
        />
      )}
      {items.length > 0 && view === 'list' && (
        <MediaList
          items={items}
          busy={busy}
          selected={selected}
          onToggle={toggle}
          onOpen={setDetail}
        />
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
        <MediaDetail
          item={detail}
          busy={busy}
          onClose={() => setDetail(null)}
          onWithdraw={withdraw}
          onRestore={restore}
          onRetryThumb={retryThumb}
        />
      )}
    </div>
  )
}
