import { useI18n } from '../../../core/i18n'

const PAGE_SIZES = [5, 10, 20]

export function UserFilters({
  query,
  roleFilter,
  statusFilter,
  size,
  onQuery,
  onRole,
  onStatus,
  onSize,
}: {
  query: string
  roleFilter: string
  statusFilter: string
  size: number
  onQuery: (v: string) => void
  onRole: (v: string) => void
  onStatus: (v: string) => void
  onSize: (v: number) => void
}) {
  const { t } = useI18n()

  return (
    <div class="admin-form">
      <input
        value={query}
        placeholder={t('admin.searchUsers')}
        onInput={(e) => onQuery((e.target as HTMLInputElement).value)}
      />
      <select value={roleFilter} onInput={(e) => onRole((e.target as HTMLSelectElement).value)}>
        <option value="">{t('admin.allRoles')}</option>
        <option value="root">root</option>
        <option value="admin">{t('admin.roleAdmin')}</option>
        <option value="user">{t('admin.roleUser')}</option>
      </select>
      <select value={statusFilter} onInput={(e) => onStatus((e.target as HTMLSelectElement).value)}>
        <option value="">{t('admin.allStatuses')}</option>
        <option value="active">{t('admin.active')}</option>
        <option value="disabled">{t('admin.disabled')}</option>
      </select>
      <select value={String(size)} onInput={(e) => onSize(Number((e.target as HTMLSelectElement).value))}>
        {PAGE_SIZES.map((n) => (
          <option key={n} value={String(n)}>{t('admin.pageSizeLabel')} {n}</option>
        ))}
      </select>
    </div>
  )
}
