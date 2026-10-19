# lopezpress 功能与逻辑审计（2026-10-14）

> ⚠️ **已归档（2026-10-15）**：本文是 2026-10-14 首轮审计的**历史快照**，结论已被
> `docs/TODO.md` 逐项落地并勾选。保留仅为追溯"当时发现了什么"，
> **不要再把本文当作待办清单**——唯一权威待办是 `docs/TODO.md`。
> 归档原因：与 TODO.md 内容重复度高，放在 docs/ 根目录易误导接手者。

> 基于通读全栈源码 + 实地查证（非凭记忆）。按 优先级 分级。每条注明「实测确认」或「威胁模型依赖」。

## 审计方法
逐层读过 `cmd/server`、`internal/*`、`web/src/*`；重点实地查证了：路由鉴权矩阵（router.go + handlers 角色校验）、`renderMD` 的 XSS 清洗、`Post` 软删、注册/改密/重置的密码与 `must_change_password` 处理、备份导出内容。

## 结论摘要
- **P0（必须修）**：无。鉴权、软删两大高危项实测均正确。
- **P1（建议尽快修）**：2 项——`renderMD` XSS 清洗不完整；`must_change_password` 后端不强制。
- **P2（可改进）**：注册无密码强度、备份文件含密码哈希、i18n 死键、后台表格未统一横滚、GORM 日志刷屏等。
- **设计取舍（非缺陷）**：限流内存态、JWT 无吊销、LIKE 搜索、XFF 可伪造（经 Nginx 可缓解）。
- **功能缺口（增强）**：2FA/安全中心、评论、RSS/站点地图、邮件通知、图片管理、操作审计日志。

---

## P1 — 建议尽快修

### P1-1 `renderMD` 存储型 XSS 清洗不完整【实测确认】
- 现状（`web/src/core/md.ts` `renderMD`）：`marked` 解析 → 只 `remove <script>` → 过滤 `img[src]` 的 `data:`/`javascript:` → `a[href]` 白名单（https?/mailto//#./）。
- 缺口：**没有通用 HTML 清洗**。`marked` 默认放行原始 HTML，因此作者文章里的这些都会被执行：
  - `<img src=x onerror=alert(document.cookie)>`（`on*` 事件属性未剥离）
  - `<iframe src=...>` / `<object>` / `<embed>` / `<svg onload=...>` / `<link>` / `<style>`（危险标签未移除）
- 威胁模型：单作者自绑 = 自己 X 自己（低危）；**一旦开放注册/多作者 = 高危**（恶意作者可偷读者/管理员的 JWT、以管理员身份操作）。
- 修复（契合项目零依赖哲学，二选一）：
  - A（推荐，轻量、零新依赖）：`renderMD` 里对解析结果做**标签白名单 + 剥 on\* 属性**——遍历所有元素，移除 `on*` 属性、移除非白签标签（iframe/object/embed/svg/link/style/form/input…），只保留排版相关标签与 `href/src/alt/title/class`。
  - B（更稳）：引入 `DOMPurify`（`npm i dompurify`）在 `renderMD` 输出前 sanitize。代价：多一个依赖。

### P1-2 `must_change_password` 后端不强制【实测确认】
- 现状：`MustChangePassword` 只是登录返回给前端的标志，前端据此跳 `/change-password`；**后端其他接口不拒绝**。用户可绕过 UI 直接调 API，带着首启临时密码正常用。
- 修复：在鉴权链（`handlers.WithAuth` 之后，或加一个中间件）判断 `user.MustChangePassword` 为真时，**除 `/api/auth/change-password`、`/api/auth/logout`、`GET /api/auth/me` 外一律 403（提示需先改密）**。

---

## P2 — 可改进

| # | 问题 | 说明 / 建议 |
|---|---|---|
| P2-1 | 注册无密码强度校验 | `services.Register` 只判非空。开放注册时可设 1 位弱密码。建议最小长度 ≥8（与改密一致）。 |
| P2-2 | 备份文件含 `password_hashes` | 导出/下载的 JSON 含所有用户 bcrypt 哈希（恢复需要，但文件=可离线爆破的敏感资产）。建议：下载/导出处强提示妥善保管，长期可考虑加密备份。 |
| P2-3 | i18n 死键 | 第 22 轮移除 draft/scheduled，但 `i18n.tsx` 仍留 `list.draft`、`list.scheduled`、`editor.schedule*` 等无用文案。清理。 |
| P2-4 | 后台表格未统一横滚 | 仅 `AdminUsersPage` 包了 `.table-scroll`；分类/标签/回收站/仪表盘未包（目前靠 `.admin-main{overflow-x:auto}` 兜底）。建议统一包一层。 |
| P2-5 | GORM `record not found` 刷屏 | 首启播种/查询未命中时以 warn 级打一片红字（用户见过）。建议 Logger 忽略 `gorm.ErrRecordNotFound` 或降 debug。 |
| P2-6 | `X-Forwarded-For` 可伪造 | 限流器取 XFF 第一跳；直连暴露 :8080 时可伪造绕过 IP 桶。生产经 Nginx 会覆盖 XFF，风险低；可在 Nginx 层 `$remote_addr` 强制覆盖。 |

---

## 设计取舍（非缺陷，列出备查）
- 限流为内存态：单实例有效；多实例部署需外接存储（当前单机不触发）。
- JWT 无吊销：logout 仅客户端信号；改密后旧 token 仍有效至过期（1h）。要"改密即全端下线"需 token 版本号。
- 搜索 `LIKE '%...%'`：纯 Go 驱动无 FTS5，千级内可接受（decision 0001）。
- 草稿仅 localStorage 30 天，不跨设备（decision 0002）。

---

## 功能缺口（增强方向，按价值排序）
1. **RSS / 站点地图**：博客刚需，利于订阅与 SEO，实现轻量。
2. **安全中心落地**：个人中心“安全中心”目前是占位（i18n 明说“后续开放”）；至少补“修改密码/查看登录历史/登出所有设备”。
3. **图片管理**：目前只能上传、不能浏览/删除已上传图片。
4. **评论**：可外接（Giscus/Disqus）或自建；个人博客常见诉求。
5. **操作审计日志**：目前只有登录历史；谁改了文章/配置无记录。
6. **邮件通知**：改密/异地登录提醒（需 SMTP，低配机可选）。
7. **文章内容多语言**：UI 已 i18n，文章内容本身无多语言。

---

## 建议修复顺序
1. **P1-1 XSS**（若未来可能开放注册/多作者，越早修越好；单作者也可顺手加固）。
2. **P1-2 must_change_password 后端强制**（让首启临时密码机制真正闭环）。
3. P2 里挑高性价比的：P2-1 密码强度、P2-3 i18n 清理、P2-4 后台表格横滚、P2-5 日志刷屏。
4. 功能缺口按需：RSS/Sitemap → 安全中心 → 图片管理。

> 结论：项目逻辑总体健康、分层清晰、鉴权与数据完整性这两大高危项实测正确；真正建议优先处理的是 **P1-1（XSS）与 P1-2（改密强制）** 两个安全闭环。
