> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 登录日志（`login_log`）

> 状态：Draft（第 19 轮新增）
> 路由：`/admin/users/{id}/logins`（查看）
> 权限：admin
> 轮次：第 19 轮
> 最后更新：2026-09-22

## 1. 目标

记录**每次登录尝试**（成功 / 失败）的用户、IP、User-Agent、时间，支持安全审计与管理员查看。
为异常登录排查、账号安全提供数据（行业通用做法：DuxCMS 在 user 表存 last_login，完整审计用独立日志表）。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为系统，我要每次登录都落库。
  - **AC-1.1**：WHEN 任何 `POST /api/auth/login` 请求（成功或失败）THEN 写一行 `login_log`（成功含 `user_id`；失败含尝试的 `identifier`，`user_id` 可为空）
  - **AC-1.2**：WHEN 登录成功 THEN 同时更新 `users.last_login_ip` / `users.last_login_at`
- **US-2**：作为 admin，我要查看某用户登录历史。
  - **AC-2.1**：WHEN 打开 `/admin/users/{id}/logins` THEN 列表展示最近 N 条（时间倒序）：时间 / IP / UA / 成功与否（彩色 chip）
- **US-3**：作为 admin，我要在仪表盘看最近登录。
  - **AC-3.1**：WHEN 打开 `/admin` THEN 仪表盘含 `recent_logins`（最近 5~10 条，跨用户）

## 3. 前端

- **布局**（ASCII）：

```
┌──────────────────────────────────────┐
│ 登录历史（抽屉/页）                    │
│ 时间 | IP | UA | 结果                  │
│ 2026-09-22 10:01 | 1.2.3.4 | curl.. | 成功│
│ 2026-09-22 09:58 | 5.6.7.8 | Moz..  | 失败│
└──────────────────────────────────────┘
```

- **组件树**：`AdminLayout > AdminUserLoginsPage`（或 `LoginHistoryDrawer` 嵌入用户管理）
- **关键交互**：列表时间倒序；失败行红色 chip；空态「暂无记录」
- **路由**：`/admin/users/:id/logins`（admin）
- **i18n 键**：`admin.*`（loginTime / loginIp / loginUa / loginOk / loginFail / noLogs）
- **共享组件依赖**：`AdminLayout` / `i18n` / `Toast`

## 4. 后端

- **API 端点**：
  - `POST /api/auth/login` 内部调用 `svc.RecordLogin`（成功/失败均记录）
  - `GET /api/users/{id}/logins?limit=50`（admin）→ `LoginLog[]`
  - `GET /api/admin/dashboard` 含 `recent_logins`（admin，见 spec 06）
- **Service 方法**：`svc.RecordLogin(ctx, userID *string, identifier, ip, ua string, success bool)` / `svc.ListUserLogins(ctx, id, limit)`
- **数据模型**：`login_log`
  - `id` integer PK
  - `user_id` text（nullable, index）—— 成功时填，失败时空
  - `identifier` text —— 尝试登录的用户名/邮箱（失败审计用）
  - `ip` text、`ua` text、`success` bool、`created_at` datetime（index `user_id,created_at`）
- **权限检查**：写由登录流程内部触发（无需鉴权）；读需 `requireAdmin`
- **错误处理**：读不存在用户 404

## 5. 数据流

```
登录请求 → svc.Login 校验 → RecordLogin(成功/失败, ip, ua) → login_log 落库
  → 成功时 UpdateUser(last_login_ip, last_login_at)
admin → listUserLogins → 渲染
```

- **可能失败点**：写日志失败不应阻断登录（日志写入用独立错误吞掉，仅记 server log）
- **回退**：日志写失败不影响主流程

## 5.5 性能与资源

- **前端包体积**：列表/抽屉，影响 < 1 kB
- **后端查询量级**：每次登录 1 行 INSERT；历史查询 LIMIT + 索引
- **内存 / 磁盘**：日志行累积；单人站量极小；候选保留策略（90 天或上限 N 行）
- **慢点在哪**：无（索引覆盖）
- **是否需重新评估**：日志膨胀时加清理任务（低优先级）

## 6. 边界情况

- 失败登录也记录（防爆破审计，不泄露密码）
- IP 取自 `X-Forwarded-For`（取首个）|| `RemoteAddr`
- 用户被删：历史 `user_id` 保留（外键不强制），仅展示 identifier

## 7. 已知限制 / 未来方向

- 单人站日志量极小；多用户时可加「异常 IP 提醒」「登录地理分布」
- 候选：日志保留期 / 上限清理、用户自查自己登录历史（`GET /api/me/logins`）

## 8. 相关

- 依赖 spec：`specs/04-auth.md`（登录流程）、`specs/09-admin-users.md`（查看入口）、`specs/06-admin-layout.md`（仪表盘）
- 相关决策：无
