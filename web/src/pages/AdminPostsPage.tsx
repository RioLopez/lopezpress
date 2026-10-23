import { useEffect, useMemo, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { useNav } from '../core/router'
import { useDebounce } from '../core/debounce'
import { listPostsPaged, listCategories, deletePost, batchPosts, banPost, importMarkdown, errMessage } from '../core/api'
import type { PostStatus, Category } from '../core/types'
import { Pagination } from '../ui/Pagination'
import { TableState } from '../ui/TableState'
import { useQuery } from '../core/useQuery'

// Spec 26: the admin post list that the site never had. Before this page,
// posts were managed from the public home page — reading and administering in
// the same surface. Bulk actions delegate to the server's single-item rules,
// so a partial permission failure comes back as `skipped`, not as a 500.

// The admin table keeps its own page size: it is a work surface, not the
// public reading list, so it must not follow site_config.page_size.
const ADMIN_PAGE_SIZE = 20

const STATUSES: PostStatus[] = ['published', 'private', 'pending', 'rejected', 'banned']

export function AdminPostsPage() {
  const { user } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const [, navigate] = useNav()

  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<PostStatus | ''>('')
  const [category, setCategory] = useState<number | null>(null)
  const [search, setSearch] = useState('')
  const [categories, setCategories] = useState<Category[]>([])

  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [tagInput, setTagInput] = useState('')
  const [batchCategory, setBatchCategory] = useState<number | ''>('')
  const [busy, setBusy] = useState(false)

  const debouncedSearch = useDebounce(search, 300)
  const isRoot = user?.role === 'root'
  const isAdmin = user?.role === 'admin' || user?.role === 'root'

  const { data, loading, reload } = useQuery(
    () =>
      listPostsPaged({
        status: status || undefined,
        q: debouncedSearch || undefined,
        page,
        size: ADMIN_PAGE_SIZE,
        category: category == null ? undefined : category,
      }),
    [status, debouncedSearch, page, category],
    { enabled: isAdmin, onError: (e) => pushToast('error', errMessage(e, t('admin.failed'))) },
  )

  useEffect(() => {
    listCategories().then(setCategories).catch(() => {})
  }, [])

  const items = data?.items ?? []
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / (data?.page_size || ADMIN_PAGE_SIZE)))
  const allSelected = items.length > 0 && items.every((p) => selected.has(p.id))

  // Selection is per-response: a filter or page change invalidates it, which
  // prevents "silently acting on rows the admin can no longer see".
  useEffect(() => {
    setSelected(new Set())
  }, [page, status, debouncedSearch, category])

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleAll = () => {
    setSelected(allSelected ? new Set() : new Set(items.map((p) => p.id)))
  }

  const runBatch = async (
    action: 'publish' | 'private' | 'delete' | 'set_category' | 'add_tags' | 'remove_tags',
    label: string,
    extra: { category_id?: number; tags?: string[] } = {},
  ) => {
    const ids = Array.from(selected)
    if (ids.length === 0) return
    if (!(await confirmDialog(t('admin.batchConfirm', { n: String(ids.length), action: label })))) return
    setBusy(true)
    try {
      const res = await batchPosts(ids, action, extra)
      pushToast('success', t('admin.batchDonePosts', { n: String(res.affected) }))
      if (res.skipped.length > 0) {
        pushToast('error', t('admin.batchSkipped', { n: String(res.skipped.length) }))
      }
      setSelected(new Set())
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusy(false)
    }
  }

  // Spec 30: banning used to be backend-only (POST /api/admin/ban/{id} had no
  // UI), so the banned vault could only ever shrink. Root-only, and the button
  // is hidden for posts already in the vault.
  const banOne = async (id: string) => {
    if (!(await confirmDialog(t('admin.banPostConfirm')))) return
    try {
      await banPost(id)
      pushToast('success', t('admin.banned'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    }
  }

  // Spec 32: bulk markdown import (admin/root). Files are posted as multipart;
  // the server reports per-file skipped reasons.
  const onImport = async (files: FileList | null) => {
    if (!files || files.length === 0) return
    const fd = new FormData()
    Array.from(files).forEach((f) => fd.append('files', f))
    setBusy(true)
    try {
      const res = await importMarkdown(fd)
      pushToast('success', t('admin.importDone', { created: String(res.created), skipped: String(res.skipped.length) }))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusy(false)
    }
  }

  const removeOne = async (id: string) => {
    if (!(await confirmDialog(t('list.deleteConfirm')))) return
    try {
      await deletePost(id)
      pushToast('success', t('post.deleted'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    }
  }

  const tagList = useMemo(
    () => tagInput.split(/[,，\s]+/).map((s) => s.trim()).filter(Boolean),
    [tagInput],
  )

  if (user?.role !== 'admin' && user?.role !== 'root') {
    return <div class="admin-page"><p class="muted">{t('admin.notAdmin')}</p></div>
  }

  const localeTag = locale === 'zh' ? 'zh-CN' : 'en-US'
  const selectedCount = selected.size

  return (
    <div class="admin-page">
      <h1>{t('admin.postsTitle')}</h1>
      <p class="muted">{t('admin.postsSub')}</p>

      <div class="admin-form">
        <input
          value={search}
          placeholder={t('list.searchPlaceholder')}
          onInput={(e) => {
            setSearch((e.target as HTMLInputElement).value)
            setPage(1)
          }}
        />
        <select
          value={status}
          onInput={(e) => {
            setStatus((e.target as HTMLSelectElement).value as PostStatus | '')
            setPage(1)
          }}
        >
          <option value="">{t('list.allStatus')}</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>{t(`admin.statusLabel.${s}`)}</option>
          ))}
        </select>
        <select
          value={category == null ? '' : String(category)}
          onInput={(e) => {
            const v = (e.target as HTMLSelectElement).value
            setCategory(v ? Number(v) : null)
            setPage(1)
          }}
        >
          <option value="">{t('list.allCategories')}</option>
          {categories.map((c) => (
            <option key={c.id} value={String(c.id)}>{c.name}</option>
          ))}
        </select>
      </div>

      <div class="admin-form">
        <label class="btn btn-ghost btn-sm">
          {t('admin.importMd')}
          <input
            type="file"
            accept=".md,.markdown"
            multiple
            style="display:none"
            onChange={(e) => onImport((e.target as HTMLInputElement).files)}
          />
        </label>
      </div>

      {selectedCount > 0 && (
        <div class="batch-bar">
          <strong class="batch-count">{t('admin.selected', { n: String(selectedCount) })}</strong>
          {isRoot && (
            <button class="btn btn-sm" disabled={busy} onClick={() => runBatch('publish', t('admin.batchPublish'))}>
              {t('admin.batchPublish')}
            </button>
          )}
          <button class="btn btn-sm" disabled={busy} onClick={() => runBatch('private', t('admin.batchPrivate'))}>
            {t('admin.batchPrivate')}
          </button>
          <select
            class="batch-select"
            value={batchCategory === '' ? '' : String(batchCategory)}
            onInput={(e) => setBatchCategory((e.target as HTMLSelectElement).value ? Number((e.target as HTMLSelectElement).value) : '')}
          >
            <option value="">{t('list.categoryLabel')}</option>
            {categories.map((c) => (
              <option key={c.id} value={String(c.id)}>{c.name}</option>
            ))}
          </select>
          <button
            class="btn btn-sm"
            disabled={busy || batchCategory === ''}
            onClick={() => runBatch('set_category', t('list.categoryLabel'), { category_id: Number(batchCategory) })}
          >
            {t('admin.batchApply')}
          </button>
          <input
            class="batch-tags"
            value={tagInput}
            placeholder={t('admin.tagsPlaceholder')}
            onInput={(e) => setTagInput((e.target as HTMLInputElement).value)}
          />
          <button
            class="btn btn-sm"
            disabled={busy || tagList.length === 0}
            onClick={() => runBatch('add_tags', t('admin.batchApply'), { tags: tagList })}
          >
            + {t('list.tagLabel')}
          </button>
          <button
            class="btn btn-sm"
            disabled={busy || tagList.length === 0}
            onClick={() => runBatch('remove_tags', t('admin.batchApply'), { tags: tagList })}
          >
            − {t('list.tagLabel')}
          </button>
          <button class="btn btn-danger btn-sm" disabled={busy} onClick={() => runBatch('delete', t('admin.batchDelete'))}>
            {t('admin.batchDelete')}
          </button>
          <button class="btn btn-ghost btn-sm" disabled={busy} onClick={() => setSelected(new Set())}>
            {t('admin.clearSelection')}
          </button>
        </div>
      )}

      <TableState
        loading={loading}
        isEmpty={items.length === 0}
        loadingText={t('common.loading')}
        emptyText={t('list.empty')}
      >
        <div class="table-scroll">
          <table class="admin-table">
            <thead>
              <tr>
                <th class="col-check">
                  <input type="checkbox" checked={allSelected} onChange={toggleAll} title={t('admin.selectAll')} />
                </th>
                <th>{t('admin.name')}</th>
                <th>{t('admin.status')}</th>
                <th>{t('list.categoryLabel')}</th>
                <th class="num">{t('list.views')}</th>
                <th>{t('admin.time')}</th>
                <th class="actions"></th>
              </tr>
            </thead>
            <tbody>
              {items.map((p) => (
                <tr key={p.id} class={selected.has(p.id) ? 'row-selected' : ''}>
                  <td class="col-check">
                    <input type="checkbox" checked={selected.has(p.id)} onChange={() => toggle(p.id)} />
                  </td>
                  <td>
                    <span
                      class="admin-post-title"
                      onClick={() => navigate(`/read/${p.id}`)}
                      title={p.title}
                    >
                      {p.pinned && <span class="badge-pinned">{t('list.pinned')}</span>}
                      {p.title}
                    </span>
                  </td>
                  <td>
                    <span class={`post-status post-status-${p.status}`}>
                      {t(`admin.statusLabel.${p.status}`)}
                    </span>
                  </td>
                  <td>{p.category?.name || '—'}</td>
                  <td class="num">{p.view_count}</td>
                  <td class="muted">{new Date(p.created_at).toLocaleDateString(localeTag)}</td>
                  <td class="actions">
                    <button class="btn btn-ghost btn-sm" onClick={() => navigate(`/edit/${p.id}`)}>
                      {t('list.edit')}
                    </button>
                    {isRoot && p.status !== 'banned' && (
                      <button class="btn btn-danger btn-sm" onClick={() => banOne(p.id)}>
                        {t('admin.banPost')}
                      </button>
                    )}
                    <button class="btn btn-danger btn-sm" onClick={() => removeOne(p.id)}>
                      {t('list.del')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </TableState>

      <Pagination page={page} totalPages={totalPages} onPage={setPage} />
    </div>
  )
}
