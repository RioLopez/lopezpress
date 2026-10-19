# 容器化部署（Docker Compose）

`deploy/docker/` 提供一套与既有 systemd 方案**并行、可选**的容器化部署，二者不互相依赖。架构与 `deploy/README.md` 的 systemd 方案一致：**Nginx 托管前端 dist 并反代 + Go 后端 + SQLite 单文件**，不改动任何业务代码。

## 文件清单

```
deploy/docker/
├── Dockerfile.api         # 多阶段：golang:1.22-alpine 交叉编译 → distroless 静态运行（非 root）
├── Dockerfile.web         # 多阶段：node:22-alpine 构建 dist → nginx:1.27-alpine 托管
├── nginx.conf             # compose 版 vhost：upstream api:8080、SPA 回退、gzip、/uploads 反代
├── docker-compose.yml     # 编排 api + web；数据落命名卷 api-data
├── api.env                # api 服务环境变量模板（务必改 JWT_SECRET）
└── README.md              # 本文件
```

根目录 `.dockerignore` 已就位：构建 context 为仓库根，`**/node_modules`、`web/dist`、`/data`、`*.db`、`.env` 等一律排除，context 保持精简、不泄露密钥。

## 前置条件

- Docker Engine 24+ 与 Compose 插件（`docker compose version`）。
- 主机放行 80 端口（web）。api 默认不对外，仅 compose 网络内可达。

## 快速开始

```bash
cd deploy/docker

# 1. 生产必须改密钥
openssl rand -hex 32         # 输出粘进 api.env 的 JWT_SECRET

# 2. 构建并启动
docker compose up -d --build

# 3. 验证
curl http://<主机IP>/health          # {"status":"ok"}
curl http://<主机IP>/                 # 前端 SPA index.html
```

首次启动时，若数据库为空，后端会在 **stderr（`docker compose logs api`）打印一次性随机 admin 密码横幅**——立即登录并改密；或在 `api.env` 设 `ADMIN_INITIAL_PASSWORD` 固定首启密码。

## 数据持久化

- SQLite 与本地图片都在 api 容器的 `/data`（`app.db` + `uploads/`），落在命名卷 **`api-data`**。
- 备份：`docker run --rm -v echo-content-api_api-data:/data -v "$PWD":/backup alpine tar czf /backup/app-data.tgz -C /data .`
- 想用主机目录代替命名卷：把 compose 里 `api-data:/data` 改成 `./data:/data`，并确保该目录对容器内 `nonroot`（uid 65532）可写：`sudo chown -R 65532:65532 ./data`。

## 端口与 TLS

- web 暴露 `80:80`。api 不暴露（调试时取消 compose 里 `ports: 127.0.0.1:8080:8080` 注释）。
- **生产 TLS**：推荐在 web 前再放一层反代（Caddy/Traefik/主机 Nginx）终止 HTTPS；或把证书挂进 web 容器并将 `nginx.conf` 的 `listen 80` 改为 `listen 443 ssl`（补 `ssl_certificate`/`ssl_certificate_key`）。

## 常用命令

```bash
docker compose logs -f api      # 后端日志（含首启 admin 密码横幅）
docker compose logs -f web      # Nginx 日志
docker compose ps               # 状态
docker compose down             # 停止（保留 api-data 卷）
docker compose up -d --build    # 改代码后重建升级
docker compose down -v          # 停止并删除数据卷（危险，清空 SQLite！）
```

## 资源占用（对标 1 vCPU / 1.5 GB VPS）

| 容器 | 基础镜像 | 常驻内存量级 |
| --- | --- | --- |
| api | distroless static（~2 MB） | 几十 MB |
| web | nginx:1.27-alpine | ~10 MB |

两个容器合计远低于 systemd 单进程方案，1.5 GB 机器余量充足。

## 与 systemd 方案的取舍

- **systemd（`deploy/` 根目录脚本）**：单二进制 + 主机 Nginx，最轻、无 Docker 依赖；适合已在用的 VPS。
- **Compose（本目录）**：一键 `up -d --build`、环境一致、易于迁移；代价是主机需装 Docker。两者可并存，按喜好选择。

## 验证状态（诚实说明）

- ✅ 已在本机验证：`CGO_ENABLED=0 GOOS=linux go build` 静态交叉编译通过（api 构建阶段核心命令）；`vite build` 前端产物构建通过（web 构建阶段核心命令）。
- ⚠️ **未执行 `docker build`**：开发机未安装 Docker，故镜像未能实际构建。Dockerfile/compose 依据上述两项已验证的前提与既有 `deploy/nginx.conf` 编写；首次在有 Docker 的环境执行 `docker compose up -d --build` 即可验证，如有细节问题再按日志微调。
