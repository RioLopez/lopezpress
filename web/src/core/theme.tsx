import { createContext } from 'preact'
import { useContext, useEffect, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'

export type Theme = 'light' | 'dark'
export type ThemeMode = 'light' | 'dark' | 'system'

const THEME_KEY = 'echo.theme'

function systemTheme(): Theme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function detectMode(): ThemeMode {
  const saved = localStorage.getItem(THEME_KEY) as ThemeMode | null
  if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
  return 'system'
}

function resolve(mode: ThemeMode): Theme {
  return mode === 'system' ? systemTheme() : mode
}

function apply(theme: Theme) {
  document.documentElement.setAttribute('data-theme', theme)
}

interface ThemeValue {
  mode: ThemeMode
  setMode: (m: ThemeMode) => void
  cycle: () => void
  current: Theme
}

const ThemeContext = createContext<ThemeValue | null>(null)

export function ThemeProvider({ children }: { children: ComponentChildren }) {
  const [mode, setModeState] = useState<ThemeMode>(detectMode)
  const [current, setCurrent] = useState<Theme>(() => resolve(mode))

  // Apply on mount and whenever the resolved theme changes.
  useEffect(() => {
    apply(current)
  }, [current])

  // When in system mode, follow OS changes live.
  useEffect(() => {
    if (mode !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => setCurrent(systemTheme())
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [mode])

  const setMode = (m: ThemeMode) => {
    localStorage.setItem(THEME_KEY, m)
    setModeState(m)
    setCurrent(resolve(m))
  }

  const order: ThemeMode[] = ['light', 'dark', 'system']
  const cycle = () => {
    const next = order[(order.indexOf(mode) + 1) % order.length]
    setMode(next)
  }

  return (
    <ThemeContext.Provider value={{ mode, setMode, cycle, current }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme(): ThemeValue {
  const v = useContext(ThemeContext)
  if (!v) throw new Error('useTheme must be used within <ThemeProvider>')
  return v
}
