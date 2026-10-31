import { useI18n } from '../../core/i18n'

// Posts per page for the personal-center list sections. This is a
// personal workspace, not the public list, so it does not follow
// site_config.page_size.
export const PAGE_SIZE = 10

export function Pager({ page, pages, loading, onPrev, onNext }: {
  page: number
  pages: number
  loading: boolean
  onPrev: () => void
  onNext: () => void
}) {
  const { t } = useI18n()
  return (
    <div class="pager">
      <button class="btn btn-ghost btn-sm" disabled={page <= 1 || loading} onClick={onPrev}>
        {t('list.prevPage')}
      </button>
      <span class="pager-info">{t('list.pageOf', { page: String(page), total: String(pages) })}</span>
      <button class="btn btn-ghost btn-sm" disabled={page >= pages || loading} onClick={onNext}>
        {t('list.nextPage')}
      </button>
    </div>
  )
}
