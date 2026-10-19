import { useApp, type Toast } from '../core/app'

export function Toasts() {
  const { toasts, dismissToast } = useApp()
  if (toasts.length === 0) return null
  return (
    <div class="toast-stack">
      {toasts.map((t: Toast) => (
        <div key={t.id} class={`toast toast-${t.type}`} onClick={() => dismissToast(t.id)}>
          <span>{t.message}</span>
          <button class="toast-close" onClick={(e) => { e.stopPropagation(); dismissToast(t.id) }}>×</button>
        </div>
      ))}
    </div>
  )
}
