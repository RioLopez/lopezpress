import type { ComponentChildren } from 'preact'

interface TableStateProps {
  loading: boolean
  isEmpty: boolean
  loadingText: string
  emptyText: string
  children: ComponentChildren
}

// Standardizes the loading / empty / data tri-state used by every admin list
// page. Previously each page hand-wrote `{loading ? … : empty ? … : <table>}`,
// which is easy to get half-right (e.g. forgetting the empty branch). The error
// branch is intentionally not here: list pages surface fetch failures via a
// toast, so the error state is "last good data + toast", not a dedicated view.
export function TableState({ loading, isEmpty, loadingText, emptyText, children }: TableStateProps) {
  if (loading) return <p class="muted">{loadingText}</p>
  if (isEmpty) return <p class="muted">{emptyText}</p>
  return <>{children}</>
}
