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
├─ cmd/server/main.go          # 入口：加载 .env → 迁移 → slug 回填 → 自动备份检查 → 装配 handler → :8080
├─ internal/
│  ├─ clientip/clientip.go     # 客户端真实 IP（TRUSTED_PROXIES 闸门，第 43 轮）
│  ├─ config/config.go         # 配置解析 + 数据库迁移(runManualMigrations) + schema 漂移检查 + 红线所在
│  ├─ models/models.go         # GORM 模型（User/Post/Category/Tag/PostView/PostRevision/SiteConfig/...）
│  ├─ models/presets.go        # 预设两层分类树的单一事实源
│  ├─ services/                # 业务规则层（handler 只做 HTTP 适配）
│  │   ├─ services.go          # 核心：列表/分页/归档/可见性/备份/快照
│  │   ├─ accounts.go          # 账号、站点配置、仪表盘聚合（spec 33）
│  │   ├─ views.go             # 浏览量指纹去重（spec 23）
│  │   ├─ revisions.go         # 修订历史与回滚（spec 31）
│  │   ├─ slug.go              # slug 生成/唯一化/历史回填（spec 29）
│  │   ├─ access.go            # public_read + 分类树（spec 27）
│  │   ├─ batch.go             # 批量操作，逐条复用单条逻辑（spec 26）
│  │   ├─ import_md.go         # Markdown 批量导入（spec 32）
│  │   ├─ related.go           # 相关文章（spec 35）
│  │   ├─ autobackup.go        # 每日快照（spec 34）
│  │   ├─ roles.go             # 权限集中判定
│  │   ├─ mail.go / fts.go / recover.go
│  ├─ handlers/                # auth / posts / admin / accounts / categories / upload / feed / import
│  ├─ middleware/              # login_limiter / limiter / middleware
│  └─ router/router.go         # 全部 76 条路由注册（速查见 `api.md`）
├─ web/src/
│  ├─ main.tsx                 # 路由表（wouter + Guarded / Readable / AdminGuarded / GuestOnly）
│  ├─ core/                    # api / auth / i18n / md / router / site / theme / app / types / format / debounce
│  ├─ pages/                   # 24 个页面（见 §5）
│  └─ ui/                      # 11 个复用组件（Layout/AdminLayout/PostEditor/Sidebar/UserMenu/Pagination/...）
├─ deploy/                     # VPS 部署（systemd / nginx / docker compose）
├─ LopezPress.bat              # Win 一键启动（幂等、自动开浏览器）
├─ MASTER.md                   # 母文档（入口，先读它）
├─ PROGRESS.md                 # 52 轮开发记录
├─ HANDOVER.md                 # 接手总览 + §7 陷阱
├─ api.md                      # 端点索引 + 关键端点细节
├─ README.md                   # 快速上手
└─ docs/                       # ARCHITECTURE / specs(01–35) / decisions(0001–0004) / 审计 / 产品文档
```

**改代码黄金动线**：`cmd/server/main.go` → `internal/config` → `internal/router` → `internal/services/*`（业务规则都在这里）→ `handlers/auth.go` → `middleware/*` → `models` → `web/src/core` + `web/src/pages`。

---

## 4. 后端 API 端点

**76 条**，按域分组的完整表 + 关键端点请求/响应样例在 **`api.md`**（本文件不再重复，避免两处失真）。

改端点时必须同时改三处：`internal/router/router.go`（注册）→ `api.md`（文档）→ 前端 `web/src/core/api.ts`（调用），
然后跑 `python scripts/contract_check.py` 确认前后端没有漂移。

---

## 5. 前端路由与页面地图（24 页 + 4 种守卫）

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
| `/settings` | `SettingsPage`（图床 token 等） | Guarded |
| `/me` | `PersonalCenterPage`（个人中心/我的文章/安全） | Guarded |
| `/u/:username` | `AuthorPage`（作者主页） | Readable |
| `/admin` | `AdminDashboardPage`（趋势图/待审/最多阅读/最近登录） | Admin |
| `/admin/posts` | `AdminPostsPage`（文章管理：筛选/勾选/批量/导入 Markdown） | Admin |
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
| `/register` | `RegisterPage` | GuestOnly |
| `/change-password` | `ChangePasswordPage`（首启强制改密） | Guarded |
| `*` | `NotFoundPage` | 公开 |

### 5.3 关键 UI 组件（`web/src/ui/`）
- `PostEditor.tsx`（642 行）：最核心、最重的组件 —— 分屏/即时渲染/源码三模式、Markdown 工具栏、封面、自动保存草稿（localStorage）、可见性/置顶、TOC。
- `Layout.tsx` / `AdminLayout.tsx`：前台/后台布局骨架。
- `Sidebar.tsx` / `UserMenu.tsx`：侧栏与用户菜单（含语言/主题切换）。
- `ImageUploader.tsx`：本地 + SM.MS 上传。
- `Pagination.tsx`（第 43 轮）：首页 / 上页 / 页码窗 / 下页 / 末页 / 跳转。
- `RevisionList.tsx`（第 51 轮）：编辑页底部的历史版本面板。
- `Toast.tsx` / `Footer.tsx` / `TableState.tsx`：轻量复用件。

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

## 10. 当前验证状态（2026-10-23 第 52 轮实测）

| 门禁 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` | ✅ |
| `go test -count=1 ./...` | ✅ 168 个 `Test*` / 39 个文件 |
| `tsc --noEmit` | ✅ |
| `vite build` | ✅ JS 233.35 kB（gzip 71.77）、CSS 61.06 kB（gzip 11.45），无新增依赖 |
| `scripts/contract_check.py` | ✅ 76 条路由 / 69 个前端调用 / 方法不匹配 0 |
| `scripts/doc_check.py` | ✅ 70 份文档、22/22 顶层被点名、70/70 横幅 |
| 真机 | 列表分页、浏览量去重、归档分页、slug 解析、相关文章、自动备份、公开读开关全部实测通过 |

未验证：**Docker 构建**（开发机无 Docker）、**真实 VPS 端到端**。

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

*本文为 2026-10-23 更新（第 52 轮后）。路由与页面以 `internal/router/router.go`、`web/src/main.tsx` 为准；两者不一致时先改本文。*
