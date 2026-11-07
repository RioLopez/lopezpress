import { useI18n } from '../../../core/i18n'

type Props = {
  count: number
  busy: boolean
  onSoft: () => void
  onHard: () => void
  onClear: () => void
}

// Sticky strip that appears only when something is selected. Two verbs with
// the same meanings as the single-item flow; both confirm before acting.
export function MediaBulkBar(p: Props) {
  const { t } = useI18n()
  if (p.count === 0) return null
  return (
    <div class="media-bulkbar">
      <span>{t('media.selectedCount', { n: String(p.count) })}</span>
      <button class="btn btn-ghost btn-sm" disabled={p.busy} onClick={p.onSoft}>
        {t('media.bulkSoftDelete')}
      </button>
      <button class="btn btn-danger btn-sm" disabled={p.busy} onClick={p.onHard}>
        {t('media.bulkHardDelete')}
      </button>
      <button class="btn btn-ghost btn-sm" disabled={p.busy} onClick={p.onClear}>
        {t('media.clearSelection')}
      </button>
    </div>
  )
}
