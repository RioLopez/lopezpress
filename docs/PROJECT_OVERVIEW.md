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
| 前端框架 | Preact + Vite 5 + TypeScript |
| 前端路由 | `wouter`（薄封装见 `web/src/core/router.tsx`） |
| 状态/UI | Preact hooks + Context（auth / app / i18n），无 Redux |
| i18n | 手写中(默认)/英双语（`web/src/core/i18n.tsx`，约 1200 行字典） |
| 部署 | `LopezPress.bat`（Win 一键）+ `deploy/`（systemd / nohup） |

---

## 3. 目录结构与代码地图

```
lopezpress/
├─ cmd/server/main.go          # 入口：加载 .env → 迁移 → 装配 handler → 启动 :8080
├─ internal/
│  ├─ config/config.go         # 配置解析 + 数据库迁移(runManualMigrations) + 红线所在
│  ├─ models/models.go         # 全部 GORM 模型（User/Post/Category/Tag/LoginLog/SiteConfig/...）
│  ├─ services/services.go     # 业务规则核心（ListPostsPage/ListArchive/GetAuthor/GetPostPublic…）
│  ├─ services/accounts.go     # 账号业务逻辑（注册/登录/改密/禁用/重置）
│  ├─ handlers/                # HTTP 适配层（auth/posts/admin/accounts/categories/upload）
│  │   ├─ auth.go              # WithAuth / WithOptionalAuth / Me / ChangePassword
│  │   ├─ posts.go             # 文章 CRUD + 公开读 + 归档 + 上下篇
│  │   ├─ admin.go             # 仪表盘 / 站点配置 / 备份 / 回收站
│  │   ├─ accounts.go          # 用户管理 / 登录历史
│  │   ├─ categories.go        # 分类 + 标签
│  │   └─ upload.go            # 本地 + SM.MS 图床
│  ├─ middleware/              # login_limiter.go（登录失败锁定）/ limiter.go（通用）/ middleware.go
│  └─ router/router.go         # 全部 40 个端点注册（见 §4）
├─ web/
│  └─ src/
│     ├─ main.tsx              # 路由表（wouter Switch + Guarded/AdminGuarded/GuestOnly）
│     ├─ core/                 # api / auth / i18n / md / router / site / theme / app / types / debounce
│     ├─ pages/                # 19 个页面（见 §5）
│     └─ ui/                   # 8 个复用组件（Layout/AdminLayout/PostEditor/Sidebar/UserMenu/...）
├─ deploy/                     # VPS 部署脚本（systemd / nohup）
├─ LopezPress.bat                 # Win 一键启动（幂等、自动开浏览器）
├─ PROGRESS.md                 # 22 轮开发记录
├─ HANDOVER.md                 # 接手总览（必读）
├─ README.md                   # 快速上手
└─ docs/                       # ARCHITECTURE / AUDIT_REPORT / specs(20) / decisions(0001-0004)
```

**改代码黄金动线**：`cmd/server/main.go` → `internal/config` → `internal/router` → `internal/services/*`（业务规则都在这里）→ `handlers/auth.go` → `middleware/*` → `models` → `web/src/core` + `web/src/pages`。

---

## 4. 后端 API 端点速查（40 条）

鉴权标注：`公`=匿名可访问，`鉴`=需登录，`管`=需管理员（实际由 `WithAuth`/`WithAdmin` 包裹）。

### 4.1 公开 / 鉴权相关
| 方法 | 路径 | 鉴权 | Handler |
|---|---|---|---|
| GET | `/health` | 公 | 健康检查 |
| GET | `/` | 公 | 前端静态入口 |
| POST | `/api/auth/register` | 公* | `Register`（受 `RegisterOpen` 开关限制） |
| GET | `/api/auth/register/open` | 公 | `RegistrationOpen`（是否开放注册） |
| POST | `/api/auth/login` | 公 | `Login`（套 `loginLimiter` 失败锁定） |
| GET | `/api/authors/{username}` | 公 | `GetAuthor` |
| GET | `/api/site-config` | 公 | `GetSiteConfig`（公开站点信息） |
| GET | `/api/posts/{id}/public` | 公 | `GetPostPublic`（私密文章返回 404） |
| GET | `/api/auth/me` | 鉴 | `Me`（启动 token 复核） |
| PUT | `/api/auth/me` | 鉴 | `UpdateMe`（资料/头像/简介） |
| POST | `/api/auth/logout` | 鉴 | `Logout` |
| POST | `/api/auth/change-password` | 鉴 | `ChangePassword` |
| GET | `/api/me/stats` | 鉴 | `MeStats`（个人中心统计） |

### 4.2 内容（文章 / 分类 / 标签）
| 方法 | 路径 | 鉴权 | Handler |
|---|---|---|---|
| GET | `/api/posts` | 公(opt) | `ListPosts`（分页/搜索/分类/标签/可见性） |
| GET | `/api/posts/recent` | 公(opt) | `RecentPosts` |
| GET | `/api/posts/archive` | 公(opt) | `ListArchive` |
| GET | `/api/posts/{id}` | 公(opt) | `GetPost` |
| GET | `/api/posts/{id}/nav` | 公(opt) | `GetPostPrevNext` |
| GET | `/api/categories` | 公(opt) | `ListCategories` |
| GET | `/api/tags` | 公(opt) | `ListTags` |
| POST | `/api/posts` | 鉴 | `CreatePost` |
| PUT | `/api/posts/{id}` | 鉴 | `UpdatePost` |
| DELETE | `/api/posts/{id}` | 鉴 | `DeletePost`（移入回收站） |
| GET | `/api/categories/with-count` | 鉴 | `ListCategoriesWithCount` |
| POST | `/api/categories` | 鉴 | `CreateCategory` |
| PUT | `/api/categories/{id}` | 鉴 | `UpdateCategory` |
| DELETE | `/api/categories/{id}` | 鉴 | `DeleteCategory` |
| GET | `/api/tags/with-count` | 鉴 | `ListTagsWithCount` |
| DELETE | `/api/tags/{id}` | 鉴 | `DeleteTag` |

### 4.3 上传 / 用户管理 / 站点 / 备份 / 回收站
| 方法 | 路径 | 鉴权 | Handler |
|---|---|---|---|
| POST | `/api/upload` | 鉴 | `UploadImage`（本地 2MB / SM.MS） |
| GET | `/api/users` | 鉴 | `ListUsers` |
| POST | `/api/users` | 鉴 | `CreateUser` |
| PUT | `/api/users/{id}` | 鉴 | `UpdateUser` |
| PUT | `/api/users/{id}/disabled` | 鉴 | `SetUserDisabled` |
| POST | `/api/users/{id}/reset-password` | 鉴 | `ResetPassword` |
| DELETE | `/api/users/{id}` | 鉴 | `DeleteUser` |
| GET | `/api/users/{id}/logins` | 鉴 | `ListUserLogins` |
| GET | `/api/admin/site-config` | 鉴 | `GetAdminSiteConfig` |
| PUT | `/api/admin/site-config` | 鉴 | `UpdateSiteConfig` |
| GET | `/api/admin/dashboard` | 鉴 | `GetDashboard` |
| POST | `/api/backup` | 鉴 | `ExportBackup`（JSON） |
| POST | `/api/backup/import` | 鉴 | `ImportBackup` |
| POST | `/api/backup/restore-snapshot` | 鉴 | `RestoreSnapshot` |
| GET | `/api/backup/download` | 鉴 | `DownloadBackup` |
| GET | `/api/trash` | 鉴 | `ListTrash`（7 天内） |
| POST | `/api/trash/empty` | 鉴 | `EmptyTrash` |
| POST | `/api/trash/{id}/restore` | 鉴 | `RestoreTrash` |
| GET | `/api/admin/pending` | 鉴 | `ListPending`（admin 见 user 发文 / root 全见） |
| POST | `/api/admin/review/{id}` | 鉴 | `ReviewPost`（体 `{"decision":...}`） |
| POST | `/api/admin/ban/{id}` | 鉴 | `BanPost`（**root only**） |
| GET | `/api/admin/banned` | 鉴 | `ListBanned`（**root only**） |
| POST | `/api/admin/banned/{id}/send-back` | 鉴 | `SendBackPost`（**root only**） |
| DELETE | `/api/admin/banned/{id}` | 鉴 | `DeleteBannedPermanently`（**root only**，硬删） |
| GET | `/uploads/` | 公 | `ServeUploads`（本地图片静态服务） |

> 注：管理员接口实际由 `WithAdmin`/`WithRoot` 包裹（`GetDashboard`/`site-config`/`users/*`/`backup/*`/`trash/*`/审核封禁系列）。全量路由以 `internal/router/router.go` 为准。

---

## 5. 前端路由与页面地图（19 页 + 3 种守卫）

### 5.1 守卫（定义在 `web/src/main.tsx`）
- `Guarded`：需登录，未登录跳转 `/login`
- `AdminGuarded`：需 **admin 或 root** 角色（第 25 轮起 root 也可进后台）
- `GuestOnly`：已登录用户反向跳走（登录/注册页用）

### 5.2 路由表
| 路径 | 页面 | 守卫 |
|---|---|---|
| `/` | `ListPage`（文章列表/搜索/分类筛选） | Guarded |
| `/archive` | `ArchivePage`（按时间归档） | Guarded |
| `/create` | `CreatePage`（新建，薄壳委托 `PostEditor`） | Guarded |
| `/edit/:id` | `EditPage`（编辑，薄壳委托 `PostEditor`） | Guarded |
| `/read/:id` | `ReadPage`（阅读页/目录/字号/代码复制） | 公开 |
| `/settings` | `SettingsPage`（图床 token 等） | Guarded |
| `/me` | `PersonalCenterPage`（个人中心/我的文章/安全） | Guarded |
| `/u/:username` | `AuthorPage`（作者主页） | 公开 |
| `/admin` | `AdminDashboardPage`（仪表盘） | Admin |
| `/admin/categories` | `AdminCategoriesPage` | Admin |
| `/admin/tags` | `AdminTagsPage` | Admin |
| `/admin/users` | `AdminUsersPage`（用户管理/批量/重置/登录历史；层级化） | Admin |
| `/admin/review` | `AdminReviewPage`（审核工作台：通过/打回+意见） | Admin |
| `/admin/banned` | `AdminBanVaultPage`（封禁库：发回/永久删除） | **Root** |
| `/admin/site` | `AdminSitePage`（站点配置/图床/友链/注册开关） | Admin |
| `/admin/backup` | `AdminBackupPage`（导出/导入/快照） | Admin |
| `/admin/trash` | `AdminTrashPage`（回收站） | Admin |
| `/admin/audit` | `AdminAuditPage`（操作审计） | Admin |
| `/login` | `LoginPage` | GuestOnly |
| `/register` | `RegisterPage` | GuestOnly |
| `/change-password` | `ChangePasswordPage`（首启强制改密） | Guarded |
| `*` | `NotFoundPage` | 公开 |

### 5.3 关键 UI 组件（`web/src/ui/`）
- `PostEditor.tsx`（642 行）：最核心、最重的组件 —— 分屏/即时渲染/源码三模式、Markdown 工具栏、封面、自动保存草稿（localStorage）、可见性/置顶、TOC。
- `Layout.tsx` / `AdminLayout.tsx`：前台/后台布局骨架。
- `Sidebar.tsx` / `UserMenu.tsx`：侧栏与用户菜单（含语言/主题切换）。
- `ImageUploader.tsx`：本地 + SM.MS 上传。
- `Toast.tsx` / `Footer.tsx`：轻量复用件。

### 5.4 i18n
`web/src/core/i18n.tsx` 手写 `zh`/`en` 双字典（约 1200 行），`t('a.b.c', vars)` 点路径取值，localStorage 持久化，`navigator.language` 兜底默认中文。新增文案务必在此同步两语言，否则回退为 key 字符串。

---

## 6. 数据模型（核心表，GORM 定义于 `internal/models/models.go`）

- **User**：id / username / email / password(bcrypt) / **role(root|admin|user)**（root 全库唯一、不可经 API 创建） / disabled / must_change_pwd（首启强制改密） / display_name / bio / avatar / created_at / updated_at
- **Post**：id / title / body(MD) / excerpt / cover / category_id / **status(published|private|pending|rejected|banned)**（banned 仅 root 可见） / **review_note / reviewed_by / reviewed_at**（审核留痕） / pinned / views / author_id / author_name / created_at / updated_at / deleted_at（软删，进回收站）
- **Category** / **Tag**：与 Post 多对一 / 多对多
- **LoginLog**：登录历史（ip / 时间 / 成功失败）
- **SiteConfig**：站点名/标题/logo/favicon/footer/icp/友链/图床模式+token/注册开关
- **BackupSnapshot**：备份快照（用于 `restore-snapshot`）
- **ActionLog**：操作审计（actor / action / target / detail / ip）

> 可见性判定**全部在 service 层**；遗留 `draft`/`scheduled` 值一律白名单兜底为 `published`（第 22 轮定稿"公开/私密"两态，已移除 draft/scheduled）。

---

## 7. 改代码必读：7 条红线（来自 HANDOVER §7，已实测确认）

1. **AutoMigrate 崩溃（最高危）** — `glebarez/sqlite` 在"误判需重建表"时走 copy-rename，`INSERT` 漏 NOT NULL 列 → 启动报 `NOT NULL constraint failed: posts__temp.title`。
   - 当前修复：`runManualMigrations` 先跑 → `AutoMigrate` 仅给**不存在的表**建表；已存在表由手动迁移管理。**改 `config.go` 的红线：不要回退成无条件 `AutoMigrate`；手动迁移顺序不能早于 `schema_migrations` 建表；`dropScheduledAt` 的 `posts_new` 列清单必须与 `Post` 模型同步。**
2. **登录限流器账号桶 key** — 现为 `email` 优先、回退 `username`（2026-10-14 加固）。前端把用户名/邮箱都放 `email` 字段，账号级失败锁定当前即生效；加固为防止未来独立 `username` 字段登录导致 key 取空（详见 HANDOVER §7.2）。
3. **SM.MS 服务端持有** — 无 token 且模式 smms → 返回 **503**（不下落本地）；前端不再把 token 存 localStorage。
4. **匿名读接口用 `WithOptionalAuth` 不是 `WithAuth`** — 坏/过期 token 仍 401，但不套强鉴权，否则访客 401→重试 429。
5. **可见性判定全在 service 层**（`canSeePost` 是五档状态唯一权威：published 全见 / private+pending+rejected 作者+admin+root / **banned 仅 root**），不在 handler/router。
6. **root 不可经 API 创建（2026-10-15 第 25 轮）** — `UpdateUser`/`CreateUser` 遇 `RoleRoot` 直接 403；root 仅由首启 `seedDemoData` 与迁移 `0003_root_migration` 产生。测试 fixture 造 root/admin 必须直改 DB（`handlers` 包 `setup(t)` 返回 `(*services.Service, *gorm.DB)` 正是为此）。权限 helper 集中在 `services/roles.go`。
7. **前端 token 自愈** — 启动 `GET /api/auth/me` 复核，无自动刷新，1h 过期需重登（设计如此非 bug）。

---

## 8. 已落地的安全加固（10 项，2026-09-24 完成并通过）

`JWT_SECRET` 启动强制校验（prod 弱密钥 `log.Fatalf`）、首启随机 admin 密码 + 强制改密、登录失败锁定（5 次/15min + `Retry-After`）、CORS 白名单（`*` 与 credentials 不同时）、备份 `dry-run` + 快照回滚、SM.MS 服务端持有、清理 `ScheduledAt` 死代码、补 service 单测（14 个 `_test.go` 全绿）等。

验证基线：`go build` / `go vet` / `go test ./...` 全绿 + `npx tsc --noEmit` + `npm run build`。

---

## 9. 已知约束 / 设计取舍（不是 bug）

- ⚠️ **FTS5 不可用** → 搜索走 `LIKE '%...%'`，千级文章内可接受，不换 cgo 驱动。
- 草稿仅 localStorage 30 天 TTL，不跨设备。
- JWT 无状态，无吊销列表（logout 仅客户端信号）。
- 限流为内存态，单实例有效；多实例需外接存储（当前单机部署不触发）。

---

## 10. 当前验证状态（2026-10-15 第 25 轮实测）

| 检查 | 命令 | 结果 |
|---|---|---|
| 后端编译 | `go build ./...` | ✅ 通过 |
| 后端静态检查 | `go vet ./...` | ✅ 通过 |
| 后端测试 | `go test ./...` | ✅ 全绿（本轮新增 `moderation_test.go` 7 个用例） |
| 前端类型检查 | `npx tsc --noEmit` | ✅ 通过 |
| 前端构建 | `npm run build` | ✅ 60 模块；JS 206.28 kB / gzip 63.42 kB，CSS 57.29 kB / gzip 10.76 kB |

---

## 11. 待拍板（HANDOVER §9，同步收敛）

1. `internal/db` 是否从 `config.go` 拆出独立包 — **倾向暂不拆**（低配机"揉一起"更直观），真上 PostgreSQL 再拆。
2. 生产部署验证 — `deploy/docker/` 已就绪但未 docker build、未上真实 VPS；需一台 1 vCPU VPS 或开发机 Docker 验证。
3. PWA / Service Worker 离线化 — 列入方向未实现。

> 功能类待办（评论等增强方向）见 `docs/TODO.md`——那才是唯一权威待办清单。

---

## 12. 推荐接手动线（审计优先级）

1. `cmd/server/main.go` → `internal/config/config.go`（理解启动与迁移顺序）
2. `internal/router/router.go` → `internal/services/services.go`（业务规则核心）+ `internal/services/roles.go`（第 25 轮权限 helper）
3. `handlers/auth.go`（鉴权包裹语义）→ `middleware/*`（限流/失败锁定）
4. `web/src/main.tsx`（路由）→ `web/src/core/*` → `web/src/ui/PostEditor.tsx`（最重组件）
5. 动手前跑一遍 `HANDOVER.md` §11 自测清单 + §10 三连命令回归。

---

*本文为 2026-10-15 更新（第 25 轮后），随代码演进请以 `internal/router/router.go` 与 `web/src/main.tsx` 的实时路由为准。*
