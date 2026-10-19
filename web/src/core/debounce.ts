import { useEffect, useRef, useState } from 'preact/hooks'

// Stable debounced value: re-renders only when the debounced value changes.
export function useDebounce<T>(value: T, delayMs: number): T {
  const [draft, setDraft] = useState<T>(value)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    timer.current = setTimeout(() => setDraft(value), delayMs)
    return () => { if (timer.current) clearTimeout(timer.current) }
  }, [value, delayMs])

  return draft
}
