import { useCallback, useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { listTagsWithCount, deleteTag, ApiError } from '../core/api'
import type { TagWithCount } from '../core/types'
import { TableState } from '../ui/TableState'

export function AdminTagsPage() {
  const { user } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t } = useI18n()
  const [tags, setTags] = useState<TagWithCount[]>([])
  const [loading, setLoading] = useState(true)

  const load = useCallback(() => {
    setLoading(true)
    listTagsWithCount()
      .then(setTags)
      .catch(() => pushToast('error', t('admin.failed')))
      .finally(() => setLoading(false))
  }, [pushToast, t])

  useEffect(() => {
    if (user?.role === 'admin' || user?.role === 'root') load()
  }, [user, load])

  if (user?.role !== 'admin' && user?.role !== 'root') {
    return <div class="admin-page"><p class="muted">{t('admin.notAdmin')}</p></div>
  }

  const del = async (tag: TagWithCount) => {
    if (!(await confirmDialog(t('admin.deleteConfirmTag', { t: tag.name })))) return
    try {
      await deleteTag(tag.id)
      pushToast('success', t('admin.deleted'))
      load()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    }
  }

  return (
    <div class="admin-page">
      <h1>{t('admin.tagsTitle')}</h1>
      <p class="muted">{t('admin.tagsSub')}</p>

      <TableState loading={loading} isEmpty={tags.length === 0} loadingText={t('common.loading')} emptyText={t('admin.emptyTag')}>
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
            {tags.map((tag) => (
              <tr key={tag.id}>
                <td><span class="post-tag-chip">{tag.name}</span></td>
                <td class="num">{tag.count}</td>
                <td class="actions">
                  <button class="btn btn-danger btn-sm" onClick={() => del(tag)}>{t('list.del')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      </TableState>
    </div>
  )
}
