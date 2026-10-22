> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 业务逻辑反推(BUSINESS_LOGIC / as-built)

> 建立:2026-10-19 第 28 轮 | 方法:**从代码反推实际行为**,不是复述设计稿。
> 定位:本文档是项目业务逻辑的**实际实现权威描述**。与 `docs/ARCHITECTURE.md`(设计意图)、
> `docs/specs/`(验收标准)互补——当三者冲突时,以本文档描述的实际代码行为为准,并应修正另外两份。
> 反推来源:`internal/services/` + `internal/handlers/` 源码走读(函数级证据链见各节标注)。
> **审计疑点清单见文末 §4**——反推过程中发现的实现与设计偏差,按 severity 分级。

---

## 1. 内容域(posts)

### 1.1 文章五态状态机

状态定义(`models.go`):`published`(公开)/ `private`(仅作者+管理员)/ `pending`(审核队列)/
`rejected`(被打回,待作者修改)/ `banned`(封禁库,仅 root 可见)。

**创建入口**(`CreatePost`, services.go:538):

| 调用者角色 | 请求 status | 实际落地 status |
|---|---|---|
| root | "" / "published" / 其他任意值 | `published`(非法值归一化) |
| root | "private" | `private` |
| admin / user / 未知角色 | 任意 | `pending`(无条件进队列) |

**状态转换全表**(反推自 `UpdatePost` :841、`ReviewPost` :1029、`BanPost` :1111、
`SendBackPost` :1149、`DeleteBannedPermanently` :1242):

| 从 | 触发 | 到 | 谁可以 | 条件 |
|---|---|---|---|---|
| — | CreatePost | published/private/pending | 见上表 | — |
| published | UpdatePost(status=private) | private | 作者 / root / admin(编辑 user 文时) | 作者仅允许 published→private(P1-14 防逃队列) |
| published | UpdatePost(status=private) | private | root | 无条件 |
| pending | ReviewPost(approve) | published | admin(仅 user 的文)/ root | 状态前置:仅 pending/rejected 可审(第 35 轮 Q1 修复) |
| pending | ReviewPost(reject) | rejected | 同上 | note 必填(400) |
| rejected | UpdatePost(status=pending) | pending | **仅作者本人** | 重新提交闭环;清空 review_note |
| rejected | ReviewPost(approve/reject) | published/rejected | admin(仅 user 的文)/ root | 状态前置:rejected 仍可再审(admin 强制平反/再打回) |
| published/private/banned | ReviewPost(approve/reject) | — | — | **第 35 轮 Q1 修复:409「post is not awaiting review」**(published 不可被审核入口悄悄下线;banned 必须走 send-back) |
| published/private/pending/rejected | BanPost | banned | **仅 root** | 不可 ban root 自己的文章(400);已 banned 重复 ban → 409(第 35 轮) |
| banned | SendBackPost | rejected | 仅 root | note 必填;留在 normal flow |
| banned | ReviewPost(approve) | published | root | 无状态前置校验(Q1) |
| banned | DeleteBannedPermanently | 硬删除 | 仅 root | 先清 post_tags 关联行(P1-4) |
| 任意 | DeletePost | 软删(回收站) | 作者 / admin(仅 user 文)/ root | **banned 文章的软删未设防(疑点 Q2)** |

### 1.2 可见性矩阵(`canSeePost` services.go:639,单篇读取唯一权威)

| 文章 status | 匿名 | user(非作者) | 作者 | admin | root |
|---|---|---|---|---|---|
| published | ✅ | ✅ | ✅ | ✅ | ✅ |
| private | ❌ | ❌ | ✅ | ✅ | ✅ |
| pending | ❌ | ❌ | ✅ | ✅ | ✅ |
| rejected | ❌ | ❌ | ✅ | ✅ | ✅ |
| banned | ❌ | ❌ | ❌ | ❌ | ✅ |

列表作用域(`ListPostsPage` :740)与单篇略有差异——**admin 的列表不含 banned**(与 canSeePost 一致),
但审核队列 `ListPending`(:1216)按**可审范围**二次收窄:admin 只见 user 作者的 pending。
即:admin 单篇可读同行 admin 的 pending(canSeePost),但队列里看不到(疑点 Q3,设计使然,记录备查)。

### 1.3 创建 / 编辑 / 删除权限(`UpdatePost` :841 / `DeletePost` :971)

- **编辑**:作者本人 / root(任何人)/ admin(**仅作者为 plain user 时**)。他人 user → 403。
- **删除**:同上权限模型;软删进回收站(`ListTrash` :2026 / `RestoreTrash` :2054 / `EmptyTrash` :2074)。
- **字段语义**:title/body 非空才覆盖;category_id 为指针(nil=不改);cover 空串忽略(防局部编辑擦除);
  tags 为 nil 不改、非 nil 全量替换(删旧建新,同事务 P3-1)。
- **状态变更白名单**(反推 :873-909):只有 private / pending(rejected→pending,作者)/
  published(仅 root)三种目标可被设置,其余值静默忽略——tampered 客户端无法直接置 banned/rejected。

### 1.4 审核流(ReviewPost :1029)

- 入口:admin-or-root;user 作者的文 → admin 或 root 可审;**admin/root 作者的文 → 仅 root**(:1053)。
- 作者已被删除的孤儿文 → 仅 root 可审(:1050)。
- approve → published(清 note,记 reviewed_by/at,发邮件 best-effort);reject → rejected(note 必填)。
- **状态前置(第 35 轮 Q1 修复)**:仅当文章处于 `pending`/`rejected` 才可审;`published`/`private`/`banned` 调 ReviewPost 一律 409「post is not awaiting review」。理由:① published 不可经审核入口被悄悄下线;② banned 必须经过 SendBackPost→作者修改→重提交→再审,不能跳过作者修复直接平反。
- FIFO 队列(`ListPending` :1229,created_at ASC)。

### 1.5 封禁库(banned vault)

- `ListBanned`(:1196)仅 root;`BanPost`(:1111)仅 root 且不可自 ban;
  `SendBackPost`(:1149)banned→rejected(root,note 必填,发邮件);
  `DeleteBannedPermanently`(:1242)硬删(root,先清 post_tags)。
- 前端入口:AdminBanVaultPage(rootOnly,admin 看不到导航)。

### 1.6 列表 / 归档 / feed

- `ListPostsPage`:分页 page/size(1..100),status/q/category/tag/sort 过滤;
  pinned 优先,其次 created_at/view_count DESC(排序列白名单防注入 :807);
  搜索 LIKE + escapeLike 转义(:730)。匿名仅 published。
- `ListArchive`(:674):全量 published,剥 body 省流量,匿名可读。
- `FeedPosts`(:697)/`GetPostPublic`(:718):仅 published,view_count 原子自增。

---

## 2. 用户域(accounts / auth)

### 2.1 注册(`Register` services.go:330)

- **门**:站点级 `allow_register`(默认 false,单用户站;`POST /api/users` 管理端建号不受此门)。
- 校验:三项非空;密码 ≥8;`username OR email` 查重(409)。
- 角色:不显式设置,依赖 DB 默认 `default:'user'`(models.go:54)——**安全由 schema 保证**。
- 注册成功即发 JWT(自动登录)。

### 2.2 登录(`Login` services.go:383)

- `username OR email` 单字段登录(前端统一放 `email` 字段)。
- 顺序:查用户 → **disabled 403** → bcrypt 校验。注意:disabled 在密码校验**之前**返回,
  响应"account is disabled"可枚举账号状态(登录场景可接受,已记录)。
- 失败:login_log 记录(失败);发登录失败邮件(**15 分钟/user throttle** 防轰炸)。
- 成功:login_log + `last_login_ip/at` + 新地点通知(`notifyNewLocation`,IP 或 UA 变化)。
- 外层防护:`LoginLimiter`(IP + 账号双桶,5 次失败锁,middleware/login_limiter.go)独立于通用桶。

### 2.3 Token 与鉴权链(`SignToken` :438 / `Authenticate` :469)

- HS256;claims:`sub`(user id)/`role`/`ver`(token_version)/`exp`/`iat`;
  有效期 `cfg.JWTExpiryHours`(`.env` 显式 24h;代码 fallback 24)。
- **吊销机制**:`RevokeAllSessions` 把 `token_version+1`,所有旧 token 在下次请求时
  ver 不匹配 → 401("token revoked")。legacy 无 ver 的 token 仅在 version=0 时接受(首部署兼容)。
- **Authenticate 门链**(按序,任何一关失败即拒):

| # | 检查 | 失败 |
|---|---|---|
| 1 | 签名/过期(仅接受 HS256,`WithValidMethods` 防 alg 混淆) | 401 |
| 2 | user 存在 | 401 "user not found" |
| 3 | token_version 匹配 | 401 "token revoked" |
| 4 | **disabled**(第 29 轮新增,见 Q6) | 403 "account is disabled" |

- handler 层第二道门(两个中间件,handlers/auth.go):
  - `WithAuth`(:214):`must_change_password` 时仅放行白名单
    `/api/auth/me`、`change-password`、`logout`、`logout-all`(:248),其余 403。
  - `WithOptionalAuth`(:275):无 token 匿名放行;坏 token 401(不静默);
    `must_change_password` 时**降级为零值 user**(只给公开读 scope,不给个人数据)。

### 2.4 账号状态 × 访问矩阵(反推核心)

| 账号状态 | Login | WithAuth(持 token) | WithOptionalAuth(持 token) |
|---|---|---|---|
| 正常 | ✅ 发 token | ✅ | ✅ |
| **disabled** | 403 | **403**(第 29 轮 Q6 修复前:✅ 漏洞!) | **403**(同上) |
| must_change_password | ✅ 发 token | 白名单外 403 | 降级匿名 |
| token_version 旧 | ✅(发新 token) | 401 revoked | 401 revoked |

### 2.5 用户管理(`CreateUser` :264 / `UpdateUser` :332 / `DeleteUser` :476)

| 端点 | 规则(反推自 accounts.go) |
|---|---|
| CreateUser | 不受注册门;admin 只能建 user;root 可建 admin;**root 角色永不可建(403)**;未知角色 fallback user |
| UpdateUser | root 账户 role/disabled 全不可改(403);不可自改角色/自禁用;**admin 不能改/禁另一个 admin**;提升 admin 需 root;角色值白名单 user/admin(400) |
| ResetPassword | 走 `canManageUser`(root 不可被重置;admin 仅 plain user;防自) |
| ChangePassword | 本人;验旧密码;≥8;**清 must_change_password 标志**;发通知邮件 |
| RevokeAllSessions | token_version+1(自助全设备登出) |
| DeleteUser | 不可自删;root 不可删;admin 仅 root 可删;**最后一个 admin 保护**;名下有文章(含回收站,Unscoped 计数)409 |
| GetAuthor | 公开作者页 + 其 published 文章;**disabled 用户 404**(不暴露存在性) |

### 2.6 与 handler 的对应(router)

`/api/auth/register|login|logout|me|change-password|logout-all`、`/api/users*`(admin)、
`/api/me/stats`、`/api/authors/{username}`。登录路由双限流(LoginLimiter 内层 + 通用桶外层)。

---

## 3. 系统域(backup / upload / mail / 限流 / 审计)

### 3.1 备份导出(`ExportBackup` services.go:1627)

- 全表序列化:users(密码哈希经 `json:"-"` 单独 Scan 成 `password_hashes` map)、
  categories、tags、posts(**Unscoped 含软删**,保证备份可恢复回收站)、post_tags。
- 元数据:`schema_version` / `app_version`(导入门)/ `exported_at` /
  `security_note`(随文件走的"内含 bcrypt 哈希"警示,泄露也能警告读者)。
- 落盘 `backup-<UTC timestamp>.json`;返回文件名/大小/行数 + `warning`(同 security_note)。
- 权限:requireAdmin(只读操作,admin 可导出)。

### 3.2 备份导入(`ImportBackup` :1770)

- **root-only(第 30 轮 Q7 修复)**:payload 是攻击者可控 JSON,可携带任意用户行
  (含 role root + 外部密码哈希)——admin 可导入即等于"经 API 造 root",整条绕过 R2。
  service 层 `IsRoot(actor)` 守卫 + handler `requireRoot` + 前端对非 root 禁用。
- `validateImportPayload`:schema_version/app_version 门(0/空兼容旧 dump,400 拒绝不兼容)。
- dry-run:只校验报数,不碰 DB、不建快照。
- apply:先 `VACUUM INTO` 自动快照(保留 3 份)→ 事务内按依赖序清表
  (post_tags → posts → tags → categories → users;posts/users Unscoped 硬清)→ 逐行重建
  (posts Unscoped Create 保留 DeletedAt;密码哈希从 `password_hashes` map 回填)。

### 3.3 快照恢复(`RestoreSnapshot` :1907)

- **root-only(同 Q7)**:恢复=全库替换,共享导入的接管风险。
- 文件名防护:`filepath.Base` + `snapshot-*.db` 前后缀白名单(双重防穿越)。
- 只读打开快照(`?mode=ro`)→ **恢复前再自动快照一次**(可撤销)→ 事务内清表 + 从快照逐表复制。
- 恢复后 Count 四张表,错误不吞(防止"看起来像空库"掩盖问题)。

### 3.4 回收站(7 天恢复窗口)

- `ListTrash`(:2069):Unscoped + `deleted_at >= now-7d`,倒序。
- `RestoreTrash`(:2097):Unscoped 查询;`deleted_at` 无效 → 400;超 7 天 → **410 Gone**。
- `EmptyTrash`(:2117):Unscoped 硬清全部软删(不管年龄),返回删除数。

### 3.5 图片上传(`UploadImage` :2138)

- 双层大小封顶:handler `MaxBytesReader`(2 MB,config UPLOAD_MAX_BYTES)+ `ParseMultipartForm` 1 MB 内存 cap + `LimitReader` 读满即止。
- 扩展名白名单(jpg/png/webp/gif);文件名随机化(time + 8 hex + ext)防碰撞;本地落盘 0644,URL `/uploads/<name>`。
- 图床路由:`cfg.SmmsToken`(.env,服务端持有)**优先** > site-config token(DB,legacy 回退) > local。
  SM.MS 模式无 token → **503**(不静默落本地);SM.MS 失败 → 回退本地。
- 已知局限(记录):**不校验 magic bytes**(改扩展名的非图片可入库,靠 Content-Type 缓解,Q10);
  SM.MS 超时已治理(原 `http.DefaultClient` **无超时**,远端挂起会占 goroutine——**第 36 轮已修**:专用 `smmsClient` 带 10s 超时 + 绑定请求 ctx 取消,Q11)。

### 3.6 邮件通知(mail.go)

- 全链路 best-effort:SMTP 未配置/失败**从不**阻断业务操作(登录/改密/审核照常)。
- Throttle:登录失败通知 **15 分钟/user**(防爆破轰炸收件箱)。
- 三种模板:`MailLoginFailure`(失败通知)、`MailPasswordChange`(改密通知)、
  `MailReviewResult`(审核结果/打回,复用 reject 文案)。
- 新地点通知:`notifyNewLocation`(IP 或 UA 与上次登录不同则发信)。

### 3.7 双层限流

| 层 | 对象 | 策略 | 配置 |
|---|---|---|---|
| 通用桶 `Limiter` | 所有路由 | 按身份分桶(登录→user ID,匿名→IP,XFF 第一跳);令牌桶 20 rps / 50 burst;LRU 512 桶封顶;429 带 `Retry-After` | `.env` RATE_LIMIT_RPS/BURST,**改后须重启** |
| `LoginLimiter` | 仅 POST /api/auth/login | IP + 账号双桶 + **5 次失败锁**;janitor 每分钟清理 | 硬编码,不走 .env |

爆破防护靠 LoginLimiter,通用桶放宽不构成绕过(2026-10-15 调宽 5/10→20/50 的论证见 config.go 注释)。

### 3.8 审计日志

- `action_logs`(`RecordAction`)记录点:create_post / update_post / delete_post / review_post /
  ban_post / send_back_post / delete_banned / import_backup / restore_snapshot。
- `login_logs`(`RecordLogin`):每次登录尝试(成功/失败)+ IP + UA;`last_login_ip/at` 随成功登录更新。
- 前端「登录日志」页(spec 14)按用户查看。

---

## 4. 审计疑点清单(反推发现,待分级处理)

| # | severity | 位置 | 问题 | 拟处置 |
|---|---|---|---|---|
| **Q2** | 🔴 P1 | `UpdatePost` :850 / `DeletePost` :979 | **作者可编辑/软删自己已 banned 的文章**——违背"banned 仅 root 管"的设计;且作者软删后 root 的 `DeleteBannedPermanently` 会 404(First 带软删 scope),封禁数据永久残留于 vault 之外 | ✅ **已修(第 28 轮)**:`UpdatePost` 对 banned 文章非 root 一律 403(root 保留编辑权);`DeletePost` 对 banned 文章**所有角色** 403(banned 只能经 vault 的 send-back/永久删出口);回归 `TestBannedPostFrozenForAuthor` 6 断言全绿 |
| **Q6** | 🔴 P1 | `Authenticate` services.go:469 | **disabled 账号在 Authenticate 无门**——Login 拦禁用,但已持有的 JWT(最长 24h)仍可调用所有 API(含 WithAuth 写操作);admin 禁用恶意账号后其会话当天仍可用。must_change_password 有门而 disabled 没有,同类状态只做一半(与 P0-1 同构的"设计了检查漏落地") | ✅ **已修(第 29 轮)**:`Authenticate` token_version 校验后加 `if user.Disabled → 403`,一处修覆盖 WithAuth/WithOptionalAuth 两个中间件;回归 `TestAuthenticateRejectsDisabledUser`(禁用 403 / 重新启用同 token 恢复) |
| **Q8** | 🟢 P3 | `UpdateUser` :379 | disabled 分支不对称:admin **不能禁用**另一个 admin,但**能启用**另一个 admin(`*in.Disabled &&` 条件只在禁用时拦) | 记录;启用危害低于禁用,暂缓 |
| **Q9** | 🟢 P3 | `DeleteUser` :495 | last-admin 保护只数未软删 admin;被软删的 admin 占名额但不活跃(无用户恢复端点,影响有限) | 记录 |
| **Q7** | 🔴 P0 | `ImportBackup` :1770 / `RestoreSnapshot` :1907(handler 仅 requireAdmin) | **备份导入/恢复是完整的 root 接管路径**:payload 是攻击者可控 JSON,`tx.Create(&u)` 不校验角色——admin 导入一份把自建用户设为 root(或替换 root 的 password_hash)的备份即可接管站点,直接绕过"root 不可经 API 创建"红线(R2)。恢复快照同理替换全库 | ✅ **已修(第 30 轮)**:service 层 `IsRoot(actor)` 守卫(ImportBackup/RestoreSnapshot 加 actor 参数)+ handler 改 `requireRoot` + 前端导入区对非 root 禁用并说明;回归 `TestImportAndRestoreAreRootOnly`(admin 403/root 放行/恶意 root 行未落库) |
| **Q10** | 🟢 P3 | `UploadImage` :2159 | 只校验扩展名白名单,不校验 magic bytes——改扩展名的非图片可入库;靠 Content-Type(image/png)缓解,存储型 XSS 需服务端错误配置才触发 | 记录;如接 Nginx 确保 uploads 目录按扩展名回 Content-Type |
| **Q11** | 🟢 P3 | `uploadToSmms` :2285 | SM.MS 代理走 `http.DefaultClient` **无超时**——远端挂起会占住 goroutine(低配机几个就紧) | ✅ **已修(第 36 轮)**:`uploadToSmms` 改用专用 `smmsClient`(`http.Client{Timeout:10s}`)+ `http.NewRequestWithContext(ctx,...)`,既绑请求 ctx(客户端断开即取消)又有硬超时兜底;调用处透传 `UploadImage` 的 ctx |
| **Q12** | 🟢 P3 | `pruneSnapshots` :1874 | 快照只保留 3 份;连续恢复/导入会挤掉最旧快照(边缘:恰好想回退到被挤掉的那一份) | 记录;keep=3 是 spec #6 定稿 |
| **Q1** | 🟡 P2 | `ReviewPost` :1061 / `BanPost` :1143 | 审核/封禁入口**无状态前置校验**:已 published 文章可被 reject 突然打回(公开内容无确认消失);banned 可被 approve 直接平反跳 published(绕过 send-back 的作者修改环节) | ✅ **已修(第 35 轮)**:`ReviewPost` 仅 `pending`/`rejected` 可审,其余状态 409;`BanPost` 已 banned 重复 ban → 409。设计取舍:不允许"重新审核已发布文章"(公开内容不可经审核入口被悄悄下线),banned 必须走 `SendBackPost`→作者修改→重提交→再审 |
| **Q3** | 🟢 P3 | `canSeePost` :648 vs `ListPending` :1226 | admin 单篇读取作用域比队列宽:可凭 ID 读同行 admin 的 pending,但队列不可见。轻微信息暴露,admin 本为可信角色 | 记录,不修 |
| **Q4** | 🟢 P3 | `ListPending` JOIN users | 作者被软删后,其 pending 文章从所有审核队列消失(孤儿文无人可审,仅 root 凭 ID 可达) | 记录;与 DeleteUser 孤儿策略(P1-3 已防硬删孤儿)联动评估 |
