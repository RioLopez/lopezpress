> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 用户管理（`/admin/users`）

> 状态：Draft（第 19 轮扩展，待实现；覆盖旧版仅启用/禁用）
> 路由：`/admin/users`、`/admin/users/{id}/logins`
> 权限：admin
> 轮次：第 11、12、19 轮
> 最后更新：2026-09-22

## 1. 目标

admin 对用户**全生命周期**管理：列表（状态 / 角色 / 最后登录 IP+时间 / 创建时间）、创建、删除、
重置密码、启用/禁用、查看登录历史。为未来多用户做准备（当前单人站亦可用）。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为 admin，我要查看用户与状态。
  - **AC-1.1**：WHEN 列表渲染 THEN 每行显示 用户名 / 邮箱 / 角色(admin 徽章) / 状态(active/disabled 彩色 chip) / 最后登录 IP / 最后登录时间 / 创建时间
- **US-2**：作为 admin，我要创建用户。
  - **AC-2.1**：WHEN 点「新建用户」THEN 弹窗填 用户名/邮箱/密码/角色 → `POST /api/users` → 成功 toast + 刷新
  - **AC-2.2**：WHEN 重名 / 邮箱占用 THEN 409 + 错误 toast
- **US-3**：作为 admin，我要删除用户。
  - **AC-3.1**：WHEN 点「删除」THEN 确认框（用户名）；`DELETE /api/users/{id}`
  - **AC-3.2**：WHEN 删除自己 THEN 拒绝 400「不能删除自己」
  - **AC-3.3**：WHEN 删除最后一名 admin THEN 拒绝 400「至少保留一名管理员」
  - **AC-3.4**：WHEN 该用户仍有文章 THEN 拒绝 409「请先转移或删除该用户的文章」（单人站默认唯一用户，此条主要面向多用户）
- **US-4**：作为 admin，我要重置密码。
  - **AC-4.1**：WHEN 点「重置密码」THEN 弹窗输入新密码（或生成随机）→ `POST /api/users/{id}/reset-password` → 用户下次用新密码登录
- **US-5**：作为 admin，我要启用/禁用用户（保留旧功能）。
  - **AC-5.1**：WHEN 点「禁用/启用」THEN 确认框 → `PUT /api/users/{id}` 切换 `disabled` → 刷新
- **US-6**：作为 admin，我要查看登录历史。
  - **AC-6.1**：WHEN 点「登录历史」THEN 显示该用户最近 N 次登录（时间 / IP / UA / 成功与否），数据来自 `specs/14-login-log.md`
- **US-7**：作为 admin，我要批量启用/禁用（保留旧功能）。
  - **AC-7.1**：WHEN 勾选多行 THEN 顶部批量条 → `Promise.allSettled` 并发 `PUT /api/users/{id}`

## 3. 前端

- **布局**（ASCII）：

```
┌──────────────────────────────────────────────┐
│ 侧边栏（见 06）                               │
├──────────────────────────────────────────────┤
│ 用户管理  [新建用户]                          │
│ [批量条]（选中>0）                             │
│ 表格：☑|用户|邮箱|角色|状态|最后登录IP|时间|操作│
│   操作：启用/禁用 · 重置密码 · 登录历史 · 删除 │
└──────────────────────────────────────────────┘
```

- **组件树**：`AdminLayout > AdminUsersPage > (admin-batchbar / admin-table / user-create-modal / reset-pwd-modal / login-history-drawer)`
- **关键交互**：
  - `selected: Set<string>`；`allSelected` useMemo
  - 单行启用/禁用走 `setUserDisabled`；批量 `Promise.allSettled`
  - 新建/重置密码走 modal；删除走 confirmDialog
  - 登录历史走右侧抽屉（`GET /api/users/{id}/logins`）
- **路由**：`/admin/users`、`/admin/users/:id/logins`（抽屉或独立页）
- **i18n 键**：`admin.*`（usersTitle / createUser / resetPwd / deleteUser / lastLoginIp / lastLoginAt / loginHistory / ...）
- **共享组件依赖**：`AdminLayout` / `i18n` / `Toast` / `confirmDialog` / `api.*`

## 4. 后端

- **API 端点**：
  - `GET /api/users`（admin）→ `UserRow[] { id, username, email, role, disabled, display_name, last_login_ip, last_login_at, created_at }`
  - `POST /api/users`（admin）→ `{ username, email, password, role }` → 创建；409 重名/邮箱
  - `PUT /api/users/{id}`（admin）→ `{ role?, disabled? }` → 更新
  - `POST /api/users/{id}/reset-password`（admin）→ `{ password }` → 重置；不返回明文
  - `DELETE /api/users/{id}`（admin）→ 删除；400 删自己/最后 admin；409 仍有文章
  - `GET /api/users/{id}/logins?limit=50`（admin）→ `LoginLog[]`（见 spec 14）
- **Service 方法**：`svc.ListUsers` / `svc.CreateUser` / `svc.UpdateUser` / `svc.ResetPassword` / `svc.DeleteUser` / `svc.ListUserLogins`
- **数据模型**：`users` 扩展字段 `display_name` / `bio` / `avatar` / `last_login_ip` / `last_login_at`（AutoMigrate 加列）；`username` 已有 `uniqueIndex`
- **权限检查**：全部 `requireAdmin`；删除/禁用带自保护校验
- **错误处理**：404 不存在；400 自删/最后 admin；409 重名/邮箱/有文章

## 5. 数据流

```
挂载 → listUsers → 渲染表格
新建 → createUser → toast + load()
删除 → confirm → deleteUser → toast + load()
重置密码 → modal → resetPassword → toast
登录历史 → 抽屉 → listUserLogins
```

- **可能失败点**：批量部分失败（某 id 404）→ `Promise.allSettled` 保留 ok/fail 计数
- **回退**：失败 toast + 重新拉取

## 5.5 性能与资源

- **前端包体积**：表格 + 弹窗 + 抽屉，影响 < 4 kB
- **后端查询量级**：用户数（几十级）；批量并发 N 次单写
- **内存 / 磁盘**：无
- **慢点在哪**：用户 > 100 时列表加分页/搜索（候选）
- **是否需重新评估**：用户多时加后端批量端点

## 6. 边界情况

- 无用户：空态「暂无用户」
- 删除唯一用户：拒绝（单人站保护）
- 批量 0 选中：按钮禁用
- 重置密码后旧 token 仍有效直到重新登录（候选：重置即令旧 JWT 失效）

## 7. 已知限制 / 未来方向

- 旧版 09（仅启用/禁用）被本版扩展；保留批量启用/禁用
- RBAC 暂用 `role` 字符串（admin/user），未来可拆 roles/permissions 表（候选）
- 候选：用户搜索 / 分页、重置密码即失效旧会话、邀请码注册

## 8. 相关

- 依赖 spec：`specs/06-admin-layout.md`、`specs/14-login-log.md`、`specs/16-registration.md`
- 相关决策：无
