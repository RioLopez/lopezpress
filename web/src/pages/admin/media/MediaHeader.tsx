import { useI18n } from '../../../core/i18n'

type Props = {
  uploading: boolean
  progress: { done: number; total: number } | null
  isRootUser: boolean
  rebuilding: boolean
  onUploadMany: (files: File[]) => void
  onRebuild: () => void
}

// Title, one-off actions and the multi-file upload control. Kept separate so
// the page component stays inside the 350-line gate and the upload wiring
// lives next to the label it belongs to.
export function MediaHeader(p: Props) {
  const { t } = useI18n()
  return (
    <div class="media-head">
      <div>
        <h1>{t('media.title')}</h1>
        <p class="muted">{t('media.sub')}</p>
      </div>
      <div class="media-head-actions">
        <label class="btn btn-primary btn-sm">
          {p.uploading
            ? p.progress
              ? `${p.progress.done}/${p.progress.total}`
              : t('common.loading')
            : t('media.upload')}
          <input
            type="file"
            accept="image/*"
            multiple
            hidden
            disabled={p.uploading}
            onChange={(e) => {
              const input = e.target as HTMLInputElement
              const files = Array.from(input.files || [])
              if (files.length) p.onUploadMany(files)
              input.value = ''
            }}
          />
        </label>
        {p.isRootUser && (
          <button class="btn btn-ghost btn-sm" disabled={p.rebuilding} onClick={p.onRebuild}>
            {p.rebuilding ? t('common.loading') : t('media.rebuildRefs')}
          </button>
        )}
      </div>
    </div>
  )
}
