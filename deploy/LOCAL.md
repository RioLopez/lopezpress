> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 本机部署

本目录提供交叉编译产物 + VPS 部署脚本。本机开发/验证流程如下。

---

## 一、产物清单

| 文件 | 说明 |
|---|---|
| `bin/lopezpress.exe` | Windows 本地开发用（`go build` 默认） |
| `bin/lopezpress-linux-amd64` | Linux amd64 交叉编译（`CGO_ENABLED=0`，用于部署到 VPS） |
| `web/dist/` | 前端构建产物（`vite build` 生成） |

> 每次代码改动后重新构建：
> ```powershell
> # 后端
> & 'C:\Users\Lopez\go-install\go\bin\go.exe' build -trimpath -ldflags="-s -w" -o bin\lopezpress.exe ./cmd/server
>
> # 前端
> cd web
> & 'C:\Users\Lopez\node-install\node-v20.19.0-win-x64\node.exe' node_modules\vite\bin\vite.js build
> ```

---

## 二、本地启动（Windows）

### 1. 准备环境变量

项目根目录下创建 `.env`（参照 `.env.example`）：

```ini
PORT=8080
JWT_SECRET=dev-secret-change-me-in-prod
JWT_EXPIRY_HOURS=24
DB_PATH=data/app.db
DB_MAX_OPEN=4
DB_MAX_IDLE=2
LOG_LEVEL=warn
RATE_LIMIT_RPS=5
RATE_LIMIT_BURST=10
UPLOAD_DIR=data/uploads
UPLOAD_MAX_BYTES=2097152
BACKUP_DIR=data/backups
```

### 2. 启动后端

```powershell
cd c:\Users\Lopez\Desktop\Lopez\lopezpress
.\bin\lopezpress.exe
```

首次启动会自动：
- 创建 `data/` 目录
- 初始化 `data/app.db`（SQLite 数据库）
- 写入种子数据：`admin / admin123`（admin 角色）+ 3 篇示例文章 + 3 个分类 + 5 个标签

### 3. 启动前端

开发模式（热更新 + `/api` 代理到 `:8080`）：

```powershell
cd web
& 'C:\Users\Lopez\node-install\node-v20.19.0-win-x64\node.exe' node_modules\vite\bin\vite.js
```

打开浏览器访问：

- 前端：`http://localhost:5173`
- 后端：`http://localhost:8080`
- 健康检查：`http://localhost:8080/health`

### 4. 生产模式（前后端合并到 `:8080`）

前端构建后由 Go 直接托管。把 `web/dist` 放到项目根目录 `dist/`，后端启动时读取该目录：

```powershell
# 1. 复制 dist
Copy-Item -Recurse -Force web\dist dist

# 2. 启动
.\bin\lopezpress.exe
```

浏览器访问 `http://localhost:8080` 即可看到完整站点。

> ⚠️ 若 `dist/` 不存在，后端 404 兜底只处理未知路径；SPA 路由（`/create`、`/read/<id>`）在生产模式下需要 Nginx 的 `try_files` 兜底。

---

## 三、VPS 部署（deploy-to-vps.sh）

前提：VPS 为 1 vCPU / 1.5 GB RAM / 5 GB 磁盘。

```bash
# 1. 交叉编译（本目录运行）
bash build.sh

# 2. 推送 + 安装
./deploy-to-vps.sh user@vps.example.com

# 3. 验证
ssh user@vps.example.com
curl http://localhost/health
sudo systemctl status lopezpress
```

`deploy-to-vps.sh` 会：
- 把 `bin/lopezpress` + `web/dist/` 打包推送
- 解压到 `/opt/lopezpress`
- 安装 `lopezpress.service`（systemd，`MemoryMax=768M`）
- 启用 Nginx 反代（`/` → 静态文件，`/api` `/uploads` → `:8080`）
- 写入 `/opt/lopezpress/.env`（提示覆盖 `JWT_SECRET`）
- `systemctl daemon-reload && systemctl enable --now lopezpress`

---

## 四、冒烟测试

```bash
# 健康检查
curl http://localhost/health

# 登录（种子账号）
curl -X POST http://localhost/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'

# 登录拿 token（字段是 email，不是 username；密码见首启终端横幅或 .env 的 ADMIN_INITIAL_PASSWORD）
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"<首启随机密码>"}' \
  | sed -E 's/.*"token":"([^"]+)".*/\1/')

# 文章列表（匿名也能读，除非后台关掉了「公开阅读」）
curl http://localhost:8080/api/posts?page=1

# 带 token 调后台
curl http://localhost:8080/api/admin/dashboard -H "Authorization: Bearer $TOKEN"
```

> 本地直跑（不经过 Nginx）**不需要** `TRUSTED_PROXIES`；它只在反代后面才需要。
> 反向代理场景见 `deploy/README.md`「同时配 TRUSTED_PROXIES」一节。
