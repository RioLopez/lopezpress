> 类别 B · 部署与运维 — 上线/排障时看这份。
> 2026-10-27 合并了原 `deploy/LOCAL.md`、`deploy/docker/README.md`、`docs/deploy/README.md` 三份（均已删除，原文在仓库外备份 `_doc_backup_20261027_before_prune/`）。

# 部署（VPS / 本机 / 容器）

目标机型：**1 vCPU / 1.5 GB RAM / 5 GB 磁盘**，资源预算 ≤ 70%。
形态：纯 Go 静态二进制（`CGO_ENABLED=0`）+ SQLite + JWT 无状态，跑在 systemd 下，Nginx 反代。

**一分钟版**：

```
开发机跑 bash deploy/build.sh  →  scp 二进制 + web/dist  →  systemctl restart lopezpress
→  bash deploy/check-live.sh 确认 in sync  →  完工
```

---

## 1. 资源预算

| 维度 | 总量 | 预算（≤70%） | systemd 限制 |
|---|---|---|---|
| 内存 | 1.5 GB | 1.0 GB | `MemoryMax=768M` / `MemoryHigh=640M` |
| CPU | 1 vCPU | — | `CPUQuota=100%`（单核独占） |
| 磁盘 | 5 GB | 3.5 GB | SQLite + 日志，远低于此 |

Go 服务空闲几十 MB，加连接池 / 限流桶 / 日志缓冲后稳态 < 200 MB。

## 2. 文件清单

```
deploy/
├── lopezpress.service      # systemd 单元（含沙箱与 MemoryMax）
├── lopezpress.env          # 生产 .env 模板（部署后必须填真实 JWT_SECRET）
├── install-systemd.sh      # VPS 上安装/重启（建用户、装单元、enable）
├── deploy-to-vps.sh        # 本地一键：交叉编译 → scp → 远端跑 install-systemd.sh
├── build.sh                # 本地构建（检查 Go 版本 ≥ go.mod 要求）
├── check-live.sh           # 比对线上 assets 哈希与本地产物：在跑的是不是这份代码
├── diagnose.sh / fix-deploy.sh   # 线上排障与修复
└── nginx.conf              # Nginx 反代（443/HTTP2/压缩/真实 IP/点文件拒绝）
```

## 3. 首次部署

```bash
cd lopezpress
export VPS_USER=root VPS_HOST=你的VPS公网IP
bash deploy/deploy-to-vps.sh
```

脚本会：交叉编译静态二进制 → scp 二进制 + systemd 单元 + `.env` 到 `/opt/lopezpress/` →
远端建 `echoapi` 系统用户、装单元、`enable` + `restart` → 打印状态与最近日志。

验证：`ssh root@VPS 'curl -s http://127.0.0.1:8081/health'` → `{"status":"ok"}`

> 服务默认 **`HOST=127.0.0.1 PORT=8081`**，只监听环回；对外只经 Nginx。
> 不要为了"方便测试"改成 `0.0.0.0`——那会把 Go 进程直接暴露到公网，绕过全部 Nginx 防护。

## 4. 上线必须做的三件事

### 4.1 改 `JWT_SECRET`

```bash
ssh root@VPS 'cd /opt/lopezpress && openssl rand -hex 32'   # 粘进 .env，然后 restart
```

空 secret 时登录接口直接 500，不会回退默认值（设计如此）。

### 4.2 配 `TRUSTED_PROXIES`

```ini
TRUSTED_PROXIES=127.0.0.1/32      # Nginx 与后端同机
TRUSTED_PROXIES=172.16.0.0/12     # Docker Compose（后端在容器网段）
```

**不配的后果**：后端看到的客户端 IP 全是反代地址，于是浏览量去重退化成"全站一天只算一次"，
登录限流把所有访客算成同一个 IP（一个人被锁，全站都进不去）。
`internal/clientip` 只信任这个白名单传来的 `X-Forwarded-For`，不在白名单时用 `RemoteAddr`（防伪造）。

### 4.3 换 `deploy/nginx.conf`

三个不能省的 location（顺序有讲究）：

| location | 作用 | 漏了会怎样 |
|---|---|---|
| `location ^~ /.well-known/` | 放行 ACME 验证 | 证书续期失败 |
| `location ~ /\.` | **拒绝一切点文件** | SPA 回退会把 `/.git/config`、`/.env` 当页面**返回 200** |
| `location ^~ /uploads/` | 反代到后端 | 图片掉进 `try_files` 返回 `index.html`（"图裂了"） |

`^~` 前缀不能少：下面的 `location ~* \.(js\|css\|png…)$` 是正则 location，
nginx 里**正则优先于普通前缀**，写 `location /uploads/` 会被那条正则抢走。
安全头（HSTS/CSP/X-Frame-Options 等）下沉到各自 location，**不要在 server 级再写一份**——
重复下发会让 CSP 出现两份而互相失效。

`client_max_body_size` 必须 ≥ 后端 `upload_max_image`（当前 nginx 32m / 后端 20MB）。

## 5. 本机部署（Windows）

`.env` 最小集（参照 `.env.example`）：

```ini
PORT=8080
HOST=127.0.0.1
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

| 模式 | 怎么起 |
|---|---|
| 一键（推荐） | 双击 `LopezPress.bat`（幂等，自动开浏览器） |
| 后端 | `./bin/lopezpress.exe` |
| 前端开发（热更新 + `/api` 代理） | `cd web && npx vite` → `http://localhost:5173` |
| 生产模式（前后端同端口） | `npm run build` → 把 `web/dist` 复制成根目录 `dist/` → 起后端 → `http://localhost:8080` |

> 若 `dist/` 不存在，SPA 路由（`/create`、`/read/<id>`）需要 Nginx `try_files` 兜底；
> 后端自己的 404 是 JSON，不是页面。

**首次启动**会自动建 `data/`、初始化 `data/app.db`、写种子数据。
admin 密码是**随机 22 位**，在 stderr 打印一次横幅（不进日志文件），登录后强制改密。
已有数据库时不会覆盖既有 admin 密码；要固定首启密码用 `ADMIN_INITIAL_PASSWORD`。

## 6. 升级 / 回滚

升级只换二进制 + 前端产物，数据（`data/app.db`）独立保留：

```bash
bash deploy/build.sh                                   # 构建 bin/lopezpress-<ver>-linux-amd64
scp bin/lopezpress-<ver>-linux-amd64 root@VPS:/opt/lopezpress/lopezpress
scp -r web/dist/* root@VPS:/opt/lopezpress/web/
ssh root@VPS 'chmod 755 /opt/lopezpress/lopezpress && systemctl restart lopezpress'
bash deploy/check-live.sh                              # 应输出 in sync
```

**回滚**：保留上一版二进制命名（`lopezpress.v<N>`），scp 回去 + `restart`。
**前端与后端必须同批更新**——`check-live.sh` 就是用来确认这件事的（比对 assets 哈希），
线上跑旧前端配新后端时 `contract_check` 管不到，只有这个脚本能发现。

## 7. 常用运维命令

```bash
systemctl start|stop|restart|status lopezpress
journalctl -u lopezpress -f                 # 跟踪日志
journalctl -u lopezpress -n 200             # 最近 200 行
systemctl status lopezpress | grep -E 'Memory|CPU'   # 确认没爆 768M
ss -lntp | grep 8081                        # 端口占用
```

## 8. 故障排查

| 症状 | 排查 |
|---|---|
| `OOMKilled` | 内存超 `MemoryMax=768M`。调低 `DB_MAX_OPEN`（如 2），或把 `MemoryMax` 提到 900M（别超 1G 预算） |
| 启动后立即重启 | `journalctl -u lopezpress -n 100` 看 `.env` 路径/权限；确认 `User=echoapi` 能读写 `data/` |
| `/health` 500 | `/health` **不碰 DB**，500 说明进程本身有问题；`/health/ready` 500 才是数据库坏了或路径不可写 |
| 限流误报 429 | 客户端 IP 桶默认 `RATE_LIMIT_RPS=20 / BURST=50`。先确认 `TRUSTED_PROXIES` 配对了（配错会让所有人共用一个桶） |
| 升级后端口被占 | `ss -lntp \| grep 8081`，确认旧进程没在跑 |
| 域名后访问 404 / 图裂 | 见 4.3 的三个 location |
| `deploy/check-live.sh` 报 out of sync | 前端产物没传全，或传到了错误目录。重新 `scp -r web/dist/*` |

## 9. HTTPS

- **直接暴露 8081**（测试用）：`curl http://VPS_IP:8081/health`。生产不要这样。
- **Nginx 反代（推荐）**：`deploy/nginx.conf` → `/etc/nginx/conf.d/lopezpress.conf`，
  改 `server_name` 与证书路径，`nginx -t && systemctl reload nginx`。
  证书用 Let's Encrypt（`certbot --nginx`），`/.well-known/` 已在 4.3 放行。

## 10. 容器化（可选，**未实跑过**）

`deploy/docker/` 有一套 Compose：`api` 用 distroless 静态镜像，`web` 用 nginx:alpine 托管
前端 `dist` 并反代 `/api`、`/health`、`/uploads`。数据落命名卷 `api-data`。

```bash
cd deploy/docker
openssl rand -hex 32                                   # 粘进 api.env 的 JWT_SECRET
docker compose up -d --build
curl http://<主机IP>/health
```

**与 systemd 的取舍**：systemd 最轻、无 Docker 依赖、适合已装好的 VPS；Compose 一键起、
环境一致、易迁移，代价是主机要装 Docker。

> **诚实说明**：本机没有 Docker，这套方案**从未真正跑起来过**，只是按 distroless + nginx
> 的标准写法配置的。要用它请先在测试机上完整跑一遍 `up -d --build` + 备份恢复，
> 再上生产。Dockerfile 的基础镜像已对齐 `go.mod` 要求的 1.26（`golang:1.26-alpine`），
> 且 `GOSUMDB` 必须是 `sum.golang.org`——设成 `off` 会让 toolchain 下载直接失败。
