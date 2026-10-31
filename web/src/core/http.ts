// The single network outlet for the whole app. Every byte that crosses the
// wire goes through one of three primitives here:
//
//   request(path, init)  JSON in / JSON out (the common REST path)
//   upload(path, form)   multipart/form-data in / JSON out (image, markdown)
//   download(path)       binary out as a Blob (backup files)
//
// Pages never call fetch() directly — they import typed endpoint helpers from
// core/api.ts, which are all thin wrappers over these three. That keeps
// auth-header injection, 429 backoff, 401 bounce, and error parsing in exactly
// one place (the reason `downloadBackup` once missed 429 retry: it was a
// hand-rolled fetch copy).

export class ApiError extends Error {
  status: number
  // Parsed body of the failed response, when it had one. Most callers only
  // need the message; the login form is the exception — a rejected captcha
  // comes back as a 400 whose body carries the replacement challenge, so the
  // user can retry without retyping their password.
  payload?: any
  constructor(status: number, message: string, payload?: any) {
    super(message)
    this.status = status
    this.payload = payload
  }
}

let tokenGetter: () => string = () => ''
export function setTokenGetter(fn: () => string) {
  tokenGetter = fn
}

// A status => localised line resolver, installed by the i18n provider. It is
// injected rather than imported because this module deliberately has no
// dependency on the dictionary: the tests import it without a component tree.
let errorTranslator: ((status: number) => string | undefined) | null = null
export function setErrorTranslator(fn: ((status: number) => string | undefined) | null) {
  errorTranslator = fn
}

// Invoked on a 401 from a non-auth endpoint (i.e. an expired/invalid token),
// so the app can clear the session and bounce to /login. Registered by the
// auth context. Business 401s from /api/auth/* are handled per-call.
let onUnauthorized: (() => void) | null = null
export function setOnUnauthorized(fn: (() => void) | null) {
  onUnauthorized = fn
}

// While the must-change flag is set the server refuses every endpoint except
// the password-change one. The client normally learns about the flag at login
// or /me, but it can miss it: an operator running `-reset-root-password`
// against a live session turns the flag on behind an already-open tab. Without
// this hook the user just collects identical 403s and never sees the form.
let onPasswordChangeRequired: (() => void) | null = null
export function setOnPasswordChangeRequired(fn: (() => void) | null) {
  onPasswordChangeRequired = fn
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

// The one and only fetch() in the codebase. Retries a 429 (rate-limited)
// honouring the server's Retry-After header. Only safe GET / read-shaped
// requests are retried: a 429 on a POST/PUT/DELETE means the write did NOT
// reach the handler (the limiter sits outside the handlers), so retrying a
// mutation would risk a double-apply. `request` therefore retries a 429 only
// when the method is GET and there is no body.
//
// `allowRetry` lets a caller opt out. The login captcha does: a challenge is
// single-use, so a retried fetch asks the server to draw an entirely new
// picture, spending another slot of a per-IP budget that is exactly what ran
// out. Retrying there made "throttled" feel like "broken" — the user saw a
// failed load, tapped retry, and burned three more slots per tap.
async function fetchRaw(path: string, init: RequestInit, allowRetry = true): Promise<Response> {
  const method = (init.method ?? 'GET').toUpperCase()
  const canRetry = allowRetry && method === 'GET' && !init.body
  let attempt = 0
  const maxAttempts = canRetry ? 3 : 1
  for (;;) {
    const res = await fetch(path, init)
    if (res.status !== 429 || attempt >= maxAttempts - 1) return res
    const ra = res.headers.get('Retry-After')
    const delay = ra ? Math.min(parseInt(ra, 10) * 1000, 2000) : 250
    await sleep(delay)
    attempt++
  }
}

function authHeaders(extra?: HeadersInit): Headers {
  const headers = new Headers(extra || {})
  const t = tokenGetter()
  if (t) headers.set('Authorization', `Bearer ${t}`)
  return headers
}

// Tolerantly read a JSON body. Non-JSON (empty, HTML error page, plain text)
// becomes null so a failed request can still surface a status-based message
// instead of throwing a SyntaxError.
async function readJson(res: Response): Promise<any> {
  const text = await res.text()
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

function raise(res: Response, data: any, path: string): never {
  const msg = (data && data.error) || `HTTP ${res.status}`
  if (res.status === 401 && onUnauthorized && !path.startsWith('/api/auth/')) {
    onUnauthorized()
  }
  // Exact match on purpose: a business 403 (wrong password, no permission)
  // must never bounce the user to the password form. The change-password path
  // is excluded so an error there cannot make the redirect feed itself.
  if (
    res.status === 403 &&
    msg === 'password change required' &&
    onPasswordChangeRequired &&
    !path.startsWith('/api/auth/change-password')
  ) {
    onPasswordChangeRequired()
  }
  throw new ApiError(res.status, msg, data)
}

export async function request<T>(path: string, init?: RequestInit, opts?: { retry429?: boolean }): Promise<T> {
  const headers = authHeaders(init?.headers)
  if (init?.body) headers.set('Content-Type', 'application/json')

  const res = await fetchRaw(path, { ...init, headers }, opts?.retry429 !== false)
  if (res.status === 204) return undefined as T

  const data = await readJson(res)
  if (!res.ok) raise(res, data, path)
  return data as T
}

// multipart/form-data upload. The browser sets the boundary automatically —
// do NOT set Content-Type here, or the boundary is lost.
export async function upload<T>(path: string, form: FormData): Promise<T> {
  const res = await fetchRaw(path, {
    method: 'POST',
    body: form,
    headers: authHeaders(),
  })
  const data = await readJson(res)
  if (!res.ok) raise(res, data, path)
  return data as T
}

// Fetch a binary body (backup file). The download endpoint serves JSON-encoded
// errors on failure, so parse those the same way request() does.
export async function download(path: string): Promise<Blob> {
  const res = await fetchRaw(path, { headers: authHeaders() })
  if (!res.ok) {
    const data = await readJson(res)
    raise(res, data, path)
  }
  return res.blob()
}

// The error-extraction template, deduplicated from ~54 call sites. Pages used
// to write `e instanceof ApiError ? e.message : t('fallback')` by hand, which
// meant the fallback policy could never evolve. These two helpers are the only
// places that know ApiError's shape.
//
// The backend has no localised strings at all — every message it sends is
// English ("too many captcha requests, slow down", "file too large"). Rendering
// one of those inside a Chinese page is the seam this code closes: statuses
// whose cause is mechanical get a localised line, while business errors (a 400
// from the password policy, say) keep the server text, because it names the
// rule that actually failed and the UI has no better answer.
export function errMessage(e: unknown, fallback: string): string {
  if (!(e instanceof ApiError)) return fallback
  const localized = errorTranslator?.(e.status)
  return localized ?? e.message
}

export function errStatus(e: unknown): number | undefined {
  return e instanceof ApiError ? e.status : undefined
}
