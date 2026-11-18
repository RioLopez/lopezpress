import { useI18n } from '../../../core/i18n'

export type MediaScope = 'image' | 'pdf'

// Two top-level tabs, mirroring the settings page's "基本 / 内容与显示 / 外观"
// row: the library holds two different things (pictures and documents) and a
// single mixed grid makes both harder to scan. The backend understands
// kind=image (everything that is not a PDF) and kind=pdf.
export function MediaTabs({ scope, onScope }: { scope: MediaScope; onScope: (s: MediaScope) => void }) {
  const { t } = useI18n()
  const tabs: { key: MediaScope; label: string }[] = [
    { key: 'image', label: t('media.tabImages') },
    { key: 'pdf', label: t('media.tabPdf') },
  ]
  return (
    <div class="media-tabs" role="tablist">
      {tabs.map((tab) => (
        <button
          key={tab.key}
          role="tab"
          aria-selected={scope === tab.key}
          class={`settings-nav-item ${scope === tab.key ? 'is-active' : ''}`}
          onClick={() => onScope(tab.key)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  )
}
