import { useCallback, useEffect, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import {
  listCategoriesWithCount,
  createCategory,
  updateCategory,
  deleteCategory,
  ApiError,
} from '../core/api'
import type { CategoryWithCount } from '../core/types'
import { TableState } from '../ui/TableState'

export function AdminCategoriesPage() {
  const { user } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t } = useI18n()
  const [cats, setCats] = useState<CategoryWithCount[]>([])
  const [loading, setLoading] = useState(true)
  const [newName, setNewName] = useState('')
  const [adding, setAdding] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editName, setEditName] = useState('')

  const load = useCallback(() => {
    setLoading(true)
    listCategoriesWithCount()
      .then(setCats)
      .catch(() => pushToast('error', t('admin.failed')))
      .finally(() => setLoading(false))
  }, [pushToast, t])

  useEffect(() => {
    if (user?.role === 'admin' || user?.role === 'root') load()
  }, [user, load])

  if (user?.role !== 'admin' && user?.role !== 'root') {
    return <div class="admin-page"><p class="muted">{t('admin.notAdmin')}</p></div>
  }

  const add = async () => {
    const name = newName.trim()
    if (!name) return
    setAdding(true)
    try {
      await createCategory({ name })
      setNewName('')
      pushToast('success', t('admin.saved'))
      load()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    } finally {
      setAdding(false)
    }
  }

  const startEdit = (c: CategoryWithCount) => {
    setEditingId(c.id)
    setEditName(c.name)
  }

  const saveEdit = async () => {
    if (editingId === null || !editName.trim()) return
    try {
      await updateCategory(editingId, { name: editName.trim() })
      setEditingId(null)
      setEditName('')
      pushToast('success', t('admin.saved'))
      load()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    }
  }

  const del = async (c: CategoryWithCount) => {
    if (!(await confirmDialog(t('admin.deleteConfirmCat', { t: c.name })))) return
    try {
      await deleteCategory(c.id)
      pushToast('success', t('admin.deleted'))
      load()
    } catch (e) {
      pushToast('error', e instanceof ApiError ? e.message : t('admin.failed'))
    }
  }

  return (
    <div class="admin-page">
      <h1>{t('admin.categoriesTitle')}</h1>
      <p class="muted">{t('admin.categoriesSub')}</p>

      <div class="admin-form">
        <input
          value={newName}
          onInput={(e) => setNewName((e.target as HTMLInputElement).value)}
          placeholder={t('admin.categoryPlaceholder')}
          onKeyDown={(e) => { if (e.key === 'Enter') add() }}
        />
        <button class="btn btn-primary" onClick={add} disabled={adding || !newName.trim()}>
          {t('admin.add')}
        </button>
      </div>

      <TableState loading={loading} isEmpty={cats.length === 0} loadingText={t('common.loading')} emptyText={t('admin.emptyCat')}>
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
            {cats.map((c) => (
              <tr key={c.id}>
                {editingId === c.id ? (
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
                  <td>{c.name}</td>
                )}
                <td class="num">{c.count}</td>
                <td class="actions">
                  {editingId !== c.id && (
                    <button class="btn btn-ghost btn-sm" onClick={() => startEdit(c)}>{t('admin.rename')}</button>
                  )}
                  <button class="btn btn-danger btn-sm" onClick={() => del(c)}>{t('list.del')}</button>
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
