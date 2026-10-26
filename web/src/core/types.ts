// DTO types mirroring the Go API's JSON responses.

export type Role = 'user' | 'admin' | 'root'
// Three-tier model (spec 24): "published" = public (visible to everyone),
// "private" = author + admin + root. The review/ban workflow adds four more:
// "pending" = awaiting review (author + admin + root), "rejected" = sent back
// for revisions (author + admin + root, carries a note), "banned" = in the
// root-only ban vault (root sees it; the author and other admins do NOT).
// `draft` is a legacy status no longer produced by this build (import maps
// `draft: true` → private), but rows written by older versions may still
// carry it, so it stays in the type and the status-label map rather than
// crashing the admin table with a missing-key warning.
export type PostStatus = 'published' | 'private' | 'pending' | 'rejected' | 'banned' | 'draft'

export interface UserPublic {
  id: string
  username: string
  email: string
  role: Role
  display_name?: string
  bio?: string
  avatar?: string
  created_at: string
  // First-boot admin flow: when true, the client must redirect the user to
  // the change-password screen after login before they can use the app.
  must_change_password?: boolean
}

export interface Category {
  id: number
  name: string
  // Spec 27: null = top level; a number = this category's parent.
  parent_id: number | null
  created_at: string
}

export interface Tag {
  id: number
  name: string
}

export interface Post {
  // Spec 29: human-readable permanent link segment ("" for legacy rows).
  slug: string
  id: string
  title: string
  body: string
  status: PostStatus
  author_id: string
  author?: UserPublic
  category_id?: number
  category?: Category
  tags?: Tag[]
  pinned: boolean
  cover?: string
  view_count: number
  // Review / ban workflow (spec 24): the reviewer's note when the post was
  // sent back for revisions, and who/when it was reviewed.
  review_note?: string
  reviewed_by?: string
  reviewed_at?: string
  created_at: string
  updated_at: string
}

export interface AuthResult {
  token: string
  expires_at: string
  user: UserPublic
}

// One login challenge. The kind decides which interaction to render, and the
// server picks it from this client's failure history — the browser cannot ask
// for the easier one.
//
// The answer is never in here. For 'image' the picture is `image`; for 'slide'
// it is the pair `bg`/`tile` plus the piece's vertical offset. The horizontal
// target is exactly what the user has to work out.
export interface CaptchaChallenge {
  id: string
  kind: 'image' | 'slide'
  image?: string
  bg?: string
  tile?: string
  tile_y?: number
  tile_size?: number
  expires_in: number
}

// Response of GET /api/auth/captcha. `enabled: false` means the server has the
// feature switched off, so the form must render no widget at all.
export interface CaptchaIssued {
  enabled: boolean
  captcha?: CaptchaChallenge
}

// What the login form hands to the transport layer once the user has solved the
// challenge. Exactly one of code/x is meaningful, depending on kind.
export interface CaptchaAnswer {
  id: string
  code: string
  x: number
}

export interface CreatePostInput {
  title: string
  body: string
  status: PostStatus
  category_id?: number
  tags?: string[]
  pinned?: boolean
  cover?: string
}

export interface UpdatePostInput {
  title?: string
  body?: string
  status?: PostStatus
  category_id?: number
  tags?: string[]
  pinned?: boolean
  cover?: string
}

// --- pagination / search (GET /api/posts with params) ---

export interface ListPostsQuery {
  status?: PostStatus | ''
  q?: string
  page?: number
  size?: number
  // Category reference: a category NAME (readable links, e.g. ?category=PHP)
  // or a numeric id as a string (legacy links). The server accepts both.
  category?: string
  tag?: string
  // Scope to a single author (personal center "my content only").
  author?: string
  // Personal-center sort (spec 20): "created_at" (default) | "views"
  sort?: 'created_at' | 'views'
  // "asc" | "desc" (default)
  order?: 'asc' | 'desc'
}

// Spec 31: one saved-before-edit snapshot.
export interface Revision {
  id: number
  post_id: string
  title: string
  body: string
  status: string
  editor_id: string
  created_at: string
}

export interface PostPage {
  items: Post[]
  total: number
  page: number
  page_size: number
}

// --- image upload (POST /api/upload) ---

// What POST /api/upload answers (spec 36). `deduped` means the library already
// held these exact bytes and nothing new was written to disk.
export interface UploadResult {
  id: string
  name: string
  url: string
  size: number
  width: number
  height: number
  mime: string
  kind: string
  deduped: boolean
  remote?: boolean
}

// One row of the image library / "my uploads" list.
export interface UploadFileInfo {
  id: string
  name: string
  url: string
  // Empty until the background worker has produced one (or when the source is
  // already small enough). Cards fall back to `url` — never assume it exists.
  thumb_url: string
  size: number
  width: number
  height: number
  mime: string
  kind: string
  owner_id: string
  owner_name: string
  original_name: string
  ip: string
  sha256: string
  ref_count: number
  legacy: boolean
  deleted: boolean
  deleted_at?: string
  created_at: string
}

// Storage counters. deleted_bytes is separate from live_bytes on purpose:
// withdrawn files still occupy the disk.
export interface UploadStats {
  live_count: number
  live_bytes: number
  deleted_count: number
  deleted_bytes: number
  orphan_count: number
  total_count: number
  total_bytes: number
}

export interface UploadPage {
  items: UploadFileInfo[]
  total: number
  page: number
  size: number
  stats: UploadStats
  user_used_bytes: number
  user_quota_bytes: number
  site_used_bytes: number
  site_quota_bytes: number
}

export interface UploadDeleteResult {
  name: string
  hard: boolean
  affected_refs: number
}

// --- editor form values ---
export interface EditorValues {
  title: string
  body: string
  status: PostStatus
  category_id?: number
  tags?: string[]
  pinned?: boolean
  cover?: string
}

// --- admin: categories / tags with counts ---
export interface CategoryWithCount {
  id: number
  name: string
  parent_id: number | null
  count: number
}

export interface TagWithCount {
  id: number
  name: string
  count: number
}

// --- prev/next navigation ---
export interface PostNav {
  prev: Post | null
  next: Post | null
}

// --- admin: user management ---
export interface UserWithDisabled {
  id: string
  username: string
  email: string
  role: Role
  disabled: boolean
  display_name: string
  last_login_ip: string
  last_login_at?: string
  created_at: string
}

// --- site config (spec 05) ---
export interface FriendLink {
  name: string
  url: string
}

// Public subset rendered site-wide (brand/title/footer); no secrets.
export interface SiteConfigPublic {
  site_name: string
  site_title: string
  site_logo: string
  site_favicon: string
  site_url: string
  site_description: string
  footer_text: string
  icp_no: string
  friend_links: string // raw JSON array string
  // Spec 24: posts per page on the public list (5/10/20/30, server-enforced).
  page_size: number
  // Spec 27: whether anonymous visitors may read the site.
  public_read: boolean
  // Spec 37 (experimental): when true, sign-up and password reset ask for a
  // code mailed to the address. Public because the forms must know whether to
  // render the field.
  email_verify_enabled: boolean
}

// Full config (admin only) including image-host token + registration flag.
export interface SiteConfigFull extends SiteConfigPublic {
  id: number
  img_host_mode: 'local' | 'smms'
  img_host_token: string
  allow_register: boolean
  // SMTP settings for security notifications; empty smtp_host disables them.
  smtp_host: string
  smtp_port: number
  smtp_user: string
  smtp_pass: string
  // Spec 34: daily snapshot settings (root-only surface).
  auto_backup: boolean
  backup_keep: number
  // Spec 36: image storage policy. The three ceilings are bytes; the two
  // switches decide what an ordinary author may do with their own pictures.
  // Sent in full on every save — the server treats a missing/zero ceiling as
  // "keep the stored one", so a stale form cannot disable the limits.
  upload_max_image: number
  upload_max_avatar: number
  user_quota_bytes: number
  site_quota_bytes: number
  allow_user_delete_upload: boolean
  allow_user_change_avatar: boolean
  allow_external_avatar: boolean
  // Spec 37 (experimental): the switch, plus the two timings. Admin-only on
  // the write side — a plain admin's save inherits all three.
  email_verify_enabled: boolean
  email_code_ttl_min: number
  email_code_cooldown: number
  updated_at: string
}

// --- login log (spec 14) ---
export interface LoginLog {
  id: number
  user_id: string
  identifier: string
  ip: string
  ua: string
  success: boolean
  created_at: string
}

// --- author page (spec 15) ---
export interface AuthorUser {
  username: string
  display_name: string
  bio: string
  avatar: string
}
export interface AuthorView {
  user: AuthorUser
  posts: Post[]
}

// --- dashboard (spec 06) ---
// Spec 33: one day of the view trend (gap-filled server-side).
export interface DayViews {
  day: string
  views: number
}

// Spec 33: a most-read published post.
export interface TopPost {
  id: string
  title: string
  slug: string
  view_count: number
}

export interface DashboardData {
  posts_total: number
  views_total: number
  pending_total: number
  view_trend: DayViews[]
  top_posts: TopPost[]
  recent_posts: Post[]
  recent_logins: LoginLog[]
}

// --- admin audit trail: who did what on the site ---
export interface ActionLog {
  id: number
  actor_id: string
  actor_name: string
  action: string
  target: string
  detail: string
  ip: string
  created_at: string
}

// --- personal center (spec 20): caller's own post metrics ---
export interface UserStats {
  posts_total: number
  views_total: number
}

// --- admin: backup ---
export interface BackupStats {
  users: number
  posts: number
  categories: number
  tags: number
  revisions: number
  upload_files: number
}

export interface BackupResult {
  filename: string
  size: number
  stats: BackupStats
}

// One VACUUM INTO snapshot on the server disk (root-only listing, P2-14).
export interface SnapshotInfo {
  name: string
  size: number
  mod_time: string
}

// --- admin: trash (soft-deleted posts, last 7 days) ---
export interface TrashPost {
  id: string
  title: string
  author_id: string
  author?: UserPublic
  deleted_at: string
  status: PostStatus
  pinned: boolean
  view_count: number
}

// --- reading experience settings ---
export type FontSize = 'small' | 'medium' | 'large' | 'xlarge'
