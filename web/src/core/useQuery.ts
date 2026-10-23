// Tiny data-fetching hooks (zero dependencies) so pages don't each hand-roll
// the same three-state machine (loading / data / error) plus race protection.
//
// This deliberately does NOT pull in TanStack Query: ~24 pages need `{data,
// loading, error, reload}` and a busy-flag wrapper, nothing more. ~80 lines of
// local code beats a dependency that would break the "3 frontend packages"
// constraint.

import { useCallback, useEffect, useRef, useState } from 'preact/hooks'
import type { Dispatch, StateUpdater } from 'preact/hooks'

export interface QueryState<T> {
  data: T | null
  loading: boolean
  error: boolean
  // Awaitable so a mutation can `await reload()` before clearing its busy
  // flag, mirroring the old hand-rolled `load()` pattern. The returned promise
  // resolves after the fresh data has been committed to state.
  reload: () => Promise<void>
  // Exposed so a caller can do optimistic updates (flip a row, then reconcile
  // against the server response) without managing a parallel useState.
  setData: Dispatch<StateUpdater<T | null>>
}

export interface UseQueryOptions {
  // When false, no request is sent and state stays at its initial value. Flip
  // to true to fetch (e.g. once auth has resolved into a role that may load).
  // Defaults to true.
  enabled?: boolean
  // Called on fetch failure. Omit it to keep the `error`-flag contract (the
  // error-state pages). Pages that surface failures as a toast instead pass
  // this; the `error` flag is still set, they just don't render it.
  onError?: (e: unknown) => void
}

// Runs `fn` whenever any entry in `deps` changes and exposes the result.
// `reload` lets the caller force a refresh without changing `deps` (retry
// button, after a resubmit). Stale responses from the *effect* path are
// dropped via a cancellation flag, so an out-of-order resolve can't clobber
// newer data. `reload` itself is an explicit user action and always commits.
export function useQuery<T>(
  fn: () => Promise<T>,
  deps: unknown[],
  opts: UseQueryOptions = {},
): QueryState<T> {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  // Keep the latest fn/opts in refs so `reload` (stable identity) always calls
  // the current closure and reads the current onError, without the effect
  // re-running on every render.
  const fnRef = useRef(fn)
  const optsRef = useRef(opts)
  useEffect(() => {
    fnRef.current = fn
    optsRef.current = opts
  })

  const enabled = opts.enabled !== false

  useEffect(() => {
    if (!enabled) return
    let cancelled = false
    setLoading(true)
    setError(false)
    fnRef.current()
      .then((d) => {
        if (!cancelled) setData(d)
      })
      .catch((e) => {
        if (!cancelled) {
          setError(true)
          optsRef.current.onError?.(e)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, enabled])

  const reload = useCallback(async () => {
    setLoading(true)
    setError(false)
    try {
      const d = await fnRef.current()
      setData(d)
    } catch (e) {
      setError(true)
      optsRef.current.onError?.(e)
    } finally {
      setLoading(false)
    }
  }, [])

  return { data, loading, error, reload, setData }
}

export interface MutationState<TArgs extends unknown[], TResult> {
  busy: boolean
  run: (...args: TArgs) => Promise<TResult>
}

// Wraps an async mutation with a `busy` flag. `run` re-throws on failure so
// the caller keeps its own success/error handling (errMessage + pushToast);
// this only removes the manual `setBusy(true)` / `finally(setBusy(false))`
// boilerplate that otherwise repeats around every write.
export function useMutation<TArgs extends unknown[], TResult>(
  fn: (...args: TArgs) => Promise<TResult>,
): MutationState<TArgs, TResult> {
  const [busy, setBusy] = useState(false)

  const run = useCallback(
    async (...args: TArgs) => {
      setBusy(true)
      try {
        return await fn(...args)
      } finally {
        setBusy(false)
      }
    },
    [fn],
  )

  return { busy, run }
}
