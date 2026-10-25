> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# lopezpress 项目全貌（接手快照 · 2026-10-13）

> 本文是 **代码地图 + 当前真实状态快照**，与 `HANDOVER.md`（接手清单）、`PROGRESS.md`（轮次记录）、`docs/ARCHITECTURE.md`（架构设计）、`docs/AUDIT_REPORT.md`（安全审计）互补。
> 作者于 2026-10-13 通读了全部后端 `internal/` 与前端 `web/src/` 源码，并实测 `go build ./...` + `go vet ./...` 全绿。

---

## 0. 怎么用这份文档

1. 第一次接手：先读 `README.md` → `HANDOVER.md` → 本文 → `docs/ARCHITECTURE.md`。
2. 要改代码：跳到 §7「改代码必读红线」和 §3 代码地图，定位文件再动手。
3. 要验证状态：参考 §10 实测命令。

---

## 1. 一句话定位

低配 VPS（1 vCPU / 1.5 GB / 5 GB 磁盘）自托管的全栈内容站点：**Go 单二进制（零 CGO）+ SQLite 单文件** 做后端，**Preact SPA** 做前端，前后端分离。定位"小而美个人博客 / 轻 CMS"，功能深度优先于广度。

**技术选型已定稿、不换框架。**

---

## 2. 技术栈（定稿）

| 层 | 选型 |
|---|---|
| 后端框架 | Go 1.22+ `net/http` 标准库 `ServeMux`（方法 + 通配路由，自实现） |
| ORM | GORM v1.25.x |
| 数据库 | 纯 Go SQLite 驱动 `github.com/glebarez/sqlite`（零 CGO，可交叉编译） |
| 鉴权 | `golang-jwt/jwt/v5` HS256（1h 有效期）+ `bcrypt` 密码哈希 |
| 限流 | `golang.org/x/time/rate` 令牌桶（内存态，单实例） |
| 图像处理 | `golang.org/x/image`（`draw` 缩放 + `webp` 解码），第 78 轮加入 |
| 邮件 | `net/smtp` 标准库自写极简客户端（25/587/465 + STARTTLS + AUTH LOGIN） |
| 前端框架 | Preact + Vite 5 + TypeScript |
| 前端路由 | `wouter`（薄封装见 `web/src/core/router.tsx`） |
| 状态/UI | Preact hooks + Context（auth / app / i18n）+ `core/useQuery.ts`（自写数据层），无 Redux |
| 数据请求 | `core/http.ts` 是**全库唯一 `fetch`**（`request`/`upload`/`download` 三基元），`core/api.ts` 是纯类型化 facade |
| i18n | 手写中(默认)/英双语，按域拆成 `core/locales/` 8 个文件（type + zh + en 各一份） |
| 测试 | 后端 `go test`（212 用例）+ 前端 `vitest`（43 用例，纯函数） |
| 部署 | `LopezPress.bat`（Win 一键）+ `deploy/`（systemd / nginx / docker compose） |

---

## 3. 目录结构与代码地图

```
lopezpress/
├─ cmd/server/main.go          # 入口：加载 .env → 校验 → 迁移 → 备份检查 → 启缩略图 worker → 装配路由 → :8080
├─ internal/
│  ├─ clientip/clientip.go     # 客户端真实 IP（TRUSTED_PROXIES 闸门，第 43 轮）
│  ├─ imghost/                 # 出站图床（SM.MS）客户端，service 层不再直接碰 HTTP（第 55 轮）
│  ├─ config/
│  │   ├─ config.go            # 配置解析 + InitDB + 种子数据 + schema 漂移检查
│  │   ├─ migrate.go           # 有序迁移注册表 0001–0013（R7 门禁：新列只能加在这）
│  │   ├─ uploads_backfill.go  # 历史上传文件登记为 legacy 行（幂等）
│  │   └─ recover.go           # root 密码离线恢复
│  ├─ models/                  # GORM 模型：models.go / all.go（AutoMigrate 唯一清单）/ site_config.go / uploads.go / email_codes.go / ids.go / presets.go
│  ├─ services/                # 业务规则层（36 文件，按域切分，单文件 ≤350 行）
│  │   ├─ posts*.go            # 增删改查 / 列表 / 编辑（含引用台账同步）
│  │   ├─ media*.go thumbs.go  # 图库：去重、配额、软删、台账、缩略图（spec 36）
│  │   ├─ email_codes.go auth_email.go mail*.go  # 验证码与邮件（spec 37）
│  │   ├─ site_config*.go      # 配置校验 + 策略合并（缺省即继承）
│  │   ├─ audit.go roles.go dto.go  # 审计单点 / 权限集中 / 全部 API 契约
│  │   └─ 其余：views / revisions / slug / access / batch / import_md / related / autobackup / backup / restore / trash / tags / taxonomy / nav / users / accounts*
│  ├─ handlers/                # 薄处理器：auth / auth_email / posts / accounts / admin / upload / categories / moderation / import / feed / query
│  ├─ middleware/              # Chain / Logger / CORS / Recover / Limiter / LoginLimiter / SecurityHeaders
│  └─ router/router.go         # 路由声明表 `var routes`（83 条，auth + roles 也在表里）
├─ web/src/
│  ├─ main.tsx                 # 路由表（wouter + Guarded / Readable / AdminGuarded / GuestOnly）
│  ├─ core/                    # http / useQuery / api / auth / i18n(+locales/) / md / router / site / theme / app / types / format / roles / taxonomy / tags
│  ├─ pages/                   # 26 个页面（见 §5），个人中心另有 pages/personal/ 4 个子组件
│  └─ ui/                      # 复用组件（Layout/AdminLayout/PostEditor/Sidebar/UserMenu/AvatarCropper/Modal/TableState/...）
├─ scripts/                    # arch_check.py / contract_check.py / doc_check.py / fakesmtp.py
├─ deploy/                     # VPS 部署（systemd / nginx / docker compose）
├─ LopezPress.bat              # Win 一键启动（幂等、自动开浏览器）
├─ MASTER.md                   # 母文档（入口，先读它）
├─ PROGRESS.md                 # 79 轮开发记录
├─ HANDOVER.md                 # 接手总览 + §7 陷阱（37 条）
├─ api.md                      # 端点索引 + 关键端点细节
├─ README.md                   # 快速上手
└─ docs/                       # ARCHITECTURE / specs(01–37) / decisions(0001–0006) / 审计 / 产品文档
```

**改代码黄金动线**：`cmd/server/main.go` → `internal/config` → `internal/router` → `internal/services/*`（业务规则都在这里）→ `handlers/auth.go` → `middleware/*` → `models` → `web/src/core` + `web/src/pages`。

---

## 4. 后端 API 端点

**83 条**，按域分组的完整表 + 关键端点请求/响应样例在 **`api.md`**（本文件不再重复，避免两处失真）。

改端点时必须同时改三处：`internal/router/router.go`（路由表加一行）→ `api.md`（文档）→ 前端 `web/src/core/api.ts`（调用），
然后跑 `python scripts/contract_check.py` 确认前后端没有漂移。
> 路由表现在是 `var routes = []route{{method, path, auth, roles, handler}}`，**权限就在这张表里**（`authPublic` / `authOptional` / `authRequired` + `roles`）。新增端点不要回到在 handler 里判角色的老写法（R4 门禁会拦）。

---

## 5. 前端路由与页面地图（26 页 + 4 种守卫）

### 5.1 守卫（定义在 `web/src/main.tsx`）
- `Guarded`：需登录，未登录跳转 `/login`
- `AdminGuarded`：需 **admin 或 root** 角色（第 25 轮起 root 也可进后台）
- `GuestOnly`：已登录用户反向跳走（登录/注册页用）
- `Readable`（第 46 轮）：读者路由专用——允许匿名，但站点关闭「公开阅读」时跳登录（`useSite().public_read`）

### 5.2 路由表
| 路径 | 页面 | 守卫 |
|---|---|---|
| `/` | `ListPage`（文章列表/搜索/分类筛选/分页 `?page=N`） | Readable |
| `/archive` | `ArchivePage`（按时间归档，服务端分页 50/页） | Readable |
| `/create` | `CreatePage`（新建，薄壳委托 `PostEditor`） | Guarded |
| `/edit/:id` | `EditPage`（编辑，薄壳委托 `PostEditor`） | Guarded |
| `/read/:id` | `ReadPage`（阅读页/目录/字号/代码复制/相关文章/面包屑） | Readable |
| `/p/:slug` | `SlugPage`（永久链接，内部转 `ReadPage`） | Readable |
| `/settings` | `SettingsPage`（头像裁剪 / 我的图库 + 配额 / 邮箱换绑 / 图床 token） | Guarded |
| `/me` | `PersonalCenterPage`（个人中心/我的文章/安全） | Guarded |
| `/u/:username` | `AuthorPage`（作者主页） | Readable |
| `/admin` | `AdminDashboardPage`（趋势图/待审/最多阅读/最近登录） | Admin |
| `/admin/posts` | `AdminPostsPage`（文章管理：筛选/勾选/批量/导入 Markdown） | Admin |
| `/admin/media` | `AdminMediaPage`（图库：缩略图/筛选/撤下/恢复/彻底删除/溯源） | Admin |
| `/admin/categories` | `AdminCategoriesPage` | Admin |
| `/admin/tags` | `AdminTagsPage` | Admin |
| `/admin/users` | `AdminUsersPage`（用户管理/批量/重置/登录历史；层级化） | Admin |
| `/admin/review` | `AdminReviewPage`（审核工作台：通过/打回+意见） | Admin |
| `/admin/banned` | `AdminBanVaultPage`（封禁库：发回/永久删除） | **Root** |
| `/admin/site` | `AdminSitePage`（站点配置/图床/友链/注册开关） | Admin |
| `/admin/backup` | `AdminBackupPage`（导出/导入/快照/每日自动备份状态） | Admin |
| `/admin/trash` | `AdminTrashPage`（回收站） | Admin |
| `/admin/audit` | `AdminAuditPage`（操作审计） | Admin |
| `/login` | `LoginPage` | GuestOnly |
| `/register` | `RegisterPage`（开启邮箱验证时多一行验证码） | GuestOnly |
| `/forgot-password` | `ForgotPasswordPage`（邮箱 + 码 + 新密码，spec 37） | GuestOnly |
| `/change-password` | `ChangePasswordPage`（首启强制改密） | Guarded |
| `*` | `NotFoundPage` | 公开 |

> `/admin/site` 的「邮箱验证」区块与 `/admin/banned` 一样是 **root 专属内容**（普通 admin 能进页面但改不了，保存时服务端继承库值）。

### 5.3 关键 UI 组件（`web/src/ui/`）
- `PostEditor.tsx`：最核心、最重的组件 —— 分屏/即时渲染/源码三模式、Markdown 工具栏、封面、自动保存草稿（localStorage）、可见性/置顶、TOC。
- `Layout.tsx` / `AdminLayout.tsx`：前台/后台布局骨架（后台导航按「内容/用户/系统」分组）。
- `Sidebar.tsx` / `UserMenu.tsx`：侧栏与用户菜单（含语言/主题切换）。
- `ImageUploader.tsx`：本地图库 + SM.MS 上传（插入后若 `deduped` 会提示"图库已有相同图片"）。
- `AvatarCropper.tsx`（第 77 轮）：pointer 拖动 + 滑块缩放（1–3x），从原图按视口比例采样输出 512×512 WebP，不留空角。
- `Modal.tsx` / `TableState.tsx` / `Toast.tsx` / `Footer.tsx`：轻量复用件。
- `Pagination.tsx` / `RevisionList.tsx`：首页分页器、编辑页历史版本面板。

### 5.4 i18n
`web/src/core/locales/` 按域拆 8 个文件（`nav` / `auth` / `center` / `reader` / `editor` / `admin` / `site` / `common`），每个文件导出一份 type + zh + en，`core/i18n.tsx` 只负责交叉组合与 `t('a.b.c', vars)` 取值。localStorage 持久化，`navigator.language` 兜底中文；`t()` 未命中时 dev 打 warn、prod 回退英文。新增文案必须在同一文件里补齐 zh/en 两份。

---

## 6. 数据模型（GORM 定义于 `internal/models/`，AutoMigrate 清单见 `all.go`）

- **User**：id / username / email / password(bcrypt) / **role(root|admin|user)**（root 全库唯一、不可经 API 创建） / disabled / must_change_pwd（首启强制改密） / token_version（改密/重置即失效旧 JWT） / display_name / bio / avatar / created_at / updated_at
- **Post**：id / title / slug / body(MD) / excerpt / cover / category_id / **status(published|private|pending|rejected|banned)** / **review_note / reviewed_by / reviewed_at**（审核留痕） / pinned / views / author_id / author_name / created_at / updated_at / deleted_at（软删，进回收站）
- **UploadFile**（第 77 轮）：id / name / **live_key（唯一索引）** / owner_id / size / width / height / mime / sha256 / original_name / ip / kind(post|avatar) / ref_count / legacy / **thumb_name / thumb_status** / deleted_at + deleted_by
  - `live_key` 三态：活跃=`sha256`、已撤下=`d:<id>`、历史文件=`legacy:<name>`。**去重查询只按 live_key，所以撤下的图永不复活给他人。**
  - `deleted_at` 是普通 `*time.Time`（**不是 `gorm.DeletedAt`**）：软删的文件必须继续服务已发布文章。
- **UploadRef**（第 77 轮）：upload_id / post_id / kind / user_id，唯一索引 `idx_upload_refs` —— 文章引用了哪些图、图被哪些文章引用，双向可查。
- **EmailCode**（第 79 轮）：email / purpose(register|reset_password|change_email) / **code_hash(HMAC)** / user_id / expires_at / used_at / attempts / ip。一次性、5 次错误作废、绑用途。
- **Category** / **Tag**：与 Post 多对一 / 多对多
- **LoginLog**：登录历史（ip / 时间 / 成功失败）
- **SiteConfig**：站点名/标题/logo/favicon/footer/icp/友链/图床模式+token/注册开关/**存储策略 7 项**（`upload_max_image`、`upload_max_avatar`、`user_quota_bytes`、`site_quota_bytes`、`allow_user_delete_upload`、`allow_user_change_avatar`、`allow_external_avatar`）/**邮箱验证 3 项**（`email_verify_enabled`、`email_code_ttl_min`、`email_code_cooldown`）
- **BackupSnapshot**：备份快照（用于 `restore-snapshot`）
- **ActionLog**：操作审计（actor / action / target / detail / ip）—— **唯一写者是 `services/audit.go` 的 `s.audit`**（R10 门禁）

> 可见性判定**全部在 service 层**；遗留 `draft`/`scheduled` 值一律白名单兜底为 `published`（第 22 轮定稿"公开/私密"两态，已移除 draft/scheduled）。

---

## 7. 改代码必读：10 条红线（来自 HANDOVER §7，已实测确认）

1. **AutoMigrate 崩溃（最高危）** — `glebarez/sqlite` 在"误判需重建表"时走 copy-rename，`INSERT` 漏 NOT NULL 列 → 启动报 `NOT NULL constraint failed: posts__temp.title`。
   - 当前修复：`runManualMigrations`（注册表在 `config/migrate.go`）先跑 → `AutoMigrate` 仅给**不存在的表**建表。**改 config 的红线：不要回退成无条件 `AutoMigrate`；新列 = 模型字段 + 注册表加一条（`ensureColumns`），不要另写 `add*Column` 克隆（R7 门禁会拦）。**
2. **登录限流器账号桶 key** — `email` 优先、回退 `username`。前端把用户名/邮箱都放 `email` 字段。
3. **SM.MS 服务端持有** — 无 token 且模式 smms → 返回 **503**（不下落本地）；前端不再把 token 存 localStorage。
4. **匿名读接口用 `WithOptionalAuth` 不是 `WithAuth`** — 坏/过期 token 仍 401，但不套强鉴权。
5. **可见性判定全在 service 层**（`canSeePost` 是唯一权威）：published 全见 / **private 仅作者**（第 73 轮收紧，admin/root 也读不到）/ pending+rejected 作者 + 审核者 / **banned 仅 root**。列表过滤与单篇判定必须用同一份名单，否则"列表有、点进去 404"。
6. **root 不可经 API 创建** — `UpdateUser`/`CreateUser` 遇 `RoleRoot` 直接 403；root 仅由首启 `seedDemoData` 与迁移 `0003_root_migration` 产生。测试 fixture 造 root 必须直改 DB。
7. **前端 token 自愈** — 启动 `GET /api/auth/me` 复核，无自动刷新，1h 过期需重登（设计如此非 bug）。
8. **上传的三步顺序：去重 → 配额 → 写盘** — 配额在写盘前（否则留半截文件），去重在配额前（复用不占额度）。**测试配额必须每次换不同字节**，否则会先命中去重。
9. **"缺省=关闭"的开关与"0=不限制"的数值，保存时一律继承** — handler 从原始 map 判断 key 是否出现；否则旧表单会把磁盘保护/头像开关静默关掉。适用：`allow_register`、`auto_backup`、`smtp_pass`、`allow_*`、`email_verify_*`。
10. **前端只有一个 `fetch`、后端只有一个 DTO 入口** — 页面/组件禁用裸 `fetch`（R8 门禁），一律走 `core/api.ts` → `core/http.ts`；新增 API 契约 struct 只进 `services/dto.go`（R3 门禁），否则两处失真。

---

## 8. 已落地的安全加固

**第一批（2026-09-24，10 项）**：`JWT_SECRET` 启动强制校验（prod 弱密钥 `log.Fatalf`）、首启随机 admin 密码 + 强制改密、登录失败锁定（5 次/15min + `Retry-After`）、CORS 白名单（`*` 与 credentials 不同时）、备份 `dry-run` + 快照回滚、SM.MS 服务端持有、清理 `ScheduledAt` 死代码、补 service 单测。

**第二批（第 72 轮，外部渗透报告）**：作者敏感字段裁剪（`SanitizeAuthor`，匿名列表不再泄露全站作者 email/IP）、安全响应头（nosniff / X-Frame-Options / Referrer-Policy / CSP / HSTS）、上传 magic-bytes 校验、改密 bump `token_version` 并回发新 token、管理员重置密码踢会话、登录支持 username、dev 补 `/uploads` 代理、nginx 补 4 个 location + 静态 location 重复声明安全头。

**第三批（第 75 轮，root/admin 权限面）**：备份导入三道闸（结构闸 / `dry_run` / 确认词）、备份导出下载收归 root、`site-config` 对非 root 屏蔽 SMTP 口令、`friend_links` 只允许 http(s)、`pinned` 仅 admin/root 可设、用户名 ASCII 化 + 大小写不敏感查重、上传追加真实解码与像素上限、恢复备份全站 token 失效、非 root 不可读 root 登录记录。

**第四批（第 77–79 轮，媒体与身份）**：上传文件名改不可枚举 ID + 元数据溯源 + 三层配额；图片撤下即轮换 `live_key`（永不复活给他人）；头像只接受本站图库（外链需站长显式开启）；邮箱验证码只存 HMAC、5 次作废、一次性、绑用途、双频限流；**开着验证时 `PUT /api/auth/me` 改邮箱 403**（堵住绕过验证码的旁路）。

验证基线见 §10。

---

## 9. 已知约束 / 设计取舍（不是 bug）

- ⚠️ **FTS5 不可用** → 搜索走 `LIKE '%...%'`，千级文章内可接受，不换 cgo 驱动。
- 草稿仅 localStorage 30 天 TTL，不跨设备。
- JWT 无状态，吊销靠 `token_version`（改密/重置/恢复备份即全失效）。
- 限流为内存态，单实例有效；多实例需外接存储（当前单机部署不触发）。
- 缩略图只有 4 态且 `failed` 不自动重试（解码失败的图不值得重试）。
- `upload_refs` 只在通过 API 保存/删除文章时同步；直改库会产生孤儿引用。
- 邮箱验证码的"同 IP 每小时 10 封"是内存固定窗口，重启清零。

---

## 10. 当前验证状态（2026-10-24 第 79 轮实测）

| 门禁 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` | ✅ |
| `go test -count=1 ./internal/...` | ✅ 212 个 `Test*` / 48 个文件 |
| `tsc --noEmit` | ✅ |
| `vitest run` | ✅ 5 文件 43 用例 |
| `vite build` | ✅ JS 261.38 kB（gzip 81.28）、CSS 66.75 kB（gzip 12.47） |
| `scripts/arch_check.py` | ✅ 无规则回升（R1/R3/R4/R5/R7/R8/R9/R10 全 0） |
| `scripts/contract_check.py` | ✅ 83 条路由 / 76 个前端调用 / 方法不匹配 0 |
| `scripts/doc_check.py` | ✅ 73 份文档、23/23 顶层被点名、73/73 横幅 |
| 真机 | 图库 31/31（去重/配额/软删/权限/台账）、缩略图 23/23、邮箱验证 28/28（隔离实例 + DB 副本 + 本地假 SMTP） |

未验证：**Docker 构建**（开发机无 Docker）、**真实 VPS 端到端**、**真实 SMTP 收信**。

## 11. 待拍板（HANDOVER §9，同步收敛）

1. **普通用户的「私密」入口** — 编辑器可见但 `CreatePost` 强制 pending；要么隐藏，要么明确提示"先过审核"。**等用户拍板。**
2. 生产部署验证 — `deploy/docker/` 已就绪但未 docker build、未上真实 VPS。
3. 邮箱验证上线前的真 SMTP 验证 — 目前只有本地假服务器证据。
4. PWA / Service Worker 离线化 — 列入方向未实现。

> 功能类待办（评论、回收站单条永久删除等）见 `docs/TODO.md`——那才是唯一权威待办清单。

---

## 12. 推荐接手动线（审计优先级）

1. `cmd/server/main.go` → `internal/config/config.go` + `config/migrate.go`（理解启动顺序与迁移注册表）
2. `internal/router/router.go`（**路由表 = 端点清单 + 权限表**）→ `internal/services/`（业务规则核心，按需读域文件）+ `services/roles.go`（权限 helper）+ `services/dto.go`（全部契约）
3. `handlers/auth.go`（鉴权包裹语义）→ `middleware/*`（限流/失败锁定/安全头）
4. `web/src/main.tsx`（路由）→ `web/src/core/{http,useQuery,api}.ts`（数据层三件套）→ `web/src/ui/PostEditor.tsx`（最重组件）
5. 动手前跑一遍 `HANDOVER.md` §11 自测清单 + §10 三连命令回归。

---

*本文为 2026-10-24 更新（第 79 轮后）。路由与页面以 `internal/router/router.go`、`web/src/main.tsx` 为准；两者不一致时先改本文。*
