# 登录 / 注册（`/login` · `/register`）

> 状态：Done（代码已实现，本 spec 首次补写；第 18 轮逐条核验后转 Done）
> 路由：`/login`、`/register`
> 权限：公开（`GuestOnly`，已登录重定向到 `/`）
> 轮次：第 1、6 轮
> 最后更新：2026-09-20

## 1. 目标

访客（或已登出的作者）通过邮箱 + 密码登录获取 JWT，或注册新账户；登录态驱动全站的 `Guarded` 路由。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为作者，我要用邮箱密码登录进入我的站点。
  - **AC-1.1**：WHEN 邮箱 / 密码任一为空 THEN 阻止提交并提示「请填写」
  - **AC-1.2**：WHEN 登录成功 THEN 存 token + 成功 toast「欢迎回来」+ 跳 `/`
  - **AC-1.3**：WHEN 登录失败（密码错 / 用户禁用）THEN 错误 toast（显示后端 `message`）
- **US-2**：作为访客，我要注册新账户。
  - **AC-2.1**：WHEN 用户名 / 邮箱 / 密码任一为空或密码 < 6 位 THEN 阻止提交
  - **AC-2.2**：WHEN 注册成功 THEN 自动登录 + 跳 `/`；重名 / 邮箱已占用 → 错误 toast
- **US-3**：作为用户，我要切换密码可见性。
  - **AC-3.1**：WHEN 点 SHOW/HIDE 按钮 THEN 密码框在 `type=password` 与 `text` 间切换

## 3. 前端

- **布局**（ASCII）：

```
┌───────────────────────────┐
│ 顶栏（Layout，登录态空）   │
│  auth-card（居中卡片）     │
│   h1 标题 + 副标题          │
│   字段（用户名/邮箱/密码）  │
│   [登录 / 注册] 全宽按钮    │
│   底部链接：注册 ↔ 登录     │
└───────────────────────────┘
```

- **组件树**：`Layout > (LoginPage | RegisterPage) > auth-card > field`
- **关键交互**：
  - 提交前置校验（非空 / 密码长度）→ `busy` 锁按钮
  - 密码 `showPwd` 切换 `type` + SHOW/HIDE 文案（第 6 轮密码可见性）
  - 底部链接 `e.preventDefault()` + `navigate('/register' | '/login')`（15 轮已修原生跳转）
  - 成功后 `useAuth().login/register` 内部存 token，再 `navigate('/')`
- **路由**：`/login`、`/register`，均 `GuestOnly` 包裹（已登录 → 跳 `/`）
- **i18n 键**：`auth.*`
- **共享组件依赖**：`Layout` / `auth`（`useAuth`）/ `i18n` / `Toast` / `router`

## 4. 后端

- **API 端点**：
  - `POST /api/auth/login` → `{ token, user }`；401 凭证错 / 用户禁用
  - `POST /api/auth/register` → `{ token, user }`；409 重名 / 邮箱占用
- **Service 方法**：`svc.Login` / `svc.Register`（签发 JWT，`JWT_SECRET` + `JWT_EXPIRY_HOURS`）
- **注册即登录**（第 18 轮定型）：`svc.Register` 直接返回 `*AuthResult`，前端免去「注册完再调 login」的第二次往返；
  此前由 `auth.tsx` 的 `doRegister` 内部补调一次 `doLogin(username, password)` 实现（后端登录支持用户名匹配）
- **数据模型**：`users`（`username`、`email`、`password`（bcrypt hash）、`role`、`disabled`、`created_at`）
- **权限检查**：公开；登录时校验 `disabled` 用户拒绝
- **错误处理**：401 / 409 带 `message`

## 5. 数据流

```
输入 → 前端校验 → login/register → POST /api/auth/* → svc → SQLite
  → 成功：存 token（localStorage['echo.token']）→ navigate('/')
  → 失败：ApiError.message → toast
```

- **可能失败点**：网络失败 / 凭证错 / 用户禁用 / 重名
- **回退**：失败保留已输入，不跳转

## 5.5 性能与资源

- **前端包体积**：两个卡片复用同构逻辑，影响 < 2 kB
- **后端查询量级**：登录 O(1) 按邮箱 / 用户名；注册 O(1) 查重
- **内存 / 磁盘**：`users` 表行数级；bcrypt 哈希计算（每次登录 ~100ms，可接受）
- **慢点在哪**：bcrypt cost 默认 10，高并发登录时可升（本项目单人，无影响）
- **是否需重新评估**：多用户场景下 bcrypt 参数

## 6. 边界情况

- 空字段 / 短密码：阻止提交
- 已登录访问 /login：`GuestOnly` 自动跳 `/`
- 用户被禁用：401 + 提示
- 网络失败：保留输入 + toast

## 7. 已知限制 / 未来方向

- 无找回密码（需 SMTP，见 13 轮决策备忘，不做）
- 无登录态刷新（JWT 过期只能重登，`JWT_EXPIRY_HOURS` 控制）
- 候选：注册时邮箱格式校验已做（`type=email`）；可扩展邀请码 / 注册开关
- 候选：登录"记住我"（延长 token 有效期）

## 8. 相关

- 依赖 spec：无（这是入口）
- 相关决策：无
