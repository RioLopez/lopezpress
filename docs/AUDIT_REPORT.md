# Echo Content API — 项目汇报与审计指引

> 版本：v1.1（2026-10-15 更新：标注第 23–25 轮进展；安全加固结论仍有效）
> 受众：负责审计本项目"逻辑 / 代码 / 安全"全貌的工程师
> 配套：`docs/ARCHITECTURE.md`、`docs/specs/*`（21 份）、`docs/decisions/*`、`docs/TODO.md`（唯一权威待办）、`PROGRESS.md`、`README.md`
>
> ⚠️ 阅读提示：本文 §3「开发历程」写到第 22 轮为止，第 23–25 轮见 `PROGRESS.md`；
> 其中**第 25 轮引入三层角色 + 审核流 + 封禁库**，鉴权矩阵已变化，见 `docs/specs/21-roles-moderation.md`。

---

## 0. 一句话

Echo Content API 是一个**面向低配 VPS（1 vCPU / 1.5 GB RAM / 5 GB 磁盘）的、可自托管的全栈内容站点**：后端 Go 单二进制 + SQLite 单文件，前端 Preact 单页，前后端分离。定位是"小而美的个人博客 / 轻 CMS"，**功能深度优先于广度**，不追求高并发，追求极致的资源占用比与可维护性。

---

## 1. 宗旨与目的

### 1.1 我们要解决什么

个人作者（或小型团队）想在**一台廉价、不稳定的 VPS** 上跑一个能长期维护的内容站点，不想要 WordPress / Ghost 那种"全家桶"。我们给的是一个**足够轻、逻辑足够透明、可以整台 VPS 全托管**的方案：

- 单二进制（Go）+ 单文件数据库（SQLite）= 部署即解压，无系统依赖。
- 零 CGO，纯 Go 交叉编译，任何 Linux 发行版直接 `./echo-api` 跑起来。
- 前端是静态产物（~180 kB JS / ~48 kB CSS），放 nginx 任意静态目录即可。
- 总依赖 < 20 个模块（后端 12 个 + 前端 2 个 npm 包），供应链可审计。

### 1.2 我们不做什么

| 不做 | 原因 |
|------|------|
| 多实例水平扩展 | 单 SQLite 文件锁 + 内存限流器，不适合多进程 |
| 高并发（> 100 QPS） | 定位低配站；令牌桶限流默认 5 rps / burst 10 per identity，**防爬不抗压** |
| 复杂权限模型 | 只有 `user` / `admin` 两角色，文章维度只有 `public` / `private` 两可见性 |
| 跨设备同步草稿 | 草稿仅浏览器 localStorage（30 天 TTL），明确不跨设备 |
| FTS5 全文搜索 | 纯 Go 驱动 `glebarez/sqlite` 无 FTS5 扩展；当前 LIKE `%...%`，千级后按需换驱动 |

### 1.3 功能范围

- 用户：注册（可被 admin 一键关闭）/ 登录 / 会话 / 登出；JWT HS256 无状态。
- 内容：文章 CRUD + 置顶 + 浏览计数 + 上下篇导航；分类 / 标签（多对多）；**公开 / 私密**两可见性（spec 22，2026-09 定稿）。
- 阅读：`/` 列表（公开）、`/read/:id` 单文（公开）、`/archive` 归档、`/u/:username` 作者公开主页（仅列公开文）。
- 写：`/create` · `/edit/:id` 三模式 Markdown 编辑器（分屏预览 / 即时渲染 / 纯源码），内置工具栏、Ctrl+B/I、Ctrl+S 自动存本地、Ctrl+Enter 发布；封面图 + 标签 + 分类。
- 管理后台：侧边栏后台外壳（`/admin` 系列），站点配置、用户 CRUD、备份导出 / 导入 / 下载、回收站 7 天软删、登录日志。
- 个人中心：`/me`（我的主页 / 内容管理 / 安全中心）；当前用户文章统计 + 公开文总浏览。
- 安全：令牌桶限流（用户 ID 优先 + IP 兜底）、备份下载防路径穿越、上传大小 / 类型限制。

---

## 2. 开发思路

### 2.1 架构分层：厚薄分离

```
browser ── SPA (Preact) ──/uploads/*── 静态（uploads 是 public GET）
              │
              ▼  /api/*
   ┌─────────────────────────────────────────────────────┐
   │  middleware 链：limiter → CORS → Logger → Recover    │
   │  （limiter 在最外，Recover 最内）                     │
   │                                                     │
   │  ServeMux（Go 1.22 方法 + 通配）                     │
   │   - withAuth(handler)         ── 写 / 管理 / 会话    │
   │   - withOptionalAuth(handler) ── 公开读（22 轮起）    │
   │   - handler                   ── 公开读（auth/backup 静态…） │
   │                                                     │
   │  handlers/*  薄层：只做 HTTP↔DTO、错误码、4xx 映射    │
   │  services/*  厚层：业务规则、可见性、权限、事务        │
   │  models/*    GORM 实体 + AutoMigrate                 │
   │  config/*    .env + 种子数据                         │
   └─────────────────────────────────────────────────────┘
              │
              ▼
       SQLite（纯 Go 驱动 glebarez）
```

**原则**：`handlers` 只懂 HTTP 错误码；`services` 只懂业务规则。任何"谁能看谁"的判定都在 service，不在 handler。这让审计可以**只看 service 层就知道可见性 / 权限的完整逻辑**，不用追路由。

### 2.2 鉴权三态

| 状态 | 端点示例 | 行为 |
|------|---------|------|
| **强鉴权** `withAuth` | `POST/PUT/DELETE /api/posts`、`/api/users`、`/api/admin/*`、`/api/backup/*`、`/api/upload` | 无 / 坏 token 一律 401；解析出的 user 注入 ctx，限流器按 user ID 分桶 |
| **可选鉴权** `withOptionalAuth` | `GET /api/posts`、`/api/posts/recent`、`/api/posts/archive`、`/api/posts/{id}`、`/api/categories`、`/api/tags` | 无 token 按**匿名用户**（零值 `User{}`）放行；有 token 但无效仍 401 |
| **公开** | `GET /api/posts/{id}/public`、`/api/authors/{username}`、`/api/site-config`、`/api/auth/register/open`、`/api/backup/download`（admin 专用，单独 403）、`/uploads/*` | 直接走 handler |

**关键安全约定（审计重点）**：

- **可见性判定完全在 service，路由只决定"能不能进"。** `ListPostsPage` 按 `user` 角色与 `ID` 分三分支过滤：admin 全量；登录非 admin = `status='published' OR author_id=me`；匿名（`user.ID==""`）仅 `status='published'`。`ListArchive` 永远只列 `published`。`GetAuthor` 永远只列 `published`。
- 任何遗留 `draft` / `scheduled` 值在 `CreatePost` / `UpdatePost` 一律**白名单兜底为 `published`**（spec 22 定稿），客户端无法"藏住"或"定时发"。

### 2.3 可见性模型（spec 22，2026-09 定稿）

```
Post.Status ∈ { "published", "private" }        // "draft" 已废
匿名访客:  仅 published
登录作者:  published（全部）∪ private（仅自己的）
admin:     全部
私密文对外: 404（不是 403，避免暴露存在性）
作者公开页:  仅 published
```

写文章页只有一个"可见范围"单选（默认公开），无草稿按钮；内容管理页每行一个可见性标签（绿=公开 / 橙=私密）+ 内联切换（乐观更新 + 6s 撤销条，无确认对话框）。

### 2.4 开发与验收节奏

采用"**轮次（round）+ spec（spec）+ 验收标准（AC）**"三段式：

- **轮次**：`PROGRESS.md` 里按时间顺序记录每一轮做了什么。目前 22 轮 + 若干回归修复。
- **Spec**：`docs/specs/` 下每个页面 / 功能一份，EARS 风格 AC（WHEN...THEN...），可逐条打勾。当前 20 份（01–20，缺 13 因与 14 合并）。
- **验收**：每次实现前对照 spec 逐条过 AC，完成后回填。第十八轮就做过一次"全量 spec 合规审计"。

### 2.5 测试基线

| 层 | 工具 | 当前结果 |
|----|------|---------|
| 前端类型 | `tsc --noEmit` | ✅ 无错 |
| 前端构建 | `vite build` | ✅ JS 183 kB / gzip ~56 kB，CSS 48 kB / gzip ~8.8 kB |
| 后端编译 | `go build ./...` | ✅ |
| 后端静态检查 | `go vet ./...` | ✅ |
| 后端单测 | `go test ./...` | ✅ 8 个 `_test.go` 全绿（含 `TestListPostsHidesOtherUsersPrivate`、`TestRegisterIssuesUsableToken`、`TestSignAndVerifyToken`、`TestAuthenticateRejectsTamperedToken` 等） |

---

## 3. 开发历程（22 轮 + 回归修复）

按"主题"分组，不按时间顺序；每条注明 spec 编号、关键产物、是否已验证。

### 3.1 基础设施（第 1–8 轮）

| 轮 | 主题 | 关键产物 |
|----|------|---------|
| 1 | 后端骨架 | `cmd/server/main.go`、`internal/{config,models,services,handlers,middleware,router}`；User/Post 模型；JWT；Post CRUD |
| 2 | 后端增强 | LIKE 搜索、分页、图片上传 + 限大小、`go test` 基线 |
| 3 | 前端骨架 | Preact SPA 6 页 + 手写 i18n（中英）/主题/路由（wouter）/Toast/MD 渲染 |
| 4 | 编辑器 + 列表增强 | 双面板 Markdown 编辑器、自动保存（localStorage 30 天 TTL）、本地图床、分页防抖 |
| 5 | 体验打磨 | SM.MS 图床双模、设置页、种子 Markdown、摘要优化 |
| 6 | 界面细节 | nav active 高亮、密码 show/hide、Ctrl+B/I 快捷键、字数统计、空状态图标 |
| 7 | 分类 / 标签 | Category / Tag 模型 + many2many、侧栏筛选 |
| 8 | 置顶 / 浏览数 | `pinned` + `view_count`、上下篇导航、公开阅读接口 |

### 3.2 管理后台（第 9–11 轮）

- **第 9 轮**：分类 / 标签管理 UI + 后端 CRUD。
- **第 10 轮**：用户管理 UI、备份导出 / 导入、回收站（7 天软删）。
- **第 11 轮**：登录日志表（`login_logs`）+ admin 查看；备份恢复（事务替换全表）；回收站批量清空。

### 3.3 阅读 / 编辑体验（第 12–16 轮）

- **第 12 轮**：阅读页图片灯箱、阅读进度条、返回顶、字号设置（小 / 中 / 大 / 特大）。
- **第 13 轮**：全文搜索调研（LIKE vs FTS5）+ 测试加固（JWT 篡改 / 过期 / 重复注册 409 等边界用例）。
- **第 14 轮**：localStorage 卫生（旧草稿 TTL 清理）+ Markdown 表格样式。
- **第 15 轮**：阅读页 TOC 滚动高亮；列表页无限滚动 + 分页防抖。
- **第 16 轮**：归档页（按年-月分组）；`ListArchive` 服务 + `GET /api/posts/archive`（匿名可读）。

### 3.4 后台重构与多用户准备（第 17–19 轮）

- **第 17 轮**：定时发布 + 备份导入 + 回收站清空。**⚠️ 第 22 轮已把"定时发布"移除**，`scheduled` 状态与后台 ticker 全删，`PublishDuePosts` 方法不再存在。
- **第 18 轮**：全量 spec 合规审计（12 份 spec 逐条 AC 比对），发现并修复 2 缺口：注册即登录（`Register` 直接签 JWT）+ 备份浏览器下载。
- **第 19 轮**（后台重构）：侧边栏后台外壳、站点配置（brand / title / footer / 友链 + 注册开关）、用户 CRUD（创建 / 改密 / 禁 / 删 / 登录日志）、登录日志页、公开作者页（`/u/:username`）、注册开关。新增 spec 14/15/16。

### 3.5 用户中心与写文章（第 20–22 轮）

- **第 20 轮**：右上角用户菜单（hover / 键盘 / a11y / 退出确认）+ 个人中心三栏（我的主页 / 内容管理 / 安全中心）。新增 `GET /api/me/stats`（当前用户文章数 + 总浏览）与 `ListPostsPage` 排序参数（`sort`/`order`）。
- **第 21 轮**：写文章页重写为三段式布局 + 三模式编辑器（分屏预览 / 即时渲染 / 纯源码），完整工具栏、全快捷键（含中文输入法让位）、拖拽 / 粘贴 / 点击三种图片上传、自动保存、浮动目录、封面图、发布流程。**因沙箱无 npm 网络，Vditor 装不上 → 改为零依赖实现**（当前 `PostEditor.tsx` 是原生组件，未用 Vditor）。
- **第 22 轮**：定稿"公开 / 私密"两可见性，去掉草稿 / 定时。`PostStatus = 'published' | 'private'`，前端写文章页只一个可见范围单选（默认公开），内容管理内联切换（乐观 + 撤销）；后端 `CreatePost` / `UpdatePost` 白名单，`ListPostsPage` 按角色过滤；删除 `PublishDuePosts` 与后台 ticker；i18n / CSS 加 `editor.visibility*` 与 `badge--private` / `post-status-private`。

### 3.6 回归修复（第 22 轮后）

- **匿名读接口 401**：22 轮把读接口统一套了强鉴权后，未登录访客打 `/api/posts`、`/api/tags`、`/api/categories`、`/api/posts/recent` 全 401，前端重试又 429。修复：`WithOptionalAuth` 中间件（无 token 匿名放行、有坏 token 仍 401）+ 读路由换用 + `ListPostsPage` 按"匿名 / 非 admin / admin"三分支过滤。详见 `docs/specs/20-visibility.md` §0。

---

## 4. 审计指引

### 4.1 建议审计顺序

1. **入口与装配**：`cmd/server/main.go` → `internal/config` → `internal/router`。看装配顺序、优雅关闭、限流器位置。
2. **业务规则（service 层，~3.5k 行）**：`internal/services/services.go`（1454 行，核心 CRUD + 可见性 + 备份 + 回收站 + 站点配置）和 `accounts.go`（396 行，用户 / 登录日志 / 注册）。**权限判定全在这两层，路由不动业务。**
3. **鉴权**：`internal/handlers/auth.go` 里 `WithAuth` / `WithOptionalAuth`（241 行）。看 401 分支、user 注入 ctx、限流器 key。
4. **中间件**：`internal/middleware/limiter.go`（令牌桶 + user / IP 双 key）、`middleware.go`（CORS / Logger / Recover / Chain）。
5. **模型与迁移**：`internal/models/models.go`（`Post.ScheduledAt` 已在**第 22 轮完全删除**——字段 / DTO / 前端类型 / 后台调度器 / 旧库 `scheduled_at` 列全部清除；旧库通过 `internal/config` 的 `dropScheduledAt` 手写迁移 + `schema_migrations` 版本门控原地 drop 该列，幂等、保留数据）。⚠️ 见 §4.7 关于 GORM `AutoMigrate` + glebarez 驱动的启动崩溃陷阱。
6. **前端**：`web/src/core`（类型 / API / i18n / 主题）、`web/src/pages`（18 个页面）、`web/src/ui`（8 个组件）。

### 4.2 安全要点（审计清单）

| 类别 | 位置 | 状态 / 备注 |
|------|------|-------------|
| 密码存储 | `services.accounts` | bcrypt，非明文 |
| JWT | `services.SignToken` / `parseToken` | HS256，`jwt.WithValidMethods(["HS256"])` 强制算法；过期 1h；`JWT_SECRET` 可 `.env` 覆盖（生产必须覆盖） |
| 篡改 / 重放 | `TestAuthenticateRejectsTamperedToken` | 已覆盖：改 signature / 垃圾串 / 过期 → 401 |
| 路径穿越（上传） | `services.ServeUploads` | `name` 含 `..`/`/` 直接 404；`filepath.Join(uploadsDir, name)` |
| 路径穿越（备份下载） | `services.DownloadBackup` | `Base(name) != name` 拒绝；`Rel(dir, full)` 二次校验防逃逸 |
| 上传大小 / 类型 | `handlers/upload.go` + `services.UploadImage` | `http.MaxBytesReader` 2 MB / 文件，仅 `image/*` 通过；`UPLOAD_MAX_BYTES` 可覆盖 |
| 限流 | `middleware.Limiter` | `golang.org/x/time/rate` 令牌桶；**user ID 优先 + 客户端 IP 兜底**；默认 5 rps / burst 10 per identity；超 → 429 + `Retry-After` |
| 备份导出 | `POST /api/backup` | admin-only；事务式 dump（users/posts/categories/tags + 关联），JSON 文件落 `BackupDir` |
| 备份导入 | `POST /api/backup/import` | admin-only；**事务替换全表**（删 + 重建），避免半截状态 |
| 回收站 | `GET /api/trash` / `empty` / `{id}/restore` | admin-only；7 天软删；`EmptyTrash` 物理删 |
| 登录日志 | `GET /api/users/{id}/logins` | admin-only；只存 IP / UA / 成功 / 时间，**无密码** |
| 可见性判定 | `services.ListPostsPage` / `ListArchive` / `GetAuthor` / `GetPostPublic` | **全部 service 层判定，路由只做"能不能进"**；匿名永远只见 published |
| 写接口 | 全部 `withAuth` + admin 端点 handler 内 `me.Role != "admin"` 403 | 双层：先 token 合法，再角色合法 |
| XSS | `marked` + `DOMPurify` 清洗（前端 `core/md.ts`） | 默认配置；审计时看 `renderMD` 的白名单 |
| 已知开放 | 见 §4.4 | 无（原 SM.MS token 存 localStorage 的低危项已由 #7 改为服务端持有；`JWT_SECRET` 生产由 `ValidateSecurity` 强制覆盖） |
| JWT_SECRET 强度 | `config.ValidateSecurity` | prod/未知环境：非空/非默认值/≥32 字节/非纯重复；dev 仅 stderr 警告 |
| 首启 admin 密码 | `config.seedDemoData` + `User.MustChangePassword` | 随机 22 字符密码仅在 stderr 横幅打印一次（不落日志）；首次登录强制 `/change-password` 改密 |
| 登录失败锁定 | `middleware.LoginLimiter` | IP+账号双桶 0.5rps/burst3；5 次失败锁 15min；429 带 `Retry-After` |
| 备份导入校验 | `services.ImportBackup` | `?dry_run=1` 仅返回统计不落库；顶层 `schema_version`/`app_version` 校验；导入前 `VACUUM INTO` 自动快照（保留 3 份）；`POST /api/backup/restore-snapshot` 回滚 |
| SM.MS token 服务端持有 | `services.UploadImage` + `handlers.UploadImage` | 优先 `cfg.SmmsToken` 代理上传；无 token 返回 503；前端不再存 `smms_token` |
| 移动端菜单 a11y | `web/src/ui/UserMenu.tsx` | overlay 仅移动端渲染；PC hover / 移动端 tap+overlay；键盘可达 |

### 4.3 性能要点

- **后端**：单 SQLite，连接池刻意小（MaxOpen=4 / MaxIdle=2，`internal/config`）。慢点：`LIKE '%...%'` 搜索（千级后建议换 FTS5，但当前纯 Go 驱动无 FTS5 扩展，决策见 `docs/decisions/0001-pure-go-sqlite-no-fts5.md`）。
- **前端**：单 JS 包 ~183 kB / gzip ~56 kB，单 CSS ~48 kB / gzip ~8.8 kB。列表页无限滚动；归档页一次拉全部 published 但**剥掉 body 字段**（仅留 author/category/tags），文章上万时改按年分页。
- **限流器**：`map[identity]*rate.Limiter` 常驻内存，单实例 OK；多实例需换共享计数（Redis）。

### 4.4 已知开放问题（审计时重点关注）

| 等级 | 问题 | 位置 / 状态 |
|------|------|------------|
| 🟡 | 编辑器草稿仅 localStorage，不跨设备 | `PostEditor.tsx`（30 天 TTL，可接受） |
| 🟢 | SM.MS 图床 token 存 localStorage 明文 | spec 05 §4（第 19 轮 Draft）；建议改服务端持 token |
| 🟢 | `JWT_SECRET` 生产必须覆盖 `.env` | `internal/config`；`.env.example` 已注明；启动校验 `ValidateSecurity` |
| 🟢 | `/archive` 一次拉全部 published | `ListArchive`；已知，接受 |
| 🟢 | GORM `Select("COUNT(*)")` 污染后续 scope | 已修，需警惕同类 |

### 4.4.1 已清理项（第 20 轮）

- ✅ `listPosts()` 旧函数死代码：已从 `web/src/core/api.ts` 删除（前端仅 `listPostsPaged` 使用）。
- ✅ `scheduled_at` 废弃字段：模型字段、`CreatePostInput`/`UpdatePostInput` DTO、前端 4 处类型声明全部删除；旧库通过 `posts_new` 表 + `schema_migrations` 版本表的手写迁移（`0001_drop_scheduled_at`）原地 drop 列，幂等、保留数据、兼容无此列的新库。

### 4.5 复现环境（审计可本地跑）

```bash
# 后端
cd echo-content-api
go build -o echo-api.exe ./cmd/server
./echo-api.exe          # 监听 :8080

# 前端
cd web && npm install && npm run dev   # 监听 :5173，HMR

# 测试
go test ./...
cd web && npx tsc --noEmit && npm run build
```

种子数据：admin@echo.local / admin（或首启自动建 admin + 3 篇示例）。

### 4.6 相关文档索引

- `docs/ARCHITECTURE.md` — 设计稿（目标架构、技术选型、中间件链、未来扩展）
- `docs/specs/` — 20 份 spec（01–20），EARS AC 可逐条验收
- `docs/decisions/` — 重大决策（如 0001 纯 Go SQLite 无 FTS5、0002 localStorage 草稿 TTL）
- `PROGRESS.md` — 22 轮开发全记录 + 风险 / 方向 / 验证基线
- `HANDOVER.md` — 部署 / 运维
- `deploy/` — VPS 部署脚本（`.sh` / `.ps1` / `.env` / systemd 单元）

---

## 附录 A：代码量 / 模块量

| 层 | 行数 | 说明 |
|----|------|------|
| `internal/services/services.go` | 1454 | 核心业务（Post CRUD + 可见性 + 备份 + 回收站 + 站点配置 + 用户统计） |
| `internal/services/accounts.go` | 396 | 用户 / 登录日志 / 注册 |
| `internal/handlers/*.go` | ~1450 | 薄处理器（auth / posts / categories / accounts / admin / upload） |
| `internal/middleware/*.go` | ~500 | 令牌桶 + CORS / Logger / Recover / Chain |
| `internal/models/*.go` + `config` + `router` | ~800 | GORM 实体 + 配置 + 路由装配 |
| **后端 Go 合计** | **~5100** | 测试 8 个文件另计 |
| `web/src`（pages / ui / core） | ~6450 TS/TSX | 18 页面 + 8 组件 + core |
| 前端产物 | JS 183 kB / CSS 48 kB | gzip 后 ~56 kB / ~8.8 kB |
