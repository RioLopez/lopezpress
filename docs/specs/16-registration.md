> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 注册开关（`allow_register`）

> 状态：Draft（第 19 轮新增）
> 路由：`/register`（公开，受开关控制）
> 权限：公开（受 `site_config.allow_register` 控制）
> 轮次：第 19 轮
> 最后更新：2026-09-22

## 1. 目标

单人站默认**关闭公开注册**，仅管理员在后台创建用户（见 spec 09）；未来多用户可开启。
通过 `site_config.allow_register` 控制 `POST /api/auth/register` 是否可用，并提供公开探测接口供前端显隐注册入口。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为系统，我要默认关闭公开注册。
  - **AC-1.1**：WHEN 首次启动（`site_config` 种子）THEN `allow_register = false`
- **US-2**：作为访客，关闭时我看不到可用注册。
  - **AC-2.1**：WHEN 打开 `/register` THEN 前端先 `GET /api/auth/register/open`；`open=false` 时显示「注册已关闭，请联系管理员」并仅提供登录入口
  - **AC-2.2**：WHEN 直接 `POST /api/auth/register` 且开关关闭 THEN 返回 403 `{message:"注册已关闭"}`
- **US-3**：作为 admin，我要开启公开注册。
  - **AC-3.1**：WHEN 在站点设置（spec 05）切换「允许公开注册」并保存 THEN `allow_register` 生效；`/register` 可用

## 3. 前端

- **布局**（ASCII）：

```
/register
┌──────────────────────────────┐
│ 注册（或：注册已关闭）        │
│  open=false → 提示 + [去登录] │
│  open=true  → 用户名/邮箱/密码│
└──────────────────────────────┘
```

- **组件树**：`Layout > RegisterPage`（读 `GET /api/auth/register/open` 决定显隐）
- **关键交互**：
  - 挂载探测 `open`；关闭时禁用表单 + 提示
  - 提交时后端 403 仍兜底提示（防绕过）
- **路由**：`/register`（GuestOnly）
- **i18n 键**：`auth.*`、`register.*`（closed / contactAdmin）
- **共享组件依赖**：`Layout` / `i18n` / `auth` / `Toast`

## 4. 后端

- **API 端点**：
  - `GET /api/auth/register/open`（**公开**）→ `{ open: bool }`（仅布尔，不下发明文配置）
  - `POST /api/auth/register`（公开）→ `svc.Register` 前置校验 `allow_register`；关闭则 403
- **Service 方法**：`svc.Register` 增加开关校验（读取 `site_config.allow_register`）
- **数据模型**：`site_config.allow_register` bool（见 spec 05）
- **权限检查**：开关关闭时拒绝公开注册；管理员 `POST /api/users` 创建用户**不受开关影响**
- **错误处理**：关闭时 403 + `message`

## 5. 数据流

```
启动 → config.go 种子 allow_register=false
/register 打开 → GET /api/auth/register/open → 显隐
提交 → POST /api/auth/register → svc.Register 查 allow_register → 关闭 403
admin 创建 → POST /api/users（绕过开关）
```

- **可能失败点**：前端探测被绕过（直接 POST）→ 后端 403 兜底
- **回退**：开关误关 → 管理员仍可在后台建用户

## 5.5 性能与资源

- **前端包体积**：探测 1 个布尔，影响 < 0.5 kB
- **后端查询量级**：探测 1 行读（可缓存）
- **内存 / 磁盘**：无
- **慢点在哪**：无
- **是否需重新评估**：无

## 6. 边界情况

- 管理员创建用户不受开关影响（正确行为）
- 探测接口仅返回布尔，不泄露内部开关语义之外信息
- 开关切换后立即生效（配置读实时，或变更失效缓存）

## 7. 已知限制 / 未来方向

- 候选：邀请码注册、注册需审核、邮箱验证（需 SMTP，本期不做）
- 与 spec 04-auth 注册流程关联：旧「注册即登录」行为在开启时保留

## 8. 相关

- 依赖 spec：`specs/05-settings.md`（allow_register 字段 + 后台开关）、`specs/04-auth.md`（注册流程）、`specs/09-admin-users.md`（管理员创建用户）
- 相关决策：无
