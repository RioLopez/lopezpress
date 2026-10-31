> 类别 A · 上线判定依据 — 回答"现在能不能上线"。细节已并入 `dev/ai-dev/MASTER.md` 第 5 节。

# 上线安全基线

> **这份文档只回答一个问题：现在能不能上线？** 不是"渗透报告修完了吗"。
> 2026-10-27 精简：原来按攻击面写的 60 多条防线清单已并入 `dev/ai-dev/MASTER.md` 第 5 节
> （那里按"权限/数据/上传/限流/配置/部署"重组，改代码时看那份更近）。
> **本文件只留三件事**：结论、还没做完的、明确接受的风险。

---

## 1. 一句话结论

**核心风险面已收敛，可以进入"有限公测"；离"无保留上线"还差两项**——
真实规格下的资源类复测（第 3.1 节）和日志告警（第 3.2 节）。

| 维度 | 状态 |
|---|---|
| 认证与会话 | 已收敛（JWT 锁算法、`token_version` 全场景撤销、分级锁定、图片验证码绑 IP） |
| 授权与权限矩阵 | 已收敛（root 唯一且不可经 API 创建、可见性单一权威 `canSeePost`、真私密） |
| 输入处理 | 已收敛（全路由统一体上限、字段全有界、上传重解码、密码策略收归一处） |
| 输出与 XSS | 已收敛（全分支转义、URL 政策按 scheme 解析、CSP） |
| 并发与数据一致性 | 已收敛（状态转换条件化、配额同事务、slug 唯一索引） |
| 备份与恢复 | 已演练（21 项往返，含恢复后旧 token 全失效） |
| 敏感数据 | 部分（SMTP 口令不回显、密码 bcrypt；**无静态加密**，靠磁盘加密兜底） |
| 依赖供应链 | 已扫描（46 个可达漏洞归零，门禁 `scripts/vuln_check.py`） |
| 可用性与可观测 | **部分**（有 `/health/ready` 就绪探针，**无告警**——见 3.2） |
| 部署加固 | 基本到位（systemd 沙箱、TLS、HSTS、CSP、点文件拒绝） |

---

## 2. 已挡住的攻击（回归清单，改完安全相关代码请重跑）

`scripts/defence_smoke.py` **39 项全过**（第 83 轮真机 HTTP，不是单测）：

- 25 MB 登录体 → 413；PNG+IEND 挂 payload 上传后 payload 被剥；24 MP 纯色图 → 400
- 伪造 XFF 换不到新限流桶；`public_read=false` 时 5 个匿名端点全部挡住
- 并发 5 个 approve 只有 1 个赢（`[200,409,409,409,409]`）

`scripts/backup_restore_drill.py` **21 项全过**（第 84 轮）。其中 `ceiling` 阶段是**对照实验**：
同一份 1.25 MiB 请求体，默认上限的路由 413、备份导入路由 200 —— 证明上限来自路由声明而非巧合。

密码策略 **10 项**（第 95 轮真机 `POST /api/auth/register`）：`12345678` / `password` /
`p@ssw0rd` / `aaaaaaaa` / 3 个汉字 / 25 个汉字 → 全部 400 且 `error` 各自点名原因；
含自己用户名 → 400；含邮箱前缀 → 400；`aG7!kLpq9` 与 `quiet-harbour-4821` → 201。

> **回归重点**：25 个汉字（75 字节）这一条。改之前它返回 **500**（bcrypt 拒收 >72 字节，
> 被当成哈希失败），现在是 400。如果哪天又变 500，说明上限守卫被绕过了。

---

## 3. 上线前还没做完的（就这两件）

### 3.1 真实环境资源类复测（阻塞）

前 6 轮报告的并发/资源类问题只能靠实测确认，必须在**与生产同规格**的机器（1C / 1.5 GB）上跑：

- 上传配额并发（10 并发 × 接近上限）
- 验证码取图配额（每 IP 每分钟 30 张，有失败收紧到 10）
- 图片解码峰值内存（24 MP 以内）
- 长连接 + 慢速请求（nginx `client_body_timeout` 已设 60s）

**做完的标志**：四个场景都有内存峰值与响应时间数字，且没有 OOM 或 502。

### 3.2 日志与告警（阻塞）

现在有 `slog` 结构日志，但**没有告警**。上线后至少要能回答三句话：
"刚才有 500 吗？""有人在爆破吗？""磁盘还剩多少？"

最小可行：`journalctl` + 一条定时 `grep`（5 分钟内 5xx 超 N 次发邮件），
或接 Uptime Kuma 打 `/health/ready`。**不需要 Prometheus**——单机 1.5 GB 装不下这套。

### 3.3 备份恢复要定期重跑（不阻塞，但有节奏）

`scripts/backup_restore_drill.py` 已通过一次，但**没做过恢复的备份不算备份**。
建议每季度 + 每次升级大版本前重跑，并在演练后核对 `snapshot-*.db` 确实生成。

两个设计事实（不是缺陷）：

- **备份 JSON 里没有图片本体**。图片在 `data/uploads/`，二进制塞进 JSON 不现实。
  完整恢复 = JSON 导入 **+** 单独同步 `data/uploads/`（`rsync` / `scp`）。
- **`public_read=false` 必须精确还原**，否则私密站恢复后变公开。round-trip 测试守着这条。

---

## 4. 已知残余风险（明确接受，不修）

| 风险 | 为什么接受 |
|---|---|
| CSP 含 `unsafe-inline` | 去掉需要重写全部内联样式与脚本，代价大于收益 |
| 数据库明文 | 单机 SQLite 无密钥托管方案；靠磁盘加密兜底 |
| 无 MFA | 目标用户是个人站长；`must_change_password` + 强密码 + 分级锁定已覆盖主要场景 |
| 正文允许任意 Markdown | 产品设计；XSS 防线在前端转义 + URL 政策 |
| `public_read=false` 时 `/uploads/` 仍可直链 | `img` 标签带不了 token；图片本身不是秘密 |
| root 可被锁定（60s 起） | 完全豁免会让锁定期内爆破速率放大 450 倍；60s 已足够 |

---

## 5. 怎么验证（可执行，不靠记性）

```bash
export GOSUMDB=sum.golang.org            # 本机默认 off，会让工具链下载失败

go test -count=1 ./internal/...
python scripts/arch_check.py             # 架构棘轮
python scripts/contract_check.py         # 前后端契约
python scripts/security_check.py         # S1–S9 安全反模式（棘轮，全 0）
python scripts/doc_check.py              # 文档没重新长胖
python scripts/vuln_check.py             # 依赖漏洞（0 = 可发布，2 = 扫描器缺失）

# 真机（需隔离实例，见脚本文档）：
python scripts/defence_smoke.py          # 39 项攻击用例
python scripts/backup_restore_drill.py   # 三阶段备份恢复
```

`security_check.py` 覆盖：路由 auth 等级自洽、体上限生效、**handler 自设上限不得超过路由声明（S9）**、
状态转换条件化、审计覆盖、字段上限、SQL 拼接、`template.HTML`、日志里的密钥。**棘轮：只降不升。**

> `cd web && npx vitest run` 要加 `--no-file-parallelism`，并行会漏文件。
