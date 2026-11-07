import { useState } from 'preact/hooks'
import { useI18n } from '../../../core/i18n'
import { groupLabel, MENU_GROUPS } from '../../../core/menu'
import type { MenuItem } from '../../../core/types'

interface Props {
  row: MenuItem
  index: number
  total: number
  busy: boolean
  onMove: (dir: -1 | 1) => void
  onSave: (patch: { label: string; path: string; group_key: string }) => void
  onToggle: () => void
  onDelete: () => void
}

// One sidebar row. Built-in rows render route and group as read-only: those
// two values are what the rest of the console navigates by, so the server
// ignores edits to them and the UI should not pretend otherwise.
export function MenuRow({ row, index, total, busy, onMove, onSave, onToggle, onDelete }: Props) {
  const { t } = useI18n()
  const [label, setLabel] = useState(row.label)
  const [path, setPath] = useState(row.path)
  const [group, setGroup] = useState(row.group_key)

  const custom = row.key === ''
  const dirty = custom
    ? label !== row.label || path !== row.path || group !== row.group_key
    : label !== row.label

  const fallback = row.label_key ? t(row.label_key) : ''
  const placeholder = fallback || row.path

  return (
    <div class={`menu-row ${row.visible ? '' : 'menu-row--hidden'}`}>
      <div class="menu-row-order">
        <button
          class="btn btn-ghost btn-sm"
          disabled={index === 0 || busy}
          onClick={() => onMove(-1)}
          aria-label="move up"
        >
          ↑
        </button>
        <button
          class="btn btn-ghost btn-sm"
          disabled={index === total - 1 || busy}
          onClick={() => onMove(1)}
          aria-label="move down"
        >
          ↓
        </button>
      </div>

      <div class="menu-row-main">
        <input
          class="menu-row-label"
          value={label}
          placeholder={placeholder}
          maxLength={64}
          onInput={(e) => setLabel((e.target as HTMLInputElement).value)}
        />
        <div class="menu-row-meta">
          {custom ? (
            <input
              value={path}
              placeholder={t('admin.menuAdmin.pathPlaceholder')}
              maxLength={256}
              onInput={(e) => setPath((e.target as HTMLInputElement).value)}
            />
          ) : (
            <span class="menu-row-route">{row.path}</span>
          )}
          <select
            value={group}
            disabled={!custom}
            onChange={(e) => setGroup((e.target as HTMLSelectElement).value)}
          >
            {MENU_GROUPS.map((g) => (
              <option key={g || 'none'} value={g}>
                {g ? groupLabel(g, t) : t('admin.menuAdmin.groupNone')}
              </option>
            ))}
          </select>
          {custom ? (
            <span class="menu-tag">{t('admin.menuAdmin.custom')}</span>
          ) : (
            <span class="muted menu-row-hint">{t('admin.menuAdmin.builtinHint')}</span>
          )}
        </div>
      </div>

      <div class="menu-row-actions">
        <button class="btn btn-ghost btn-sm" disabled={busy} onClick={onToggle}>
          {row.visible ? t('admin.menuAdmin.hide') : t('admin.menuAdmin.show')}
        </button>
        <button
          class="btn btn-primary btn-sm"
          disabled={busy || !dirty}
          onClick={() => onSave({ label, path, group_key: group })}
        >
          {t('admin.menuAdmin.save')}
        </button>
        {custom && (
          <button class="btn btn-danger btn-sm" disabled={busy} onClick={onDelete}>
            ×
          </button>
        )}
      </div>
    </div>
  )
}
