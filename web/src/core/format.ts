// Shared date formatters (P2-2 dedup).
//
// Six pages each carried a private fmtDate before this module existed, and
// none of them passed a locale — so an English UI still rendered dates in
// the browser's locale. Pass `locale` ("zh-CN" / "en-US", from useI18n) so
// the output follows the interface language, matching ListPage's localeTag.
//
// Two shapes exist on purpose:
//   - fmtDate:  full timestamp ("2026/10/19 14:30:00"), for admin tables.
//   - fmtDay:   calendar day only ("2026-10-19"), for compact profile rows.
//               ISO short form, locale-independent (dates as identifiers).

/** fmtDate formats an ISO timestamp for display; missing/empty renders "—". */
export function fmtDate(iso?: string, locale?: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return locale ? d.toLocaleString(locale) : d.toLocaleString()
}

/** fmtDay renders only the calendar day of an ISO timestamp ("" when absent). */
export function fmtDay(iso?: string): string {
  return iso ? iso.slice(0, 10) : ''
}
