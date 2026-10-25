> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 37 — 邮箱验证（注册 / 找回密码 / 换绑邮箱）

> 状态：Accepted（**实验性功能，默认关闭**）
> 路由：`/register`、`/login`→`/forgot-password`、`/settings`；后台 `/admin/site`
> 权限：三条流程本人；开关 root 专属
> 轮次：第 79 轮
> 最后更新：2026-10-24

## 1. 定位

给「注册」「忘记密码」「换邮箱」三件事加一次性验证码。用户原话是「全套都做好吧 不过同样的 属于测试功能 目前默认关闭 并且提示是测试 然后呢 管理员也可以开启和关闭之类的」。

三条硬约束，按重要性排序：

1. **默认关闭，且必须能一键关回去**。开启后站点行为会变（注册要多填一个码），所以它不能是"装好就开"的东西。
2. **界面必须标明是测试功能**。前端在注册表单和后台开关旁各挂一个 `测试功能 / Beta` 标签，写明「管理员可随时关闭」。
3. **不能出现"看起来成功了其实没发信"**。SMTP 没配时接口返回 503 并说清楚原因，而不是 200 完事。

## 2. 威胁模型

六位数字只有 100 万种，安全性全靠外围约束：

| 风险 | 约束 | 代码位置 |
|---|---|---|
| 猜码 | 单码最多 5 次，超了作废（`UsedAt` 置值，无法再用） | `email_codes.go` `verifyEmailCode` |
| 重放 | 一次性 + 绑定用途，注册码不能拿去重置密码 | `Purpose` 字段参与 HMAC |
| 枚举 | 请求验证码永远同一句回答，与邮箱是否注册无关 | `handlers/auth_email.go` |
| 库被拖走 | 库里只存 HMAC（key = `JWT_SECRET`），不是裸 sha256 —— 否则六位数就是一张 10^6 彩虹表 | `Service.hashCode` |
| 刷别人邮箱 | 同邮箱 60s 冷却（可配）+ 同 IP 每小时 10 封 | `ensureCodeCooldown` / `allowCodeFromIP` |
| 绕过验证 | 开着验证时 `PUT /api/auth/me` 改邮箱直接 403 | `accounts_profile.go` `applyIdentityChange` |

最后一条是本轮最容易漏的：前端表单不显示邮箱输入框**不等于**服务端安全，加这条守卫前，一次 PUT 就能跳过整套验证码。

## 3. 数据模型

`email_codes`（新表，`models/email_codes.go`）：

| 字段 | 说明 |
|---|---|
| `email` / `purpose` | 联合索引 `idx_email_codes_lookup`，用途取 `register` / `reset_password` / `change_email` |
| `code_hash` | HMAC-SHA256(`email|purpose|code`, JWT_SECRET)，不落明文 |
| `user_id` | 换绑时记录发起人；注册/找回为空 |
| `attempts` | 错一次 +1，到 5 作废 |
| `expires_at` / `used_at` / `ip` / `created_at` | 过期、消费、溯源；行保留 24h 供追溯，之后由 `pruneEmailCodes` 清掉 |

`site_configs` 新增 3 列（迁移 `0013_add_email_verify`）：

- `email_verify_enabled`（默认 0）
- `email_code_ttl_min`（默认 10，clamp 2–60）
- `email_code_cooldown`（默认 60 秒，clamp 30–600；若 ≥ TTL 则折半，否则验证码会在能重发之前就过期）

## 4. 三条流程

**注册**：`POST /api/auth/email-code {email, purpose:"register"}` → 收信 → `POST /api/auth/register {..., code}`。
校验顺序是「查重名 → 校验码 → 建号」：先查重名，避免一个注定失败的注册白白烧掉用户等来的码。

**找回密码**：发码（purpose `reset_password`）→ `POST /api/auth/password/reset {email, code, password}`。
成功后 `token_version` +1，**所有会话失效**——用户来重置密码通常是因为号可能被盗，留着旧 token 等于白重置。旧密码立即失效。

**换绑邮箱**：登录态 `POST /api/me/email-change/request {email}` → 码发到**新**地址（收到码本身就是所有权证明）→ `POST /api/me/email-change/confirm {email, code}`。
成功后给**旧**地址发一封通知（`MailEmailChanged`）：万一不是本人操作，旧邮箱是唯一还能发现的地方。用户名和当前会话都不受影响（JWT 里是 user id，不是邮箱）。

## 5. 端点与权限

| 方法 | 路径 | 权限 |
|---|---|---|
| POST | `/api/auth/email-code` | 公开（`purpose` 只接受 register / reset_password） |
| POST | `/api/auth/password/reset` | 公开 |
| POST | `/api/me/email-change/request` | 登录 |
| POST | `/api/me/email-change/confirm` | 登录 |
| GET | `/api/site-config` | 公开，新增 `email_verify_enabled`（表单要知道该不该渲染验证码框） |

换绑的码**不允许**从匿名接口申请（`change_email` 走公开端点直接 403）：否则攻击者可以先给受害者邮箱预埋一个码，等拿到会话后直接用。

后台开关是 **root 专属**，和 `allow_register`、`smtp_*` 同一层。理由不是"权限洁癖"：开着的开关意味着任何匿名访客都能让这台 1C 机器往外发信，这是滥用面而不是展示偏好。普通 admin 保存设置时这三个字段一律继承库值。

## 6. 前端

| 位置 | 变化 |
|---|---|
| `/register` | 开关打开时多一行「验证码 + 发送」，60 秒倒计时；带 `测试功能` 标签 |
| `/login` | 新增「忘记密码？」（验证关闭时点进去会明确说"本站未开启"） |
| `/forgot-password`（新） | 邮箱 + 码 + 新密码，成功跳登录 |
| `/settings` | 开关打开时邮箱变成「当前地址 + 更换」面板，改完立即刷新；关闭时保持原来的可编辑输入框 |
| `/admin/site` | 新增「邮箱验证」区块：总开关 + 有效期 + 重发间隔，非 root 只读 |

## 7. 测试与验收

- **Go 单测 +9**（`email_codes_test.go`）：默认关闭 403 / 无 SMTP 503 / 冷却 429 / 5 次作废后正确码也失效 / 过期 / 注册必须有码且码不可复用 / 重置后 token_version 与新旧密码 / 换绑地址不匹配被拒 / `PUT /api/auth/me` 不能绕过。
- **端到端 28/28**（隔离实例 `:18083` + 数据库副本 + **本地假 SMTP**）：真收信、真取码、真注册；含"码死后正确码也无效""普通 admin 关不掉开关""关掉开关后注册不再要码"。
  假 SMTP 留在 `scripts/fakesmtp.py`，`python scripts/fakesmtp.py <输出目录> 2525` 即可复用——它故意不实现 STARTTLS，让 `smtpSend` 走明文分支。
- **门禁**：`arch_check` 一度报 R2.files 1→3 / R2.funcs 15→17 / R3 0→1，收敛方式是把 `SiteConfig` 拆成独立文件、把策略合并函数拆到 `site_config_policy.go`、把 `EmailCodePolicy` 挪进 `dto.go`、把 `IssueEmailCode` 的冷却与 SMTP 检查抽成子函数。最终全部回到基线。

## 8. 明确不做

- **不做「未验证就不能用」**。没有 `email_verified` 字段，也不打算加：开关是软的、随时可关的，一个硬门禁会让"关掉开关"变成"关不掉"。
- **不做 HTML 邮件**。整套邮件栈都是纯文本（无追踪像素），和 `mail.go` 里其它模板一致。
- **不做登录二次验证（2FA）**。这是另一个量级的东西，当前需求里没有。
- **不做验证码图形化 / 多语言邮件模板**。中文为主、英文副行，够自建站用。
