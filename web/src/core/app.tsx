// App-level UI state: toast notifications + shared helpers (confirm dialog).
// Routing now lives in wouter (see core/router.tsx); this context only holds
// the transient UI state that has no URL representation.

import { createContext } from 'preact'
import { useContext, useRef, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'

export interface Toast {
  id: number
  type: 'success' | 'error' | 'info'
  message: string
}

interface AppContextValue {
  toasts: Toast[]
  pushToast: (type: Toast['type'], message: string) => void
  dismissToast: (id: number) => void
  confirmDialog: (message: string) => Promise<boolean>
}

const AppContext = createContext<AppContextValue | null>(null)

export function AppProvider({ children }: { children: ComponentChildren }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const idRef = useRef(0)

  const pushToast = (type: Toast['type'], message: string) => {
    const id = ++idRef.current
    setToasts((t) => [...t, { id, type, message }])
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4000)
  }

  const dismissToast = (id: number) => setToasts((t) => t.filter((x) => x.id !== id))

  const confirmDialog = (message: string) =>
    new Promise<boolean>((resolve) => resolve(window.confirm(message)))

  const value: AppContextValue = { toasts, pushToast, dismissToast, confirmDialog }
  return <AppContext.Provider value={value}>{children}</AppContext.Provider>
}

export function useApp() {
  const v = useContext(AppContext)
  if (!v) throw new Error('useApp must be used within <AppProvider>')
  return v
}
