// Typed REST facade over the Go backend. Every endpoint helper here is a thin
// wrapper over the three transport primitives in core/http.ts (request /
// upload / download) — there is no fetch() and no error parsing in this file.
// All calls are same-origin in dev (vite proxy) and prod (Nginx).

import { request, upload, download } from './http'
export { ApiError, setTokenGetter, setOnUnauthorized, setOnPasswordChangeRequired, errMessage, errStatus } from './http'

import type {
  AuthResult,
  AuthorView,
  BackupResult,
  BackupStats,
  CaptchaIssued,
  Category,
  CategoryWithCount,
  CreatePostInput,
  DashboardData,
  ListPostsQuery,
  LoginLog,
  Post,
  PostNav,
  Revision,
  PostPage,
  SiteConfigFull,
  SiteConfigPublic,
  SnapshotInfo,
  Tag,
  TagWithCount,
  TrashPost,
  ActionLog,
  UpdatePostInput,
  UploadDeleteResult,
  UploadPage,
  UploadResult,
  UserPublic,
  UserStats,
  UserWithDisabled,
} from './types'

// --- auth ---
// Registering issues a JWT, so the new account is signed in straight away.
// `code` is only required while the site has email verification switched on
// (experimental); the server ignores it otherwise.
export const register = (b: {
  username: string
  email: string
  password: string
  code?: string
}) => request<AuthResult>('/api/auth/register', { method: 'POST', body: JSON.stringify(b) })

// Ask for a one-time code. The server answers identically whether or not the
// address belongs to an account, so the UI must not infer anything from it.
export const requestEmailCode = (b: { email: string; purpose: 'register' | 'reset_password' }) =>
  request<{ message: string }>('/api/auth/email-code', { method: 'POST', body: JSON.stringify(b) })

// Finish a self-service reset. Every session is revoked by the reset, so the
// caller must sign in again afterwards.
export const resetPasswordWithCode = (b: { email: string; code: string; password: string }) =>
  request<{ message: string }>('/api/auth/password/reset', {
    method: 'POST',
    body: JSON.stringify(b),
  })

// Verified email change: request mails a code to the NEW address, confirm
// applies it and returns the refreshed account.
export const requestEmailChange = (b: { email: string }) =>
  request<{ message: string }>('/api/me/email-change/request', {
    method: 'POST',
    body: JSON.stringify(b),
  })

export const confirmEmailChange = (b: { email: string; code: string }) =>
  request<UserPublic>('/api/me/email-change/confirm', {
    method: 'POST',
    body: JSON.stringify(b),
  })

export const login = (b: {
  email: string
  password: string
  captcha_id?: string
  captcha_code?: string
}) => request<AuthResult>('/api/auth/login', { method: 'POST', body: JSON.stringify(b) })

// Fetch a fresh login challenge. Always answers — even with the feature
// switched off — so the form can tell "not required" from "failed to load".
//
// No 429 auto-retry: a challenge is single-use, so a retry draws a brand new
// picture and spends another slot of the per-IP issue budget — the exact
// resource that just ran out. The form turns a 429 into a countdown instead.
export const getCaptcha = () => request<CaptchaIssued>('/api/auth/captcha', undefined, { retry429: false })

export const me = () => request<UserPublic>('/api/auth/me')

export const logout = () => request<{ message: string }>('/api/auth/logout', { method: 'POST' })

// Update the caller's own profile (username / email / display_name / bio /
// avatar). All fields are optional; sending one changes only that field, and
// an explicit empty string clears it.
export const updateMe = (b: { username?: string; email?: string; display_name?: string; bio?: string; avatar?: string }) =>
  request<UserPublic>('/api/auth/me', { method: 'PUT', body: JSON.stringify(b) })

// Change the caller's own password (first-boot admin flow): verifies the
// current password and clears must_change_password on success.
// The server bumps the token version on a password change (revoking every
// previously issued JWT) and returns a fresh token so the current session
// survives. Callers must swap the new token in (useAuth().adoptToken).
export const changePassword = (b: { current_password: string; new_password: string }) =>
  request<{ message: string; token: string; expires: string }>('/api/auth/change-password', {
    method: 'POST',
    body: JSON.stringify(b),
  })

// Public author profile + published posts.
export const getAuthor = (username: string) =>
  request<AuthorView>(`/api/authors/${encodeURIComponent(username)}`)

// Personal-center metrics: the caller's post count + total views.
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

// `category_id: undefined` in the editor means "this post has no category",
// and the update endpoint needs that as an explicit 0: an absent key means
// "leave the category alone", which is exactly what 不分类 must not do. Create
// is the other way round — there nil already means "none" — so the translation
// lives here, in the one function that speaks the write contract, rather than
// in the editor, which should not know about either encoding.
export const updatePost = (id: string, b: UpdatePostInput) =>
  request<Post>(`/api/posts/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ ...b, category_id: b.category_id ?? 0 }),
  })

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
  // Category is a name (or a legacy numeric id): presence check only, an
  // empty string means "no filter".
  if (q.category) params.set('category', q.category)
  if (q.tag) params.set('tag', q.tag)
  if (q.author) params.set('author', q.author)
 // Personal-center sort: sort=created_at|views, order=asc|desc.
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

// Paginated archive. The UI uses this — the unpaginated form above
// loads every published post on every visit.
export const listArchivePaged = (q: { page?: number; size?: number } = {}) => {
  const p = new URLSearchParams()
  if (q.page) p.set('page', String(q.page))
  if (q.size) p.set('size', String(q.size))
  const qs = p.toString()
  return request<PostPage>(`/api/posts/archive/paged${qs ? `?${qs}` : ''}`)
}

// --- admin: category CRUD + tag with count ---
export const listCategoriesWithCount = () =>
  request<CategoryWithCount[]>('/api/categories/with-count')

export const createCategory = (b: { name: string; parent_id?: number | null }) =>
  request<Category>('/api/categories', { method: 'POST', body: JSON.stringify(b) })

export const updateCategory = (id: number, b: { name: string; parent_id?: number | null }) =>
  request<Category>(`/api/categories/${id}`, { method: 'PUT', body: JSON.stringify(b) })

// Apply the shipped category tree. Idempotent — safe to click twice.
export const applyPresetCategories = () =>
  request<{ created: number; reparented: number }>('/api/admin/categories/preset', { method: 'POST' })

export const deleteCategory = (id: number) =>
  request<void>(`/api/categories/${id}`, { method: 'DELETE' })

export const listTagsWithCount = () =>
  request<TagWithCount[]>('/api/tags/with-count')

export const deleteTag = (id: number) =>
  request<void>(`/api/tags/${id}`, { method: 'DELETE' })

// Rename a tag in place. Associations survive because post_tags is
// keyed by tag id; delete+create would silently drop them.
export const updateTag = (id: number, name: string) =>
  request<Tag>(`/api/tags/${id}`, { method: 'PUT', body: JSON.stringify({ name }) })

// --- prev/next nav + public read ---
// Daily snapshot state + on-demand run.
export const getAutoBackupStatus = () =>
  request<{ enabled: boolean; keep: number; last_at: string; files: number }>('/api/admin/backup/auto')

export const runAutoBackup = () =>
  request<{ file: string }>('/api/admin/backup/auto', { method: 'POST' })

// Related posts.
export const getRelatedPosts = (id: string) => request<Post[]>(`/api/posts/${id}/related`)

export const getPostPrevNext = (id: string) =>
  request<PostNav>(`/api/posts/${id}/nav`)

// Report one read of a post. The server de-duplicates per visitor per
// day, ignores the author and crawlers, and returns the current count so the
// UI can show the post-trip value. Fire-and-forget: a failed report must never
// break reading the article, so errors are swallowed into `null`.
export type ViewReport = { counted: boolean; view_count: number; reason?: string }

export const reportView = (id: string) =>
  request<ViewReport>(`/api/posts/${id}/view`, { method: 'POST' }).catch(() => null)

// Revision history.
export const listRevisions = (id: string) =>
  request<Revision[]>(`/api/posts/${id}/revisions`)

export const restoreRevision = (id: string, revId: number) =>
  request<Post>(`/api/posts/${id}/revisions/${revId}/restore`, { method: 'POST' })

// Markdown import via multipart (the shared request helper would set
// Content-Type: application/json and break the multipart boundary).
export const importMarkdown = (fd: FormData) =>
  upload<{ created: number; skipped: { name: string; reason: string }[] }>(
    '/api/admin/import/markdown',
    fd,
  )

export const getPostPublic = (id: string) =>
  request<Post>(`/api/posts/${id}/public`)

// Resolve a permanent link.
export const getPostBySlug = (slug: string) =>
  request<Post>(`/api/permalinks/${encodeURIComponent(slug)}`)

// Pull a post into the banned vault (root only).
export const banPost = (id: string) =>
  request<Post>(`/api/admin/ban/${id}`, { method: 'POST' })

// One bulk action over many posts. The server routes every id
// through the single-item service method, so partial permission failures come
// back as `skipped` instead of failing the whole batch.
export interface BatchResult {
  affected: number
  skipped: { id: string; reason: string }[]
}

export const batchPosts = (
  ids: string[],
  action: 'publish' | 'private' | 'delete' | 'set_category' | 'add_tags' | 'remove_tags',
  extra: { category_id?: number; tags?: string[] } = {},
) =>
  request<BatchResult>('/api/admin/posts/batch', {
    method: 'POST',
    body: JSON.stringify({ ids, action, ...extra }),
  })

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

// --- content review / ban workflow ---
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

// --- site config ---
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

// --- registration gate ---
export const registrationOpen = () =>
  request<{ open: boolean }>('/api/auth/register/open')

// --- admin: dashboard ---
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

// Root-only: list the VACUUM INTO snapshots available for restore.
export const listSnapshots = () =>
  request<SnapshotInfo[]>('/api/backup/snapshots')

// Root-only: replace the whole database with a snapshot file. The service
// auto-snapshots the current state first, so the restore is undoable.
//
// The confirmation phrase is required by the server: a snapshot restore
// replaces the content AND the account table, so it is gated like the backup
// import rather than trusted because the caller clicked twice in the UI.
// `allowDataLoss` is a separate, explicit acknowledgement for the case the
// server refuses by default — a snapshot that would delete every article.
export const RESTORE_CONFIRM_PHRASE = 'RESTORE-SNAPSHOT'
export const restoreSnapshot = (file: string, allowDataLoss = false) =>
  request<BackupStats>('/api/backup/restore-snapshot', {
    method: 'POST',
    body: JSON.stringify({
      file,
      confirm: RESTORE_CONFIRM_PHRASE,
      ...(allowDataLoss ? { allow_data_loss: true } : {}),
    }),
  })

// Root-only: preview what a snapshot holds without applying it. Used to tell
// the operator "this restores 0 articles over your 42" BEFORE they commit.
export const previewSnapshotRestore = (file: string) =>
  request<BackupStats>(`/api/backup/restore-snapshot?dry_run=1`, {
    method: 'POST',
    body: JSON.stringify({ file, confirm: RESTORE_CONFIRM_PHRASE }),
  })

// Backups are written to the server's disk, so the file itself must be
// pulled separately. Returns a Blob the caller can hand to saveAs().
export const downloadBackup = (filename: string) =>
  download(`/api/backup/download?file=${encodeURIComponent(filename)}`)

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
//
// Two-step by design: call it with dryRun=true first to get the row counts
// without touching anything, then call it again with dryRun=false. The server
// refuses an apply that does not carry confirm="REPLACE-ALL-DATA", so a
// single accidental click can no longer wipe the site.
export const importBackup = (data: unknown, dryRun = false) =>
  dryRun
    ? request<BackupStats>('/api/backup/import?dry_run=1', {
        method: 'POST',
        body: JSON.stringify(data),
      })
    : request<BackupStats>('/api/backup/import', {
        method: 'POST',
        body: JSON.stringify({ ...(data as Record<string, unknown>), confirm: 'REPLACE-ALL-DATA' }),
      })

// --- admin: trash / recovery ---
export const listTrash = () =>
  request<TrashPost[]>('/api/trash')

export const restoreTrash = (id: string) =>
  request<void>(`/api/trash/${id}/restore`, { method: 'POST' })

export const emptyTrash = () =>
  request<{ deleted: number }>('/api/trash/empty', { method: 'POST' })

// --- image upload ---
// Sends an image as multipart/form-data. The browser sets the multipart
// boundary automatically; do NOT set Content-Type here.
//
// `kind` picks the server-side per-file ceiling and the record's purpose:
// article images default to 5 MB, avatars to 512 KB.
export const uploadImage = (file: File, kind: 'post' | 'avatar' | 'site' = 'post') => {
  const form = new FormData()
  form.append('file', file)
  form.append('kind', kind)
  return upload<UploadResult>('/api/upload', form)
}

// --- image library (admin: site-wide assets) ---
export const listUploads = (opts?: {
  owner?: string
  kind?: string
  status?: 'live' | 'deleted' | 'all'
  page?: number
  size?: number
}) => {
  const q = new URLSearchParams()
  if (opts?.owner) q.set('owner', opts.owner)
  if (opts?.kind) q.set('kind', opts.kind)
  if (opts?.status) q.set('status', opts.status)
  if (opts?.page) q.set('page', String(opts.page))
  if (opts?.size) q.set('size', String(opts.size))
  const qs = q.toString()
  if (qs) return request<UploadPage>(`/api/uploads?${qs}`)
  return request<UploadPage>('/api/uploads')
}

// Soft delete (default): the file stays on disk so published articles keep
// rendering, but it leaves every list and can never be re-used.
export const deleteUpload = (name: string) =>
  request<UploadDeleteResult>(`/api/uploads/${encodeURIComponent(name)}`, { method: 'DELETE' })

// Hard delete: removes the bytes. Only admins, and it breaks any article that
// still embeds the image — which is exactly why it is a separate call.
export const purgeUpload = (name: string) =>
  request<UploadDeleteResult>(`/api/uploads/${encodeURIComponent(name)}?hard=1`, { method: 'DELETE' })

export const restoreUpload = (name: string) =>
  request<{ message: string }>(`/api/uploads/${encodeURIComponent(name)}/restore`, { method: 'POST' })

// --- my uploads (any signed-in author) ---
// Scoped to the caller by the server: there is no `owner` parameter here on
// purpose, and passing one would be ignored.
export const listMyUploads = (page = 1, size = 20) =>
  request<UploadPage>(`/api/me/uploads?page=${page}&size=${size}`)

export const deleteMyUpload = (name: string) =>
  request<UploadDeleteResult>(`/api/me/uploads/${encodeURIComponent(name)}`, { method: 'DELETE' })

// --- SM.MS image hosting ---
// The SM.MS token is now held server-side (.env SM_MS_TOKEN or the
// site-config img_host_token). The browser uploads through /api/upload, which
// the backend proxies to SM.MS with the secret — the token never reaches the
// client. `uploadImage` above is the single safe path.
