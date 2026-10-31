import { useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'

// Classic numbered pager.
//
// Window rule: total <= 7 lists everything; otherwise first and last page are
// always visible, the current page keeps ±2 neighbours, and gaps collapse to
// an ellipsis. This keeps the control a fixed-ish width no matter how many
// pages exist — important because the page count grows with the archive.
interface PaginationProps {
  page: number
  totalPages: number
  onPage: (n: number) => void
}

type Slot = number | 'gap'

function range(from: number, to: number): number[] {
  const out: number[] = []
  for (let i = from; i <= to; i++) out.push(i)
  return out
}

export function pagerSlots(current: number, total: number): Slot[] {
  if (total <= 7) return range(1, Math.max(1, total))
  const start = Math.max(2, Math.min(current - 2, total - 4))
  const end = Math.min(total - 1, Math.max(current + 2, 5))
  const out: Slot[] = [1]
  if (start > 2) out.push('gap')
  for (let i = start; i <= end; i++) out.push(i)
  if (end < total - 1) out.push('gap')
  out.push(total)
  return out
}

export function Pagination({ page, totalPages, onPage }: PaginationProps) {
  const { t } = useI18n()
  const [jump, setJump] = useState('')

  if (totalPages <= 1) return null

  const clamp = (n: number) => Math.min(totalPages, Math.max(1, n))
  const submitJump = () => {
    const n = parseInt(jump, 10)
    if (!Number.isNaN(n)) onPage(clamp(n))
    setJump('')
  }

  return (
    <nav class="pager" aria-label={t('list.pagination')}>
      <button class="pager-btn" disabled={page <= 1} onClick={() => onPage(1)}>
        {t('list.firstPage')}
      </button>
      <button class="pager-btn" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        {t('list.prevPage')}
      </button>

      <span class="pager-pages">
        {pagerSlots(page, totalPages).map((slot, i) =>
          slot === 'gap' ? (
            <span key={`gap-${i}`} class="pager-gap">…</span>
          ) : (
            <button
              key={slot}
              class={`pager-num${slot === page ? ' pager-num--active' : ''}`}
              aria-current={slot === page ? 'page' : undefined}
              onClick={() => onPage(slot)}
            >
              {slot}
            </button>
          ),
        )}
      </span>

      <button class="pager-btn" disabled={page >= totalPages} onClick={() => onPage(page + 1)}>
        {t('list.nextPage')}
      </button>
      <button class="pager-btn" disabled={page >= totalPages} onClick={() => onPage(totalPages)}>
        {t('list.lastPage')}
      </button>

      <span class="pager-jump">
        <span>{t('list.jumpTo')}</span>
        <input
          type="text"
          inputMode="numeric"
          value={jump}
          placeholder={String(page)}
          onInput={(e) => setJump((e.target as HTMLInputElement).value.replace(/[^0-9]/g, ''))}
          onKeyDown={(e) => {
            if (e.key === 'Enter') submitJump()
          }}
        />
        <span>{t('list.jumpPage')}</span>
      </span>
    </nav>
  )
}
