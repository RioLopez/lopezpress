> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# lopezpress

基于 **Go 标准库（`net/http`）+ GORM + SQLite + Preact** 的内容型站点全栈项目，前后端分离。

- 后端：Go 1.22+ 标准库 `ServeMux`（方法 + 通配路由），纯 Go SQLite 驱动（无需 CGO，可交叉编译）
- 前端：Preact + Vite + TypeScript（`web/`），构建为单页应用（SPA）
- 鉴权：JWT（HS256，无状态）+ bcrypt 密码哈希
- 限流：自写令牌桶中间件（用户ID优先 + IP 兜底，默认 **20 rps / burst 50**，`.env` 的 `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` 可调）+ 登录专用限流（IP/账号双桶 + 5 次失败锁）

## 功能

**内容**
- 文章 CRUD（作者 / 管理员权限控制）、Markdown 渲染 + XSS 白名单清洗
- 五态状态机：`published` / `private` / `pending` / `rejected` / `banned`（`banned` 仅 root 可见）
- 分类 / 标签（many2many）、置顶、浏览计数、上下篇导航、归档页、RSS + 站点地图
- 全文搜索（`LIKE`）、分页 / 无限滚动、封面图、图片上传（本地 / SM.MS 双模）

**账号与权限**
- 注册 / 登录 / 当前会话 / 登出 / 登出所有设备（`token_version` 吊销）
- 三角色层级：`root`（全库唯一，不可经任何 API 创建或提升）/ `admin` / `user`
- 内容审核流（user→admin 审、admin→root 审）+ root 封禁库（送出/永久删）
- 个人中心（我的主页 · 内容管理 · 安全中心：改密 / 登录历史 / 吊销会话）
- 操作审计日志（`ActionLog`）、登录历史、禁用账号即时失效

**运维**
- JSON 备份导出 / 导入（含 dry-run 与导入前自动快照）、快照列表与恢复（root-only）、回收站
- SMTP 邮件通知（登录失败 / 改密 / 异地登录，手写零依赖客户端）
- 首启自动建表 + 写入演示数据（root 账号 `admin` + 3 篇示例文章 + 3 分类 + 5 标签）

## 项目结构

```
lopezpress/
├── cmd/server/main.go         # 入口：装配依赖、初始化 DB、启动 HTTP 服务（优雅关闭）
├── internal/
│   ├── config/                # 配置（.env）+ 数据库连接 + 迁移 + 种子数据
│   ├── models/                # GORM 数据模型（User / Post / Category / Tag / ActionLog / LoginLog / SiteConfig）
│   ├── handlers/              # 薄处理器（auth / accounts / posts / categories / moderation / upload / admin / feed / query）
│   ├── services/              # 业务厚层（含 roles.go 权限唯一入口、canSeePost 可见性权威）
│   ├── middleware/            # 自写中间件：Chain / Logger / CORS / Recover / 通用限流 / 登录限流
│   ├── router/                # 路由装配（Go 1.22 ServeMux 方法+通配 + 中间件链）
│   └── httpx/                 # 请求/响应小工具
├── web/                       # 前端（Preact + Vite + TS）：22 个页面 + 9 个公共组件 + core 层（api/auth/i18n/router/theme/md/format）
├── deploy/                    # 部署脚本（systemd / Nginx / Docker Compose / 一键上传）
├── docs/                      # 文档体系（入口 docs/README.md；AI 协作见 AGENTS.md）
├── scripts/contract_check.py  # 前后端 method+path 契约自动核对
├── HANDOVER.md                # 交接文档（AI 接手必读：陷阱 / 未决决策 / 自测清单）
├── AGENTS.md                  # AI 协作者入口（红线 + 验证门禁 + 文档两类制）
├── LopezPress.bat             # 一键启动（Windows）：后端 + 前端，自动开浏览器
├── .env                       # 运行时配置（端口 / JWT / DB 路径 / 限流）
└── go.mod
```

> 启动命令：`go run ./cmd/server`，默认监听 `:8080`。
> **一键启动（Windows）**：双击根目录 `LopezPress.bat` 即可同时拉起后端（:8080）和前端（:5173）并自动打开浏览器；支持 `status` / `stop` / `restart` / `dev` / `build` / `help` 子命令，已做端口幂等检测，避免重复启动。
> **后端路由总量**：63 条（前端已接入 56 条，`scripts/contract_check.py` 可复核；未接入的 6 条为 `/`、`/health`、`/feed`、`/sitemap.xml`、`/uploads/` 静态服务与 `POST /api/admin/ban/{id}`）。

## 快速开始（本地开发）

### 1. 安装 Go

需要 Go 1.22+（标准库 `ServeMux` 方法通配路由特性）。

```powershell
winget install GoLang.Go   # 或手动安装后重开终端
go version
```

### 2. 启动后端

```powershell
cd lopezpress
go mod tidy
go run ./cmd/server        # 默认监听 :8080
```

首次运行会在 `data/app.db` 创建 SQLite 数据库并写入演示数据。

### 3. 启动前端

```powershell
cd web
npm install
npm run dev               # http://127.0.0.1:5173
```

前端 dev server 会把 `/api/*` 和 `/health` 反代到 `http://localhost:8080`，可同源自由调用后端。

### 4. 本地预览生产构建（可选）

```powershell
cd web
npm run build            # 产出 dist/（SPA）
npm run preview          # http://127.0.0.1:4173，同样带 API 反代
```

`preview` 与 `dev` 一样把 `/api`、`/health` 反代到后端，可用于验收构建产物；线上则用 Nginx 托管 `dist/` + 反代后端（见部署）。

### 5. 环境配置（可选）

默认值可直接运行。需要覆盖时（完整清单见 `.env.example` 与 `docs/ARCHITECTURE.md` §8）：

```powershell
$env:PORT="8080"
$env:APP_ENV="dev"              # prod 时强制校验 JWT_SECRET 强度 + CORS 非 "*"
$env:JWT_SECRET="change-me"     # prod 必须 ≥32 字节强随机，否则启动失败
$env:JWT_EXPIRY_HOURS="24"
$env:DB_PATH="data/app.db"
$env:CORS_ORIGINS="*"           # prod 改为显式来源列表，逗号分隔
$env:RATE_LIMIT_RPS="20"
$env:RATE_LIMIT_BURST="50"
$env:SM_MS_TOKEN=""             # 可选：SM.MS 图床 token 服务端持有（spec #7）
```

> 生产部署务必：更换 `JWT_SECRET`（≥32 字节）、`APP_ENV=prod`、`CORS_ORIGINS` 显式列表、启用 HTTPS。安全加固细节见 `docs/AUDIT_REPORT.md` §4.4.2，接手前必读 `HANDOVER.md`。

## 演示账号

| 用户名 | 邮箱 | 密码 | 角色 |
| --- | --- | --- | --- |
| admin | admin@example.com | 首次全新启动随机生成（仅终端打印一次） | **root**（全库唯一超级管理员） |

> **admin 密码说明**：`seedDemoData` 仅在数据库**不存在 admin 账号**时新建，并使用随机 22 字符密码（在 stderr 打印一次横幅，不落日志），登录后强制跳 `/change-password` 改密。若数据库**已存在** admin 账号（如本机开发库），则沿用既有密码不被覆盖——历史开发库的 admin 密码可能是早期固定的 `admin123` 或此前某次随机值，**以实际数据库为准**；忘记密码可用 `ADMIN_INITIAL_PASSWORD` 环境变量在下次首启（无 admin 时）固定。
>
> ⚠️ **首启账号是 root，不是 admin**（`config.go` seed 写入 `Role: RoleRoot`）。因此：
> - `POST /api/users/{id}/reset-password` **无法重置 root 密码**（admin 重置 root 会被 `canManageUser` 403 拦下，这是 P0-1 的修复结果）；
> - root 密码一旦丢失**没有 API 恢复路径**，只能直改数据库（`password_hash`）或用 `ADMIN_INITIAL_PASSWORD` 重建库。这是当前已知运维缺口。

## 验证

```powershell
# 健康检查
curl http://localhost:8080/health

# 登录拿 token（admin；密码见上方"admin 密码说明"：全新库用终端横幅里的随机密码，已有库用既有密码）
curl -X POST http://localhost:8080/api/auth/login -H "Content-Type: application/json" -d '{"email":"admin@example.com","password":"<密码>"}'

# 当前会话
curl http://localhost:8080/api/auth/me -H "Authorization: Bearer <token>"

# 文章列表
curl http://localhost:8080/api/posts -H "Authorization: Bearer <token>"

# 创建文章
curl -X POST http://localhost:8080/api/posts -H "Content-Type: application/json" -H "Authorization: Bearer <token>" -d '{"title":"新文章","body":"正文"}'
```

## 前端说明（web/）

| 命令 | 说明 |
| --- | --- |
| `npm run dev` | Vite dev server（带 API 反代），`http://127.0.0.1:5173` |
| `npm run build` | 构建生产产物到 `web/dist/`（SPA） |
| `npm run preview` | 本地预览构建产物 |
| `npm run typecheck` | `tsc --noEmit` 类型检查 |

要点：
- 框架为 **Preact**（API 兼容 React），路由用 **`wouter`**（`core/router.tsx` 薄封装），**非** `react-router` / `preact-router`
- `tsconfig.json` 用 `jsx: "react-jsx"` + `jsxImportSource: "preact"`
- `createContext` 来自顶层 `preact`；`useContext/useEffect/useState/useRef` 来自 `preact/hooks`
- 子类型用 `ComponentChildren`（Preact 无 `Child` 类型）
- token 存 `localStorage`，启动时用 `GET /api/auth/me` 重新校验有效性
- 实际在用的运行时依赖只有 `preact` / `wouter` / `marked`；`package.json` 中 `@preact/signals`、`@preact/signals-core`、`preact-router`、`preact-render-to-string` **全仓库零 import（待清理）**

## 文档导航

仓库文档分**两类**（见 `AGENTS.md` §6）：Class A 约束 AI 怎么干活，Class B 描述项目是什么。

| 想了解 | 看这里 |
| --- | --- |
| AI 协作入口（红线 / 门禁 / 文档两类制） | [`AGENTS.md`](AGENTS.md) |
| AI 工作流程标准（T0–T3 分级 / 五阶段 / 十条红线） | [`docs/AI_STANDARD.md`](docs/AI_STANDARD.md) |
| 接手交接（现状 / 关键陷阱 / 未决决策） | [`HANDOVER.md`](HANDOVER.md) |
| 轮次史 | [`PROGRESS.md`](PROGRESS.md) |
| 唯一权威待办 | [`docs/TODO.md`](docs/TODO.md) |
| 架构设计 / 代码地图 | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) · [`docs/PROJECT_OVERVIEW.md`](docs/PROJECT_OVERVIEW.md) |
| API 参考 | [`api.md`](api.md) |
| 页面规格 / 决策记录 / 审计报告 | [`docs/specs/`](docs/specs) · [`docs/decisions/`](docs/decisions) · [`docs/AUDIT_2026-10-15.md`](docs/AUDIT_2026-10-15.md) |
| 提示词模板（继续 / 新功能 / 修 bug / 自查 / 验证） | [`docs/prompts/`](docs/prompts) |

## 部署

见 [deploy/README.md](deploy/README.md)：systemd 服务、Nginx 反代 + 前端静态托管、一键上传脚本（已包含前端构建）；容器化方案见 [deploy/docker/](deploy/docker)。

## 说明

- 默认 SQLite（`glebarez/sqlite` 纯 Go 驱动，无需 CGO，可交叉编译到 Linux）。换 PostgreSQL/MySQL 仅需改 `config.InitDB` 的驱动。
- JWT 无状态，但**已支持服务端吊销**：`User.token_version` 写入 token 的 `ver` claim，`POST /api/auth/logout-all` bump 版本使旧 token 全部 401（`POST /api/auth/logout` 仍是客户端信号，需要时可改用 logout-all）。
- 生产环境务必：更换 `JWT_SECRET`（≥32 字节）、收紧 CORS、启用 HTTPS。
