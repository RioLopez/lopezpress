# VPS 部署（systemd）说明

目标机型：**1 vCPU / 1.5 GB RAM / 5 GB 磁盘**，资源预算 ≤ 70%。
架构：纯 Go 静态二进制（`CGO_ENABLED=0`）+ SQLite + JWT 无状态鉴权 + 自写限流，跑在 systemd 下。

## 资源预算

| 维度 | 总量 | 预算（≤70%） | systemd 限制 |
| --- | --- | --- | --- |
| 内存 | 1.5 GB | 1.0 GB | `MemoryMax=768M` / `MemoryHigh=640M` |
| CPU | 1 vCPU | — | `CPUQuota=100%`（单核独占） |
| 磁盘 | 5 GB | 3.5 GB | SQLite + 日志，远低于此 |

Go 服务空闲常驻约几十 MB，加连接池 / 限流桶 / 日志缓冲后稳态 < 200 MB，余量充足。

## 文件清单

```
deploy/
├── lopezpress.service   # systemd 单元
├── lopezpress.env       # 生产 .env 模板（部署后填真实 JWT_SECRET）
├── install-systemd.sh         # VPS 上安装/重启服务（创建用户、装单元、启用）
├── deploy-to-vps.sh           # 本地一键：交叉编译 → scp → 在 VPS 跑 install-systemd.sh
└── nginx.conf                 # 可选 Nginx 反代（443/HTTP2/压缩/透传真实 IP）
```

## 首次部署（推荐一键脚本）

开发机（Linux 或 Windows Git Bash）：

```bash
cd lopezpress
export VPS_USER=root VPS_HOST=你的VPS公网IP
bash deploy/deploy-to-vps.sh
```

脚本会：
1. 交叉编译 Linux amd64 静态二进制到 `bin/lopezpress`；
2. scp 上传二进制 + `install-systemd.sh` + `lopezpress.service` + `.env` 到 `/opt/lopezpress/`；
3. 在 VPS 上自动创建 `echoapi` 系统用户、装 systemd 单元、`enable` + `restart`；
4. 打印 `systemctl status` 与最近日志。

完成后：

```bash
ssh root@你的VPS_IP 'curl -s http://localhost:8080/health'
# {"status":"ok"}
```

**立即改 `.env` 里的 `JWT_SECRET`**：

```bash
ssh root@你的VPS_IP 'cd /opt/lopezpress && openssl rand -hex 32'
# 把输出粘进 .env，然后：systemctl restart lopezpress
```

## 手动部署（分步）

把 `deploy/lopezpress`、`deploy/.env.vps`、`deploy/install-systemd.sh`、`deploy/lopezpress.service` 分别放到 VPS 的 `/opt/lopezpress/` 下（`.env.vps` 改名为 `.env`），再在 VPS 上：

```bash
cd /opt/lopezpress
sudo bash install-systemd.sh
sudo systemctl status lopezpress
```

## 常用运维命令

```bash
sudo systemctl start|stop|restart|status lopezpress
sudo journalctl -u lopezpress -f          # 跟踪日志
sudo journalctl -u lopezpress -n 200      # 最近 200 行
sudo systemctl list-units --state=running | grep echo   # 是否运行中
# 看内存/CPU 占用（确认没爆 768M）
systemctl status lopezpress | grep -E 'Memory|CPU'
```

## 回滚 / 升级

升级只换二进制 + 重启，无需重建数据（SQLite 在 `data/app.db` 独立保留）：

```bash
# 开发机：新跑一次 deploy-to-vps.sh 即可，脚本会覆盖二进制并 restart
# 手动：scp bin/lopezpress root@VPS:/opt/lopezpress/ && \
#       ssh root@VPS 'systemctl restart lopezpress'
```

回滚到上一版二进制：保留 `bin/lopezpress.v<N>` 命名，`scp` 回去再 `systemctl restart`。

## 关于 HTTPS / Nginx

- **直接暴露 8080**（测试）：`curl http://VPS_IP:8080/health`。生产不建议。
- **加 Nginx 反代（推荐）**：把 `deploy/nginx.conf` 放到 `/etc/nginx/conf.d/lopezpress.conf`，改 `server_name` + 证书路径，`nginx -t && systemctl reload nginx`。Nginx 会把真实 IP 通过 `X-Forwarded-For` 透传，限流器据此做「每用户/每 IP」桶。

## 故障排查

| 症状 | 排查 |
| --- | --- |
| `systemctl status` 显示 OOMKilled | 内存超 `MemoryMax=768M`。调低 `DB_MAX_OPEN`（如 2），或把 `MemoryMax` 提到 900M（别超 1G 预算） |
| 启动后立即重启 | `journalctl -u lopezpress -n 100` 看 `.env` 路径/权限；确认 `User=echoapi` 能读写 `data/` |
| `health` 返回 500 | 数据库文件损坏或路径不可写；`ls -l /opt/lopezpress/data/app.db` |
| 限流误报 429 | 正常。客户端 IP 桶默认 `RATE_LIMIT_RPS=5 / BURST=10`，单 IP 连发 10 个以上会触发。调 `.env` 放宽后 `systemctl restart` |
| 升级后端口被占 | `ss -lntp | grep 8080`，确认旧进程没在跑；或改 `.env` 的 `SERVER_PORT` |

## 一句话

```
开发机跑一次 deploy-to-vps.sh  →  换 JWT_SECRET  →  reload nginx（可选）  →  完工
```

## 容器化部署（可选）

如需 Docker 化部署，见 [`docker/README.md`](docker/README.md)：一套 Docker Compose 方案——`api` 用 distroless 静态镜像（纯 Go 零 CGO），`web` 用 Nginx 托管前端 `dist` 并反代 `/api`、`/health`、`/uploads`。与上文 systemd 方案**并行可选**，不改业务代码；数据落命名卷，`docker compose up -d --build` 一键起。
