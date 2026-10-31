import { useMemo, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { useNav } from '../core/router'
import { listTagsWithCount, deleteTag, updateTag, errMessage } from '../core/api'
import type { TagWithCount } from '../core/types'
import { TableState } from '../ui/TableState'
import { useQuery } from '../core/useQuery'

// Rename in place, filter locally, and make the count a link into the
// public list's ?tag= filter. The filter is client-side on purpose — a blog has
// tens of tags, not thousands, so a backend search endpoint would be dead
// weight on a 1 GB VPS.
export function AdminTagsPage() {
  const { pushToast, confirmDialog } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [filter, setFilter] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editName, setEditName] = useState('')

  const { data, loading, reload } = useQuery(() => listTagsWithCount(), [], {
    onError: () => pushToast('error', t('admin.failed')),
  })
  const tags = data ?? []

  const visible = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return tags
    return tags.filter((tg) => tg.name.toLowerCase().includes(q))
  }, [tags, filter])

  const startEdit = (tg: TagWithCount) => {
    setEditingId(tg.id)
    setEditName(tg.name)
  }

  const saveEdit = async () => {
    if (editingId === null || !editName.trim()) return
    try {
      await updateTag(editingId, editName.trim())
      setEditingId(null)
      setEditName('')
      pushToast('success', t('admin.saved'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    }
  }

  const del = async (tag: TagWithCount) => {
    if (!(await confirmDialog(t('admin.deleteConfirmTag', { t: tag.name })))) return
    try {
      await deleteTag(tag.id)
      pushToast('success', t('admin.deleted'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    }
  }

  return (
    <div class="admin-page">
      <h1>{t('admin.tagsTitle')}</h1>
      <p class="muted">{t('admin.tagsSub')}</p>

      <div class="admin-form">
        <input
          value={filter}
          onInput={(e) => setFilter((e.target as HTMLInputElement).value)}
          placeholder={t('admin.filterPlaceholder')}
        />
      </div>

      <TableState
        loading={loading}
        isEmpty={tags.length === 0}
        loadingText={t('common.loading')}
        emptyText={t('admin.emptyTag')}
      >
        {visible.length === 0 ? (
          <p class="muted">{t('admin.noMatch')}</p>
        ) : (
          <div class="table-scroll">
            <table class="admin-table">
              <thead>
                <tr>
                  <th>{t('admin.name')}</th>
                  <th class="num">{t('admin.postCount')}</th>
                  <th class="actions"></th>
                </tr>
              </thead>
              <tbody>
                {visible.map((tag) => (
                  <tr key={tag.id}>
                    {editingId === tag.id ? (
                      <td>
                        <div class="admin-form admin-form--inline">
                          <input
                            value={editName}
                            onInput={(e) => setEditName((e.target as HTMLInputElement).value)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') saveEdit()
                              if (e.key === 'Escape') setEditingId(null)
                            }}
                          />
                          <button class="btn btn-primary btn-sm" onClick={saveEdit}>{t('admin.rename')}</button>
                          <button class="btn btn-ghost btn-sm" onClick={() => setEditingId(null)}>{t('editor.cancel')}</button>
                        </div>
                      </td>
                    ) : (
                      <td><span class="post-tag-chip">{tag.name}</span></td>
                    )}
                    <td class="num">
                      {tag.count > 0 ? (
                        <a
                          href="#"
                          class="count-link"
                          title={t('admin.viewPosts')}
                          onClick={(e) => {
                            e.preventDefault()
                            navigate(`/?tag=${encodeURIComponent(tag.name)}`)
                          }}
                        >
                          {tag.count}
                        </a>
                      ) : (
                        tag.count
                      )}
                    </td>
                    <td class="actions">
                      {editingId !== tag.id && (
                        <button class="btn btn-ghost btn-sm" onClick={() => startEdit(tag)}>{t('admin.rename')}</button>
                      )}
                      <button class="btn btn-danger btn-sm" onClick={() => del(tag)}>{t('list.del')}</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </TableState>
    </div>
  )
}
