// Typed REST facade over the Go backend. Every endpoint helper here is a thin
// wrapper over the three transport primitives in core/http.ts (request /
// upload / download) — there is no fetch() and no error parsing in this file.
// All calls are same-origin in dev (vite proxy) and prod (Nginx).

import { request, upload, download } from './http'
export { ApiError, setTokenGetter, setOnUnauthorized, errMessage, errStatus } from './http'

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
  UploadResult,
  UserPublic,
  UserStats,
  UserWithDisabled,
} from './types'

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
  if (q.author) params.set('author', q.author)
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

// Spec 28: paginated archive. The UI uses this — the unpaginated form above
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

// Spec 27: apply the shipped category tree. Idempotent — safe to click twice.
export const applyPresetCategories = () =>
  request<{ created: number; reparented: number }>('/api/admin/categories/preset', { method: 'POST' })

export const deleteCategory = (id: number) =>
  request<void>(`/api/categories/${id}`, { method: 'DELETE' })

export const listTagsWithCount = () =>
  request<TagWithCount[]>('/api/tags/with-count')

export const deleteTag = (id: number) =>
  request<void>(`/api/tags/${id}`, { method: 'DELETE' })

// Spec 25: rename a tag in place. Associations survive because post_tags is
// keyed by tag id; delete+create would silently drop them.
export const updateTag = (id: number, name: string) =>
  request<Tag>(`/api/tags/${id}`, { method: 'PUT', body: JSON.stringify({ name }) })

// --- prev/next nav + public read ---
// Spec 34: daily snapshot state + on-demand run.
export const getAutoBackupStatus = () =>
  request<{ enabled: boolean; keep: number; last_at: string; files: number }>('/api/admin/backup/auto')

export const runAutoBackup = () =>
  request<{ file: string }>('/api/admin/backup/auto', { method: 'POST' })

// Spec 35: related posts.
export const getRelatedPosts = (id: string) => request<Post[]>(`/api/posts/${id}/related`)

export const getPostPrevNext = (id: string) =>
  request<PostNav>(`/api/posts/${id}/nav`)

// Spec 23: report one read of a post. The server de-duplicates per visitor per
// day, ignores the author and crawlers, and returns the current count so the
// UI can show the post-trip value. Fire-and-forget: a failed report must never
// break reading the article, so errors are swallowed into `null`.
export type ViewReport = { counted: boolean; view_count: number; reason?: string }

export const reportView = (id: string) =>
  request<ViewReport>(`/api/posts/${id}/view`, { method: 'POST' }).catch(() => null)

// Spec 31: revision history.
export const listRevisions = (id: string) =>
  request<Revision[]>(`/api/posts/${id}/revisions`)

export const restoreRevision = (id: string, revId: number) =>
  request<Post>(`/api/posts/${id}/revisions/${revId}/restore`, { method: 'POST' })

// Spec 32: markdown import via multipart (the shared request helper would set
// Content-Type: application/json and break the multipart boundary).
export const importMarkdown = (fd: FormData) =>
  upload<{ created: number; skipped: { name: string; reason: string }[] }>(
    '/api/admin/import/markdown',
    fd,
  )

export const getPostPublic = (id: string) =>
  request<Post>(`/api/posts/${id}/public`)

// Spec 29: resolve a permanent link.
export const getPostBySlug = (slug: string) =>
  request<Post>(`/api/permalinks/${encodeURIComponent(slug)}`)

// Spec 30: pull a post into the banned vault (root only).
export const banPost = (id: string) =>
  request<Post>(`/api/admin/ban/${id}`, { method: 'POST' })

// Spec 26: one bulk action over many posts. The server routes every id
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
// Sends an image as multipart/form-data. The browser sets the multipart
// boundary automatically; do NOT set Content-Type here.
export const uploadImage = (file: File) => {
  const form = new FormData()
  form.append('file', file)
  return upload<UploadResult>('/api/upload', form)
}

// --- image library (browse + delete locally-stored uploads) ---
export const listUploads = () => request<LocalUpload[]>('/api/uploads')
export const deleteUpload = (name: string) =>
  request<{ message: string }>(`/api/uploads/${encodeURIComponent(name)}`, { method: 'DELETE' })

// --- SM.MS image hosting ---
// Spec #7: the SM.MS token is now held server-side (.env SM_MS_TOKEN or the
// site-config img_host_token). The browser uploads through /api/upload, which
// the backend proxies to SM.MS with the secret — the token never reaches the
// client. `uploadImage` above is the single safe path.
