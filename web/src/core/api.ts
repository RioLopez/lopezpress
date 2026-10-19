// Thin REST client over the Go backend. All calls are same-origin in dev
// (vite proxy) and prod (Nginx). The Authorization header is attached from
// the auth context via a closure that's set by setTokenGetter.

import type {
  AuthResult,
  AuthorView,
  BackupResult,
  BackupStats,
  Category,
  CategoryWithCount,
  CreatePostInput,
  DashboardData,
  ListPostsQuery,
  LoginLog,
  LocalUpload,
  Post,
  PostNav,
  PostPage,
  SiteConfigFull,
  SiteConfigPublic,
  SnapshotInfo,
  Tag,
  TagWithCount,
  TrashPost,
  ActionLog,
  UpdatePostInput,
  UploadResult,
  UserPublic,
  UserStats,
  UserWithDisabled,
} from './types'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

let tokenGetter: () => string = () => ''
export function setTokenGetter(fn: () => string) {
  tokenGetter = fn
}

// Invoked on a 401 from a non-auth endpoint (i.e. an expired/invalid token),
// so the app can clear the session and bounce to /login. Registered by the
// auth context. Business 401s from /api/auth/* are handled per-call.
let onUnauthorized: (() => void) | null = null
export function setOnUnauthorized(fn: (() => void) | null) {
  onUnauthorized = fn
}

// Retry a failed fetch on 429 (rate-limited), honouring the server's
// Retry-After header. The general per-identity limiter can trip when a
// single page mount fires a burst of parallel requests (e.g. the admin
// review workbench). Without a backoff the first burst would surface the
// whole page as "failed to load" even though the data is fine 100ms later.
//
// Only safe GET / read-shaped requests should be retried: a 429 on a
// POST/PUT/DELETE means the write did NOT reach the handler (the limiter
// sits outside the handlers, before CORS/Recover), so retrying a mutation
// would risk a double-apply. `request` therefore retries a 429 only when
// the method is GET (or no method, i.e. GET) and there is no body.
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

async function fetchWith429Retry(
  path: string,
  init: RequestInit | undefined,
): Promise<Response> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const canRetry = method === 'GET' && !init?.body
  let attempt = 0
  const maxAttempts = canRetry ? 3 : 1
  for (;;) {
    const res = await fetch(path, { ...init })
    if (res.status !== 429 || attempt >= maxAttempts - 1) return res
    // Read Retry-After (seconds) from the limiter; fall back to a fixed
    // 250ms backoff if the header is absent.
    const ra = res.headers.get('Retry-After')
    const delay = ra ? Math.min(parseInt(ra, 10) * 1000, 2000) : 250
    await sleep(delay)
    attempt++
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers || {})
  const t = tokenGetter()
  if (t) headers.set('Authorization', `Bearer ${t}`)
  if (init?.body) headers.set('Content-Type', 'application/json')

  const res = await fetchWith429Retry(path, { ...init, headers })
  if (res.status === 204) return undefined as T

  let data: any
  const text = await res.text()
  data = text ? JSON.parse(text) : null

  if (!res.ok) {
    if (res.status === 401 && onUnauthorized && !path.startsWith('/api/auth/')) {
      onUnauthorized()
    }
    const msg = (data && data.error) || `HTTP ${res.status}`
    throw new ApiError(res.status, msg)
  }
  return data as T
}

// --- auth ---
// Registering issues a JWT, so the new account is signed in straight away.
export const register = (b: {
  username: string
  email: string
  password: string
}) => request<AuthResult>('/api/auth/register', { method: 'POST', body: JSON.stringify(b) })

export const login = (b: { email: string; password: string }) =>
  request<AuthResult>('/api/auth/login', { method: 'POST', body: JSON.stringify(b) })

export const me = () => request<UserPublic>('/api/auth/me')

export const logout = () => request<{ message: string }>('/api/auth/logout', { method: 'POST' })

// Update the caller's own profile (display_name / bio / avatar).
export const updateMe = (b: { display_name?: string; bio?: string; avatar?: string }) =>
  request<UserPublic>('/api/auth/me', { method: 'PUT', body: JSON.stringify(b) })

// Change the caller's own password (first-boot admin flow): verifies the
// current password and clears must_change_password on success.
export const changePassword = (b: { current_password: string; new_password: string }) =>
  request<{ message: string }>('/api/auth/change-password', { method: 'POST', body: JSON.stringify(b) })

// Public author profile + published posts.
export const getAuthor = (username: string) =>
  request<AuthorView>(`/api/authors/${encodeURIComponent(username)}`)

// Personal-center metrics: the caller's post count + total views (spec 20).
export const myStats = () => request<UserStats>('/api/me/stats')

// Self-service login history for the security center: the caller's own recent
// login attempts (success + failure).
export const meLoginHistory = (limit = 20) =>
  request<LoginLog[]>(`/api/me/login-history?limit=${limit}`)

// Revokes every JWT the caller holds ("log out all devices"). After this the
// local token is invalid, so the caller must clear its session.
export const logoutAll = () =>
  request<{ message: string }>('/api/auth/logout-all', { method: 'POST' })

// --- posts ---
export const getPost = (id: string) => request<Post>(`/api/posts/${id}`)

export const createPost = (b: CreatePostInput) =>
  request<Post>('/api/posts', { method: 'POST', body: JSON.stringify(b) })

export const updatePost = (id: string, b: UpdatePostInput) =>
  request<Post>(`/api/posts/${id}`, { method: 'PUT', body: JSON.stringify(b) })

export const deletePost = (id: string) =>
  request<void>(`/api/posts/${id}`, { method: 'DELETE' })

// posts: paginated / search ---

// Returns a PostPage when any of page/size/q/category/tag is set; otherwise a bare array
// (legacy shape) for the no-filter case.
export const listPostsPaged = (q: ListPostsQuery = {}) => {
  const params = new URLSearchParams()
  if (q.status) params.set('status', q.status)
  if (q.q) params.set('q', q.q)
  if (q.page) params.set('page', String(q.page))
  if (q.size) params.set('size', String(q.size))
  // Presence checks, not truthiness: `if (q.category)` would skip id 0.
  if (q.category !== undefined) params.set('category', String(q.category))
  if (q.tag) params.set('tag', q.tag)
  // Personal-center sort (spec 20): sort=created_at|views, order=asc|desc.
  if (q.sort) params.set('sort', q.sort)
  if (q.order) params.set('order', q.order)
  const qs = params.toString()
  const path = '/api/posts' + (qs ? `?${qs}` : '')
  return request<PostPage>(path)
}

// --- categories / tags / recent ---
export const listCategories = () => request<Category[]>('/api/categories')
export const listTags = () => request<Tag[]>('/api/tags')
export const getRecentPosts = () => request<Post[]>('/api/posts/recent')
export const listArchive = () => request<Post[]>('/api/posts/archive')

// --- admin: category CRUD + tag with count ---
export const listCategoriesWithCount = () =>
  request<CategoryWithCount[]>('/api/categories/with-count')

export const createCategory = (b: { name: string }) =>
  request<Category>('/api/categories', { method: 'POST', body: JSON.stringify(b) })

export const updateCategory = (id: number, b: { name: string }) =>
  request<Category>(`/api/categories/${id}`, { method: 'PUT', body: JSON.stringify(b) })

export const deleteCategory = (id: number) =>
  request<void>(`/api/categories/${id}`, { method: 'DELETE' })

export const listTagsWithCount = () =>
  request<TagWithCount[]>('/api/tags/with-count')

export const deleteTag = (id: number) =>
  request<void>(`/api/tags/${id}`, { method: 'DELETE' })

// --- prev/next nav + public read ---
export const getPostPrevNext = (id: string) =>
  request<PostNav>(`/api/posts/${id}/nav`)

export const getPostPublic = (id: string) =>
  request<Post>(`/api/posts/${id}/public`)

// --- admin: user management ---
export const listUsers = () =>
  request<UserWithDisabled[]>('/api/users')

export const setUserDisabled = (id: string, disabled: boolean) =>
  request<UserWithDisabled>(`/api/users/${id}/disabled`, {
    method: 'PUT',
    body: JSON.stringify({ disabled }),
  })

// Full user CRUD (admin). CreateUser/UpdateUser/ResetPassword/DeleteUser.
export const createUser = (b: { username: string; email: string; password: string; role?: string }) =>
  request<UserWithDisabled>('/api/users', { method: 'POST', body: JSON.stringify(b) })

export const updateUser = (id: string, b: { role?: string; disabled?: boolean }) =>
  request<UserWithDisabled>(`/api/users/${id}`, { method: 'PUT', body: JSON.stringify(b) })

export const resetPassword = (id: string, password: string) =>
  request<{ message: string }>(`/api/users/${id}/reset-password`, {
    method: 'POST',
    body: JSON.stringify({ password }),
  })

export const deleteUser = (id: string) =>
  request<void>(`/api/users/${id}`, { method: 'DELETE' })

export const listUserLogins = (id: string, limit = 50) =>
  request<LoginLog[]>(`/api/users/${id}/logins?limit=${limit}`)

// --- content review / ban workflow (spec 24) ---
// Pending queue: posts awaiting a review decision. Root sees every pending
// post; an admin only sees pending posts authored by plain users.
export const listPending = () => request<Post[]>('/api/admin/pending')

// Approve or reject a pending post. Rejecting requires a review note.
export const reviewPost = (
  id: string,
  b: { decision: 'approve' | 'reject'; note?: string },
) =>
  request<Post>(`/api/admin/review/${id}`, {
    method: 'POST',
    body: JSON.stringify(b),
  })

// Root-only: list every banned post (the ban vault).
export const listBanned = () => request<Post[]>('/api/admin/banned')

// Root-only: send a banned post back to its author for revisions.
export const sendBackBanned = (id: string, note: string) =>
  request<Post>(`/api/admin/banned/${id}/send-back`, {
    method: 'POST',
    body: JSON.stringify({ note }),
  })

// Root-only: permanently delete a banned post (hard delete, unrecoverable).
export const deleteBanned = (id: string) =>
  request<void>(`/api/admin/banned/${id}`, { method: 'DELETE' })

// --- site config (spec 05) ---
// Public read-only subset (brand/title/footer). No secrets.
export const getSiteConfig = () =>
  request<SiteConfigPublic>('/api/site-config')

// Admin full config (incl. image-host token + registration flag).
export const getAdminSiteConfig = () =>
  request<SiteConfigFull>('/api/admin/site-config')

export const updateSiteConfig = (cfg: Partial<SiteConfigFull>) =>
  request<SiteConfigFull>('/api/admin/site-config', {
    method: 'PUT',
    body: JSON.stringify(cfg),
  })

// Verify SMTP settings with a synchronous test e-mail to the admin's own inbox.
export const sendTestMail = () =>
  request<{ sent: boolean }>('/api/admin/mail/test', { method: 'POST' })

// --- registration gate (spec 16) ---
export const registrationOpen = () =>
  request<{ open: boolean }>('/api/auth/register/open')

// --- admin: dashboard (spec 06) ---
export const getDashboard = () =>
  request<DashboardData>('/api/admin/dashboard')

// Admin operation audit trail: recent content + admin mutations.
export const listActions = (limit = 50, actor?: string) => {
  const qs = new URLSearchParams({ limit: String(limit) })
  if (actor) qs.set('actor', actor)
  return request<ActionLog[]>(`/api/admin/actions?${qs.toString()}`)
}

// No batch endpoint on the backend; fan out one call per user and tolerate
// partial failure. Returns how many succeeded so the caller can report it.
export async function batchSetUserDisabled(
  ids: string[],
  disabled: boolean,
): Promise<{ ok: number; fail: number }> {
  const results = await Promise.allSettled(
    ids.map((id) => setUserDisabled(id, disabled)),
  )
  const ok = results.filter((r) => r.status === 'fulfilled').length
  return { ok, fail: results.length - ok }
}

// --- admin: data backup ---
export const exportBackup = () =>
  request<BackupResult>('/api/backup', { method: 'POST' })

// Root-only: list the VACUUM INTO snapshots available for restore (P2-14).
export const listSnapshots = () =>
  request<SnapshotInfo[]>('/api/backup/snapshots')

// Root-only: replace the whole database with a snapshot file. The service
// auto-snapshots the current state first, so the restore is undoable.
export const restoreSnapshot = (file: string) =>
  request<BackupStats>('/api/backup/restore-snapshot', {
    method: 'POST',
    body: JSON.stringify({ file }),
  })

// Backups are written to the server's disk, so the file itself must be
// pulled separately. Returns a Blob the caller can hand to saveAs().
export async function downloadBackup(filename: string): Promise<Blob> {
  const headers = new Headers()
  const t = tokenGetter()
  if (t) headers.set('Authorization', `Bearer ${t}`)

  const res = await fetch(
    `/api/backup/download?file=${encodeURIComponent(filename)}`,
    { headers },
  )
  if (!res.ok) {
    // The download endpoint serves JSON-encoded errors on failure.
    const text = await res.text()
    let msg = `HTTP ${res.status}`
    try {
      const parsed = JSON.parse(text)
      if (parsed && parsed.error) msg = parsed.error
    } catch {
      /* non-JSON body (e.g. HTML error page) — keep the status fallback */
    }
    throw new ApiError(res.status, msg)
  }
  return res.blob()
}

// Triggers a browser download for an already-fetched Blob.
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  // Give the browser a tick to start the download before revoking.
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

// Restores the database from an exported backup JSON (destructive: replaces
// current contents). Send the already-parsed object so the caller can read
// the file locally first.
export const importBackup = (data: unknown) =>
  request<BackupStats>('/api/backup/import', {
    method: 'POST',
    body: JSON.stringify(data),
  })

// --- admin: trash / recovery ---
export const listTrash = () =>
  request<TrashPost[]>('/api/trash')

export const restoreTrash = (id: string) =>
  request<void>(`/api/trash/${id}/restore`, { method: 'POST' })

export const emptyTrash = () =>
  request<{ deleted: number }>('/api/trash/empty', { method: 'POST' })


// --- image upload ---

// Sends an image as multipart/form-data. The browser sets the
// multipart boundary automatically; do NOT set Content-Type here.
export const uploadImage = (file: File) => {
  const form = new FormData()
  form.append('file', file)
  const headers = new Headers()
  const t = tokenGetter()
  if (t) headers.set('Authorization', `Bearer ${t}`)

  return fetch('/api/upload', { method: 'POST', body: form, headers }).then(
    async (res) => {
      const text = await res.text()
      let data: any = null
      try {
        data = text ? JSON.parse(text) : null
      } catch {
        /* non-JSON */
      }
      if (!res.ok) {
        const msg = (data && data.error) || `HTTP ${res.status}`
        throw new ApiError(res.status, msg)
      }
      return data as UploadResult
    }
  )
}

// --- image library (browse + delete locally-stored uploads) ---
export const listUploads = () => request<LocalUpload[]>('/api/uploads')
export const deleteUpload = (name: string) =>
  request<{ message: string }>(`/api/uploads/${encodeURIComponent(name)}`, { method: 'DELETE' })

// --- SM.MS image hosting ---
// Spec #7: the SM.MS token is now held server-side (.env SM_MS_TOKEN or the
// site-config img_host_token). The browser uploads through /api/upload, which
// the backend proxies to SM.MS with the secret — the token never reaches the
// client. The old direct-browser upload (uploadImageSmms) was removed so no
// code path can leak the token to localStorage. `uploadImage` below is the
// single safe path.
