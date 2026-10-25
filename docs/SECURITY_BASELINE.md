> 类别 A · AI 指挥/约束（母文档族）— 由根目录 `AGENTS.md` 统管，新对话须先读。

# 上线安全基线（2026-10-25 第 84 轮建立）

> **这份文档回答一个问题：现在能不能上线？** 不是"渗透报告修完了吗"。
>
> 渗透报告是抽样。前 8 轮里同一个类别的漏洞反复以不同入口出现（见 §2），
> 说明"等报告再修"这条路走不通。这份基线按**攻击面**组织，而不是按报告编号，
> 每一项都写清楚"防住了什么 / 靠什么防 / 怎么验证 / 还剩什么"。
>
> 维护规则：任何一项的结论变了就改这里；新增防线要在 §3 加一行，否则下一轮
> 没人知道它存在。

---

## 1. 一句话结论

**核心风险面已收敛，可以进入"有限公测"；离"无保留上线"还差 §6 的三项（依赖扫描、真实规格压测、告警）。**

备份恢复演练已于第 84 轮完成（§6-3），并在演练中当场抓到一个数据丢失级缺陷
（路由体上限默认 1 MiB 使备份无法恢复），已修 + 加门禁 S9。

具体地说：

| 维度 | 状态 |
|---|---|
| 认证与会话 | ✅ 已收敛（JWT 锁算法、token_version 全场景撤销、分级锁定） |
| 授权与权限矩阵 | ✅ 已收敛（root 唯一、admin 审核者不是作者、可见性单一权威） |
| 输入处理 | ✅ 已收敛（全路由统一体上限、全部字段有界、上传重编码） |
| 输出与 XSS | ✅ 已收敛（全分支转义、URL 政策按 scheme 解析、CSP） |
| 并发与数据一致性 | ✅ 已收敛（状态转换条件化、配额同事务、slug 唯一索引） |
| 备份与恢复 | ✅ 已演练（21 项，含两个实例的完整往返） |
| 敏感数据 | ⚠️ 部分（SMTP 不回显、密码哈希、但无静态加密） |
| 依赖供应链 | ⚠️ **未完成**（见 §6-1） |
| 可用性与可观测 | ⚠️ 部分（有就绪探针，无指标/Prometheus/告警，见 §6-4） |
| 部署加固 | ✅ 基本到位（systemd 沙箱、TLS、HSTS、CSP） |

---

## 2. 为什么这份文档按攻击面写，不按报告写

同一个类别的漏洞，连续三轮从不同入口冒出来：

| 类别 | 报告点名的入口 | 实际存在的其他入口 |
|---|---|---|
| 请求体无上限 | R3-05 login | 另外 18 个 handler（第 84 轮才发现） |
| 字段无长度上限 | R3-07 title/body、R4-10 导入器 | tags、分类 CRUD、cover、bio、display_name、review_note（第 83 轮才发现） |
| 状态转换竞态 | R4-11 approve‖delete | 所有 8 个转换点（第 82 轮补齐） |

结论：**修报告 ≠ 修类别**。所以本文档的每一项都要求"全入口覆盖"，并在
`scripts/security_check.py` 里能用机器验的部分做成门禁。

---

## 3. 防线清单（按攻击面）

### 3.1 认证与会话

| 防线 | 实现 | 验证 |
|---|---|---|
| JWT 算法固定 | `jwt.WithValidMethods(["HS256"])` + 显式检查 `t.Method.Alg()`，无 `kid` | `internal/services/auth.go` `parseToken` |
| 签名密钥无默认值 | `JWT_SECRET` 为空时：dev 生成随机并缓存（0600），prod **拒绝启动** | `config.EnsureJWTSecret` + `jwt_secret_test.go` |
| 撤销语义统一 | 改密 / admin reset / logout-all / **禁用账号** / 恢复导入 全部 bump `token_version` | `services/token_epoch.go` |
| 恢复不复活旧 token | 恢复后抬到 `liveMax + 1000 + 随机`（`+1` 会正好等于"快照后、logout-all 前"的版本） | `tokenRevokeFloor` |
| 登录限流 | IP 桶 + 账号桶 + 失败锁定（**60s → 5min → 15min 分级**） | `middleware/login_lockout.go` |
| 锁定不可武器化 | 首档 60 秒，避免匿名者用错口令把管理员锁 15 分钟 | `TestLockoutEscalatesAndResetsOnSuccess` |
| 密码存储 | bcrypt（cost 见 `services/auth.go`） | — |
| 首登强制改密 | `must_change_password` 拦在 `RequirePasswordChanged` | 实测：不改密时所有写请求 403 |

**残余**：无 MFA；无设备管理（"我登录过哪些设备"）；token 无刷新机制（1h 过期需重登）。

### 3.2 授权与权限矩阵

| 防线 | 实现 |
|---|---|
| root 全库唯一 | 任何 API 都不能创建/提升 root（`CreateUser`/`UpdateUser` 遇 root 直 403） |
| 唯一权威可见性判定 | `services/visibility.go` `canSeePost`；列表过滤 `postsOnly` 切片必须与之同步 |
| 审核者不是作者 | admin 改他人 `title`/`body`/`cover`/`slug` → 403；保留审核动作 |
| 修订读写分离 | 读 `assertCanReadHistory`（作者/admin/root）、写 `assertCanWriteContent`（作者/root） |
| 字段级授权 | 非 root 不能改 `auto_backup`/`allow_register`/SMTP/图床 token（**不是路由守卫，必须留**） |
| 路由级权限声明 | 权限在 `var routes` 表上，不放 handler |
| 备份面 root-only | 导出/下载/导入/恢复全部 `rootRoles` |

**残余**：`public_read` 关闭时，`/uploads/` 仍匿名可读（img 标签带不了 token，有意为之，但意味着"只读站点"的图片仍可被直链访问）。

### 3.3 输入处理

| 防线 | 实现 |
|---|---|
| **全路由统一体上限** | `route.maxBody`（默认 1 MiB，上传 16 MiB）；`Content-Length` 超限**读前即 413**，chunked 由 `MaxBytesReader` 兜底 |
| 字段长度集中声明 | `services/limits.go`：标题 500 / 正文 256KiB / 审计 512 / 分类标签名 64 / **tags 每篇 20 个** / **review_note 2000** / **bio 2000** / **display_name 64** / **cover 500** |
| URL 政策按 scheme 解析 | `ValidateCoverURL`（`/uploads/` 或 http(s)）、`validateAvatarValue`；**拒绝 `//host`** |
| 上传内容验证 | 魔数 → `DecodeConfig` 几何 → **完整 `image.Decode` 后从位图重编码**（剥掉 IEND 之后字节）→ 压缩比守卫（≥4MP 且 <1B/200px 拒） |
| 上传顺序 | 去重 → 配额（**同事务**）→ 写盘 |
| SQL 注入 | 全部参数化；`security_check.py` S6 拦字符串拼接 |
| 路径穿越 | 文件名 `crypto/rand` 生成；`sanitizeOriginalName` 走 `filepath.Base` |

**残余**：正文允许任意 Markdown（设计如此）；`UPLOAD_MAX_BYTES` 与 `upload_max_image` 两个上限并存，前者是环境变量、后者是后台设置，容易混淆。

### 3.4 输出与 XSS

| 防线 | 实现 |
|---|---|
| 全分支转义 | `highlight()` **含空查询也先 `escapeHTML`**（它输出给 `dangerouslySetInnerHTML`） |
| URL 渲染政策 | `isSafeHref` / `isSafeSrc` / `isSafeSrcset`；`//host` 一律拒 |
| CSP | nginx 注入；`script-src 'self' 'unsafe-inline'` |
| 响应头 | `nosniff` / `X-Frame-Options DENY` / `Referrer-Policy` / HSTS |

**残余（明确接受）**：CSP 里的 `'unsafe-inline'` 去不掉，因为 `index.html` 有一个防主题闪烁的 inline script。取舍：改成 hash 会在前端改动时静默失效（后果是页面被拦，比闪烁更糟）；改用外部文件会引入额外请求，仍有闪烁窗口。**真正的防线是 §3.4 前两条，CSP 只是最后一层。**

### 3.5 并发与数据一致性

| 防线 | 实现 |
|---|---|
| 状态转换条件化 | `transitionPost`：期望状态进 `WHERE`，`RowsAffected==0` → 409 |
| 编辑不撤销审核 | `UpdatePost` 只写作者字段，`status` 走独立条件转换 |
| 编辑不覆盖浏览量 | 同上（整行 `Save` 会把读到的旧 `view_count` 写回） |
| 配额原子 | 检查 + 插入同事务；DSN `_txlock=immediate` |
| slug 唯一 | 迁移 0014 partial UNIQUE 索引 + 插入冲突重试后缀 |
| 恢复后台账一致 | `rebuildUploadRefsTx`（批量，四语句与文章数无关） |
| 分页溢出 | `maxPageNumber` 前置校验（负 OFFSET 会被 SQLite 当 0） |

### 3.6 敏感数据

| 项 | 状态 |
|---|---|
| 密码 | bcrypt 哈希，不可逆 |
| SMTP 口令 / 图床 token | **任何角色（含 root）读接口都不回显**；空值语义为"保持不变" |
| 审计 detail | `auditDetail` 统一截断，rune 边界安全 |
| 登录日志 | 记 IP + UA，仅本人与 root 可读 |
| **数据库文件** | ⚠️ 明文（SQLite）。备份 JSON 含 `password_hashes` |

**残余**：无静态加密。对一个单机 SQLite 站点，这是可接受的取舍（密钥放哪？），但**必须**配合：磁盘加密（LUKS）、`data/` 目录 0700、备份文件 0600。

### 3.7 部署与运维

| 项 | 状态 |
|---|---|
| systemd 沙箱 | `NoNewPrivileges` / `ProtectSystem=strict` / `ProtectHome` / `PrivateTmp` / `SystemCallFilter` / `ReadWritePaths` 白名单 |
| 以非 root 运行 | `User=echoapi`；Docker `USER 65532` |
| TLS | 仅 TLSv1.2/1.3；`:80` 301 跳转 |
| 请求体上限 | nginx `client_max_body_size 32m`（**必须 ≥ 后台的 `upload_max_image`，否则上传在 nginx 就被 413**） |
| 版本隐藏 | `server_tokens off` |
| 优雅关闭 | SIGTERM → 30s 排空 |
| 健康检查 | `/health`（liveness，**不碰 DB**）+ `/health/ready`（readiness，探 DB）+ 二进制 `-healthcheck`（distroless 镜像用） |

**为什么 liveness 不能碰 DB**：磁盘满或 DB 损坏时重启进程解决不了任何问题，而
"liveness 失败 → 编排系统重启"会把降级变成重启风暴。

---

## 4. 门禁（可执行的，不靠记性）

```bash
go build ./... && go vet ./...
go test -count=1 ./...                 # 后端
python scripts/arch_check.py           # 架构棘轮
python scripts/contract_check.py        # 前后端契约
python scripts/security_check.py       # 安全反模式（S1–S9，棘轮）
python scripts/doc_check.py            # 母文档自洽
cd web && node node_modules/typescript/bin/tsc --noEmit
cd web && npx vitest run --no-file-parallelism   # 必须串行，并行会漏文件
```

`security_check.py` 覆盖：路由 auth 等级自洽、体上限生效、**handler 自设上限不得超过
路由声明（S9）**、状态转换条件化、审计覆盖、字段上限、SQL 拼接、`template.HTML`、
日志里的密钥。它是**棘轮**：基线只降不升（当前 S1–S9 全 0）。

真机验证：
- `scripts/defence_smoke.py`（39 项攻击用例，需隔离实例，见其 docstring）
- `scripts/backup_restore_drill.py`（三阶段备份恢复演练，见 §6-3）

---

## 5. 已验证的攻击被挡住（真机 HTTP，不是单测）

39 项全过（第 83 轮），含：25MB 登录体 → 413、PNG+IEND 挂 payload 上传后
payload 被剥、24MP 纯色图 → 400、伪造 XFF 换不到新桶、`public_read=false`
时 5 个匿名端点全挡、并发 5 个 approve 只 1 个赢（`[200,409,409,409,409]`）。

备份恢复 21 项全过（第 84 轮），见 §6-3。其中 `ceiling` 阶段是**对照实验**：
同一份 1.25 MiB 请求体，默认上限的路由 413、备份导入路由 200 —— 证明上限来自
路由声明而非巧合。

---

## 6. 上线前必须做完的事

### 6-1 依赖漏洞扫描（**未完成**，阻塞）

`govulncheck` 需要 Go 1.26 且校验库联网，本机装不上。上线前在有网环境执行：

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
cd web && npm audit --omit=dev    # 本机已验证：0 vulnerabilities
```

已知需要盯的依赖：`golang.org/x/image`（图片解析 CVE 高发）、`golang.org/x/crypto`、
`golang-jwt/jwt/v5`、`glebarez/sqlite`（SQLite 版本跟随）。

### 6-2 真实环境下的资源类复测（阻塞）

前 6 轮报告的并发/资源类问题只能靠实测确认。必须在**与生产同规格**的机器上跑：

- 上传配额并发（10 并发 × 接近上限）
- 验证码预算（每 IP 10 封/小时）
- 图片解码峰值内存（24MP 以内）
- 长连接 + 慢速请求（nginx `client_body_timeout` 已设 60s）

### 6-3 备份可恢复性（**已完成一次演练**，仍需定期重跑）

演练脚本：`scripts/backup_restore_drill.py`（三阶段 `populate` / `restore` / `ceiling`，
用法见文件头）。**第 84 轮实跑结果：21 项断言全过。**

演练做了什么（不是单测能覆盖的部分）：

| 阶段 | 验证内容 |
|---|---|
| `populate` | 源站建图 + 文章 + 8 篇批量正文，导出后**下载**（运营动线是「导出→下载→上传」，不是导出即得正文） |
| `restore` | 空库实例：坏 `schema_version` → 400 提前拒；缺 confirm 短语 → 拒；dry-run → 200；正式导入 → 200；恢复后**源站口令可用、两个恢复前口令失效**；**恢复前的 token 全部 401**；文章标题/正文/图片引用完整；19 篇文章全列；**图片台账 `ref_count` 已按恢复后正文重建**；`/health/ready` 200 |
| `ceiling` | 同一个 1.25 MiB 请求体：默认 1 MiB 的路由拒（413）、`/api/backup/import` 接受（200）——证明导入的体积上限来自路由声明而不是运气 |

**这次演练当场抓到一个数据丢失级缺陷**：统一请求体上限（本轮为堵内存耗尽而加）
默认 1 MiB，而 `/api/backup/import` 的 handler 自己写了 20 MiB。路由层先跑，
**小的赢**，于是 handler 的上限变成死代码 —— 导出的 1.25 MiB 备份无法恢复。
也就是说：站上有一堆备份，没有一份能用。已修（路由显式声明 20 MiB / 24 MiB /
24 MiB），并加了门禁 **S9**：handler 自设上限大于路由声明 → 构建失败。

> 待办：把这三阶段做成**上线后定期执行**（建议每季度 + 每次升级大版本前），
> 并在演练后核对 `snapshot-*.db` 确实生成。没做过恢复的备份不算备份。

**第 86 轮补的完整性问题**：备份曾只导出 `users/categories/tags/posts/post_tags`，
**不含站点设置 / 修订史 / 图库元数据** —— 导出→导入还原不出站点。现已补齐
`site_configs` / `revisions` / `upload_files` 三个表，并用 round-trip 测试守住
（含一个安全关键点：`public_read=false` 必须精确还原，否则私密站恢复后变公开）。

**仍需人工处理的部分（设计如此，非缺陷）**：备份 JSON 里**没有图片文件本体**——
图片字节在 `data/uploads/` 磁盘上，二进制塞进 JSON 不现实。所以：
- 导出响应的 `warning` 已明确说明这一点；
- 完整恢复 = JSON 导入 **+** 单独备份/拷贝 `data/uploads/` 目录（`rsync` / `scp`）。

### 6-4 日志与告警（阻塞）

现在有 `slog` 结构日志，但**没有告警**。上线后至少要能回答：
"刚才有 500 吗？""有人在爆破吗？""磁盘还剩多少？"

最小可行：`journalctl` + 一条 `grep` 定时任务（5 分钟内 5xx 超过 N 次就发邮件），
或接 Uptime Kuma 打 `/health/ready`。

---

## 7. 已知残余风险（明确接受，不修）

| 风险 | 为什么接受 |
|---|---|
| CSP 含 `unsafe-inline` | 见 §3.4；去掉的代价大于收益 |
| 数据库明文 | 单机 SQLite 无密钥托管方案；靠磁盘加密兜底 |
| 无 MFA | 目标用户是个人站长；`must_change_password` + 强密码 + 分级锁定已覆盖主要场景 |
| 正文允许任意 Markdown | 产品设计；XSS 防线在前端转义 + URL 政策 |
| `/uploads/` 在 `public_read=false` 时仍可直链 | img 标签带不了 token；图片本身不是秘密 |
| root 可被锁定（分级 60s 起） | 完全豁免会让锁定期内爆破速率放大 450 倍；60s 已足够 |

---

## 8. 怎么用这份文档

- **改动前**：看 §3 找到对应攻击面，确认你要动的地方有防线。
- **改动后**：跑 §4 全部门禁。`security_check.py` 变红就是引入了新债。
- **上线前**：逐条勾 §6。四项都做完之前，只能说"可以有限公测"。
- **被渗透后**：按 §2 的方式归类，修**整类**而不是那一个入口，然后在 §3 记一行。
