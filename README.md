# lopezpress

基于 **Go 标准库（`net/http`）+ GORM + SQLite + Preact** 的内容型站点全栈项目，前后端分离。

- 后端：Go 1.22+ 标准库 `ServeMux`（方法 + 通配路由），纯 Go SQLite 驱动（无需 CGO，可交叉编译）
- 前端：Preact + Vite + TypeScript（`web/`），构建为单页应用（SPA）
- 鉴权：JWT（HS256，无状态）+ bcrypt 密码哈希
- 限流：自写令牌桶中间件（用户ID优先 + IP 兜底，默认 5 rps / burst 10）

## 功能

- 用户注册 / 登录 / 当前会话（`/api/auth/register` `/login` `/me` `/logout`）
- JWT 鉴权（Bearer Token）
- 文章 CRUD（作者 / 管理员权限控制）
- 首次启动自动建表 + 写入演示数据（admin + 3 篇示例文章）

## 项目结构

```
lopezpress/
├── cmd/server/main.go         # 入口：装配依赖、初始化 DB、启动 HTTP 服务（优雅关闭）
├── internal/
│   ├── config/                # 配置（.env）+ 数据库连接 + 迁移 + 种子数据
│   ├── models/                # GORM 数据模型（User / Post）
│   ├── handlers/              # 薄处理器（auth / posts），注入 services
│   ├── services/              # 业务厚层（注册/登录/JWT/文章 CRUD 全部逻辑）
│   ├── middleware/            # 自写中间件：Chain / Logger / CORS / Recover / 限流
│   ├── router/                # 路由装配（Go 1.22 ServeMux 方法+通配 + 中间件链）
│   └── httpx/                 # 请求/响应小工具
├── web/                       # 前端（Preact + Vite + TS）
├── deploy/                    # 部署脚本（systemd / Nginx / 一键上传）
├── docs/ARCHITECTURE.md       # 架构设计文档
├── HANDOVER.md                # 交接文档（AI 接手必读：陷阱 / 未决决策 / 自测清单）
├── LopezPress.bat                # 一键启动（Windows）：后端 + 前端，自动开浏览器
├── .env                       # 运行时配置（端口 / JWT / DB 路径 / 限流）
└── go.mod
```

> 启动命令：`go run ./cmd/server`，默认监听 `:8080`。
> **一键启动（Windows）**：双击根目录 `LopezPress.bat` 即可同时拉起后端（:8080）和前端（:5173）并自动打开浏览器；支持 `status` / `stop` / `restart` / `dev` / `build` / `help` 子命令，已做端口幂等检测，避免重复启动。

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
$env:RATE_LIMIT_RPS="5"
$env:RATE_LIMIT_BURST="10"
$env:SM_MS_TOKEN=""             # 可选：SM.MS 图床 token 服务端持有（spec #7）
```

> 生产部署务必：更换 `JWT_SECRET`（≥32 字节）、`APP_ENV=prod`、`CORS_ORIGINS` 显式列表、启用 HTTPS。安全加固细节见 `docs/AUDIT_REPORT.md` §4.4.2，接手前必读 `HANDOVER.md`。

## 演示账号

| 用户名 | 邮箱 | 密码 | 角色 |
| --- | --- | --- | --- |
| admin | admin@example.com | 首次全新启动随机生成（仅终端打印一次） | admin |

> **admin 密码说明**：`seedDemoData` 仅在数据库**不存在 admin 账号**时新建，并使用随机 22 字符密码（在 stderr 打印一次横幅，不落日志），登录后强制跳 `/change-password` 改密。若数据库**已存在** admin 账号（如本机开发库），则沿用既有密码不被覆盖——历史开发库的 admin 密码可能是早期固定的 `admin123` 或此前某次随机值，**以实际数据库为准**；忘记密码可用 `ADMIN_INITIAL_PASSWORD` 环境变量在下次首启（无 admin 时）固定，或走 `POST /api/users/{id}/reset-password`（admin）。

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
- 框架为 **Preact**（API 兼容 React），路由用 `@preact/signals` + 状态导航，**非** `react-router`
- `tsconfig.json` 用 `jsx: "react-jsx"` + `jsxImportSource: "preact"`
- `createContext` 来自顶层 `preact`；`useContext/useEffect/useState/useRef` 来自 `preact/hooks`
- 子类型用 `ComponentChildren`（Preact 无 `Child` 类型）
- token 存 `localStorage`，启动时用 `GET /api/auth/me` 重新校验有效性

## 部署

见 [deploy/README.md](deploy/README.md)：systemd 服务、Nginx 反代 + 前端静态托管、一键上传脚本（已包含前端构建）。

## 说明

- 默认 SQLite（`glebarez/sqlite` 纯 Go 驱动，无需 CGO，可交叉编译到 Linux）。换 PostgreSQL/MySQL 仅需改 `config.InitDB` 的驱动。
- JWT 无状态，服务端不保存 token；`POST /api/auth/logout` 仅作为客户端信号，需真正吊销请引入黑名单。
- 生产环境务必：更换 `JWT_SECRET`、收紧 CORS、启用 HTTPS。
