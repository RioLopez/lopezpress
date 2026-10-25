> 类别 B · 项目/内容/参考 — 部署手册，部署者与运维按需查阅。

# 部署手册

这是**面向部署者**的完整部署入口。具体配置与脚本在 `deploy/`，本文档负责「从零到上线」的步骤、环境要求、和上线前检查清单。

## 1. 目标环境

| 维度 | 要求 | 说明 |
|---|---|---|
| CPU / 内存 | 1 vCPU / 1.5 GB（可更低） | 全项目按 ≤70% 资源预算设计 |
| 磁盘 | ≥ 5 GB | SQLite 单文件 + 上传图片 + 备份快照 |
| 系统 | 任意 Linux（建议 Debian/Ubuntu） | systemd 部署；Docker 亦可 |
| 依赖 | **无** | 纯 Go 静态二进制（`CGO_ENABLED=0`）+ SQLite，无运行时依赖 |

## 2. 两种部署方式

| 方式 | 入口 | 适用 |
|---|---|---|
| **systemd（推荐）** | `deploy/README.md` + `deploy/install-systemd.sh` | 裸机 / VPS，资源占用最低 |
| **Docker Compose** | `deploy/docker/` | 需要容器化或隔离 |

一键脚本：本地 `deploy/deploy-to-vps.sh`（交叉编译 → scp → VPS 上安装并启用）。

## 3. 部署前必须配置的环境变量

复制 `deploy/lopezpress.env` 到生产，至少改这三项，**否则生产模式会拒绝启动**：

```bash
APP_ENV=production                     # 生产严格校验
JWT_SECRET=<openssl rand -base64 32>   # 必须强随机、≥32 字节；留空或弱值会拒启
CORS_ORIGINS=https://blog.example.com  # 显式来源列表，禁止 *
ADMIN_INITIAL_PASSWORD=<首启密码>      # 可选；留空则自动生成并打印一次
```

> 其余变量（DB_PATH / UPLOAD_DIR / BACKUP_DIR / 限流 / 配额）见 `deploy/lopezpress.env` 内注释。

## 4. 上线前检查清单（逐条过）

```bash
# 1. 健康检查：liveness（进程活着）与 readiness（数据库可用）
curl -fsS http://localhost:8080/health          # 应 200 {"status":"ok"}
curl -fsS http://localhost:8080/health/ready    # 应 200 {"status":"ready"}
# 注意：-healthcheck 子命令等价于打 /health/ready，供容器 HEALTHCHECK 用

# 2. 安全响应头与体上限
curl -sI https://blog.example.com | grep -iE 'x-frame|x-content-type|content-security|strict-transport'
# 上传超限应 413（不是 500）

# 3. 备份可恢复性：至少做过一次「导出 → 空库导入 → 验证」演练
#    python scripts/backup_restore_drill.py populate|restore|ceiling

# 4. 安全反模式门禁
python scripts/security_check.py
```

## 5. 上线前仍阻塞的三项（见 `docs/SECURITY_BASELINE.md` §6）

1. **依赖漏洞扫描**：`govulncheck ./...`（需 Go 1.26 + 联网，本机未跑）。
2. **真实规格压测**：在 1C/1.5G 真机上复测并发上传 / 验证码预算 / 解码峰值内存。
3. **日志与告警**：有 `/health/ready`，但没有「5xx 突增 / 爆破 / 磁盘将满」的告警。

## 6. 数据与备份

- 数据库：`data/app.db`（SQLite，WAL 模式）。
- 图片文件：`data/uploads/`（**不在 JSON 备份内**，需单独 `rsync`/`scp` 备份）。
- JSON 备份：`data/backups/`（含 users/posts/站点配置/修订史/图库元数据）。
- 快照：`data/backups/snapshot-*.db`（`VACUUM INTO` 文件级快照，可整库回滚）。
