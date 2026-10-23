import { useMemo, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { useNav } from '../core/router'
import { listCategoriesWithCount, createCategory, updateCategory, deleteCategory, applyPresetCategories, errMessage } from '../core/api'
import type { CategoryWithCount } from '../core/types'
import { TableState } from '../ui/TableState'
import { useQuery } from '../core/useQuery'

// Spec 27: categories are a TWO-level tree (parent -> child, never deeper).
// The table renders parents first, then their children indented underneath,
// so the structure is legible without a drag-and-drop tree widget.

interface Row {
  cat: CategoryWithCount
  depth: 0 | 1
}

function buildRows(cats: CategoryWithCount[], filter: string): Row[] {
  const q = filter.trim().toLowerCase()
  const byId = new Map(cats.map((c) => [c.id, c]))
  const roots = cats.filter((c) => c.parent_id == null)
  const childrenOf = (id: number) => cats.filter((c) => c.parent_id === id)

  const rows: Row[] = []
  const push = (cat: CategoryWithCount, depth: 0 | 1, include: boolean) => {
    if (include) rows.push({ cat, depth })
    for (const child of childrenOf(cat.id)) {
      if (q && !child.name.toLowerCase().includes(q)) continue
      rows.push({ cat: child, depth: 1 })
    }
  }

  for (const root of roots) {
    const rootMatches = !q || root.name.toLowerCase().includes(q)
    const hasMatchingChild = childrenOf(root.id).some((c) => c.name.toLowerCase().includes(q))
    // A parent stays visible when it matches itself or when any child matches,
    // so a filtered child is never shown detached from its parent.
    push(root, 0, rootMatches || hasMatchingChild)
  }
  // Orphans (parent_id pointing at a deleted row) would otherwise vanish.
  for (const cat of cats) {
    if (cat.parent_id != null && !byId.has(cat.parent_id)) {
      if (!q || cat.name.toLowerCase().includes(q)) rows.push({ cat, depth: 0 })
    }
  }
  return rows
}

export function AdminCategoriesPage() {
  const { pushToast, confirmDialog } = useApp()
  const { t } = useI18n()
  const [, navigate] = useNav()
  const [filter, setFilter] = useState('')
  const [newName, setNewName] = useState('')
  const [newParent, setNewParent] = useState<number | ''>('')
  const [adding, setAdding] = useState(false)
  const [presetBusy, setPresetBusy] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editParent, setEditParent] = useState<number | ''>('')

  const { data, loading, reload } = useQuery(() => listCategoriesWithCount(), [], {
    onError: () => pushToast('error', t('admin.failed')),
  })
  const cats = data ?? []

  const rows = useMemo(() => buildRows(cats, filter), [cats, filter])
  // Only top-level categories can be chosen as a parent (two-level rule).
  const parentOptions = useMemo(() => cats.filter((c) => c.parent_id == null), [cats])

  const add = async () => {
    const name = newName.trim()
    if (!name) return
    setAdding(true)
    try {
      await createCategory({
        name,
        parent_id: newParent === '' ? null : Number(newParent),
      })
      setNewName('')
      setNewParent('')
      pushToast('success', t('admin.saved'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setAdding(false)
    }
  }

  const startEdit = (c: CategoryWithCount) => {
    setEditingId(c.id)
    setEditName(c.name)
    setEditParent(c.parent_id ?? '')
  }

  const saveEdit = async () => {
    if (editingId === null || !editName.trim()) return
    try {
      await updateCategory(editingId, {
        name: editName.trim(),
        // 0 is the server's explicit "move to top level" sentinel.
        parent_id: editParent === '' ? 0 : Number(editParent),
      })
      setEditingId(null)
      setEditName('')
      setEditParent('')
      pushToast('success', t('admin.saved'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    }
  }

  const del = async (c: CategoryWithCount) => {
    if (!(await confirmDialog(t('admin.deleteConfirmCat', { t: c.name })))) return
    try {
      await deleteCategory(c.id)
      pushToast('success', t('admin.deleted'))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    }
  }

  const applyPreset = async () => {
    setPresetBusy(true)
    try {
      const res = await applyPresetCategories()
      pushToast('success', t('admin.presetApplied', { created: String(res.created), reparented: String(res.reparented) }))
      await reload()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setPresetBusy(false)
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
        <select
          value={newParent === '' ? '' : String(newParent)}
          onInput={(e) => setNewParent((e.target as HTMLSelectElement).value ? Number((e.target as HTMLSelectElement).value) : '')}
        >
          <option value="">{t('admin.topLevel')}</option>
          {parentOptions.map((c) => (
            <option key={c.id} value={String(c.id)}>{c.name}</option>
          ))}
        </select>
        <button class="btn btn-primary" onClick={add} disabled={adding || !newName.trim()}>
          {t('admin.add')}
        </button>
      </div>

      <div class="admin-form">
        <input
          value={filter}
          onInput={(e) => setFilter((e.target as HTMLInputElement).value)}
          placeholder={t('admin.filterPlaceholder')}
        />
        <button class="btn btn-ghost btn-sm" onClick={applyPreset} disabled={presetBusy}>
          {t('admin.applyPreset')}
        </button>
      </div>
      <p class="settings-hint">{t('admin.presetHint')}</p>

      <TableState loading={loading} isEmpty={cats.length === 0} loadingText={t('common.loading')} emptyText={t('admin.emptyCat')}>
        {rows.length === 0 ? (
          <p class="muted">{t('admin.noMatch')}</p>
        ) : (
          <div class="table-scroll">
            <table class="admin-table">
              <thead>
                <tr>
                  <th>{t('admin.name')}</th>
                  <th>{t('admin.parentCategory')}</th>
                  <th class="num">{t('admin.postCount')}</th>
                  <th class="actions"></th>
                </tr>
              </thead>
              <tbody>
                {rows.map(({ cat: c, depth }) => (
                  <tr key={c.id} class={depth === 1 ? 'row-child' : ''}>
                    {editingId === c.id ? (
                      <>
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
                          </div>
                        </td>
                        <td>
                          <div class="admin-form admin-form--inline">
                            <select
                              value={editParent === '' ? '' : String(editParent)}
                              onInput={(e) => setEditParent((e.target as HTMLSelectElement).value ? Number((e.target as HTMLSelectElement).value) : '')}
                            >
                              <option value="">{t('admin.topLevel')}</option>
                              {parentOptions.filter((p) => p.id !== c.id).map((p) => (
                                <option key={p.id} value={String(p.id)}>{p.name}</option>
                              ))}
                            </select>
                            <button class="btn btn-primary btn-sm" onClick={saveEdit}>{t('admin.rename')}</button>
                            <button class="btn btn-ghost btn-sm" onClick={() => setEditingId(null)}>{t('editor.cancel')}</button>
                          </div>
                        </td>
                        <td class="num">{c.count}</td>
                        <td class="actions"></td>
                      </>
                    ) : (
                      <>
                        <td>
                          {depth === 1 && <span class="tree-indent">└</span>}
                          <span class={depth === 0 ? 'tree-root' : ''}>{c.name}</span>
                        </td>
                        <td class="muted">
                          {c.parent_id != null
                            ? cats.find((p) => p.id === c.parent_id)?.name || '—'
                            : '—'}
                        </td>
                        <td class="num">
                          {c.count > 0 ? (
                            <a
                              href="#"
                              class="count-link"
                              title={t('admin.viewPosts')}
                              onClick={(e) => {
                                e.preventDefault()
                                navigate(`/?category=${c.id}`)
                              }}
                            >
                              {c.count}
                            </a>
                          ) : (
                            c.count
                          )}
                        </td>
                        <td class="actions">
                          <button class="btn btn-ghost btn-sm" onClick={() => startEdit(c)}>{t('admin.rename')}</button>
                          <button class="btn btn-danger btn-sm" onClick={() => del(c)}>{t('list.del')}</button>
                        </td>
                      </>
                    )}
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
