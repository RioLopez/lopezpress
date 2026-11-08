import { useMemo, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listTrash, restoreTrash, purgeTrash, emptyTrash, errMessage } from '../core/api'
import type { TrashPost } from '../core/types'
import { fmtDate } from '../core/format'
import { TableState } from '../ui/TableState'
import { ContentTools, type SortDir } from '../ui/ContentTools'
import { useQuery } from '../core/useQuery'

export function AdminTrashPage() {
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const [busyId, setBusyId] = useState<string | null>(null)
  const [emptying, setEmptying] = useState(false)
  const [q, setQ] = useState('')
  const [sort, setSort] = useState<'deleted' | 'title' | 'views'>('deleted')
  const [dir, setDir] = useState<SortDir>('desc')

  const { data, loading, reload } = useQuery(() => listTrash(), [], {
    onError: () => pushToast('error', t('admin.failed')),
  })
  const items = data ?? []

  // The list endpoint returns the whole (bounded) trash set, so filtering and
  // sorting happen here and every keystroke is instant.
  const visible = useMemo(() => {
    const needle = q.trim().toLowerCase()
    const matched = needle
      ? items.filter((p) => `${p.title} ${p.author?.username ?? ''} ${p.author?.display_name ?? ''}`.toLowerCase().includes(needle))
      : items
    const mul = dir === 'asc' ? 1 : -1
    const epoch = (p: TrashPost) => {
      const n = new Date(p.deleted_at).getTime()
      return Number.isNaN(n) ? 0 : n
    }
    return [...matched].sort((a, b) => {
      if (sort === 'title') return a.title.localeCompare(b.title, locale) * mul
      if (sort === 'views') return (a.view_count - b.view_count) * mul
      return (epoch(a) - epoch(b)) * mul
    })
  }, [items, q, sort, dir, locale])

  const restore = async (p: TrashPost) => {
    if (!(await confirmDialog(t('admin.restoreConfirm', { title: p.title })))) return
    setBusyId(p.id)
    try {
      await restoreTrash(p.id)
      pushToast('success', t('admin.restored'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusyId(null)
    }
  }

  const purge = async (p: TrashPost) => {
    if (!(await confirmDialog(t('admin.trashPurgeConfirm', { title: p.title })))) return
    setBusyId(p.id)
    try {
      await purgeTrash(p.id)
      pushToast('success', t('admin.trashPurged'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusyId(null)
    }
  }

  const empty = async () => {
    if (!(await confirmDialog(t('admin.emptyTrashConfirm')))) return
    setEmptying(true)
    try {
      const r = await emptyTrash()
      pushToast('success', t('admin.emptyTrashDone', { n: String(r.deleted) }))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setEmptying(false)
    }
  }


  return (
    <div class="admin-page">
      <div class="admin-head">
        <h1>{t('admin.trashTitle')}</h1>
        {items.length > 0 && (
          <button class="btn btn-danger btn-sm" disabled={emptying} onClick={empty}>
            {t('admin.emptyTrashBtn')}
          </button>
        )}
      </div>
      <p class="muted">{t('admin.trashSub')}</p>

      <ContentTools
        q={q}
        onQ={setQ}
        placeholder={t('admin.searchContent')}
        sorts={[
          { value: 'deleted', label: t('admin.deletedAt') },
          { value: 'title', label: t('admin.name') },
          { value: 'views', label: t('list.views') },
        ]}
        sort={sort}
        onSort={(v) => setSort(v as typeof sort)}
        dir={dir}
        onDir={setDir}
        descLabel={t('admin.orderDesc')}
        ascLabel={t('admin.orderAsc')}
      />

      <TableState
        loading={loading}
        isEmpty={items.length === 0 || visible.length === 0}
        loadingText={t('common.loading')}
        emptyText={q.trim() && items.length > 0 ? t('admin.noMatchContent') : t('admin.emptyTrash')}
      >
        <div class="table-scroll">
        <table class="admin-table admin-table--cards">
          <thead>
            <tr>
              <th>{t('admin.name')}</th>
              <th>{t('list.author')}</th>
              <th>{t('admin.deletedAt')}</th>
              <th class="actions"></th>
            </tr>
          </thead>
          <tbody>
            {visible.map((p) => (
              <tr key={p.id}>
                <td class="card-title">
                  <div class="admin-user-cell">
                    <span>{p.title}</span>
                    <span class="muted">{p.view_count} {t('list.views')}</span>
                  </div>
                </td>
                <td data-label={t('list.author')}>{p.author?.username || p.author_id}</td>
                <td class="muted" data-label={t('admin.deletedAt')}>{fmtDate(p.deleted_at, locale)}</td>
                <td class="actions">
                  <button
                    class="btn btn-primary btn-sm"
                    disabled={busyId === p.id}
                    onClick={() => restore(p)}
                  >
                    {t('admin.restore')}
                  </button>
                  <button
                    class="btn btn-danger btn-sm"
                    disabled={busyId === p.id}
                    onClick={() => purge(p)}
                  >
                    {t('admin.trashPurge')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      </TableState>

      <p class="muted admin-trash-hint">{t('admin.expiredHint')}</p>
    </div>
  )
}
