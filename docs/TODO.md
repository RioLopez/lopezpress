# lopezpress 待办清单（2026-10-15 三次更新）

> 来源：功能逻辑审计 + 二轮严格自查 + **2026-10-15 全栈深度审计**（均基于读实现，非猜测）。
> 完成后把 `[ ]` 改 `[x]`。
> **本文件是唯一权威待办清单**。首轮审计快照已归档至 `docs/archive/2026-10-14-FUNCTIONAL_AUDIT.md`（勿再当作待办）。
>
> **当前状态（2026-10-19 三次修复轮 + 第 27 轮后）**：
> - 早期 P1/P2/P3 与功能缺口全部完成（见下方各节勾选）；
> - **P0 级 6 项已全部修复**（修复轮 1）；
> - P1 级已修 **P1-3 / P1-4 / P1-5 / P1-6 / P1-7 / P1-8 / P1-9 / P1-10 / P1-11 / P1-12 / P1-13 / P1-14 / P1-2**（修复轮 1–3）；仅剩 P1-1 余部（roles.go helper 全量化，架构美化非缺陷，暂缓）；
> - P2 级已修 **P2-1（死代码 9 处）/ P2-4 / P2-5 / P2-7 / P2-8 / P2-9 / P2-10 / P2-11 / P2-12（Q1 审核状态前置）/ P2-13 / P2-14 / P2-15（死导出删除）/ P2-2（重复代码抽取：fmtDate/Modal/writeInvalidBody/TableState 全收敛）/ P2-3（limit 解析已统一 + 其余 7 类核查为纯样式）**；仅剩 **P2-6（可信代理闸门，已降级评估）**；
> - 「评论」仍为唯一未做的增强方向。

## 🔴 P0 — 必须修（2026-10-15 深度审计新增）

> 证据链见 `docs/AUDIT_2026-10-15.md` §1。

- [x] **P0-1 `ResetPassword` 越权 → admin 可重置 root 密码接管站点**
  `services/accounts.go:385` `ResetPassword(ctx, id, password)` **无 actor 参数**；`handlers/accounts.go:163` 只查 `requireAdmin`（admin-or-root）。
  后果：任何 admin 可设 root 新密码并登录接管。修法：签名加 `actor`，入口 `canManageUser(actor, target)` 校验；补「admin 重置 root → 403」测试。
  ✅ **已修（2026-10-15 修复轮）**：`ResetPassword(ctx, actor, id, password)` + `canManageUser` 守卫；handler 透传 `userFromContext(r)`。回归测试 `TestResetPasswordForbidsAdminResettingRoot`（handler）、`TestResetPasswordHierarchy`（service）全绿。
- [x] **P0-2 `DeleteUpload` 越权 → 任意登录用户可删他人上传的图片**
  `handlers/upload.go:54-61` 无属主、无 `requireAdmin`；`services.go:2251` 直接 `os.Remove`。
  同类管理端点（categories/users/trash）全有 `requireAdmin`，唯独此处漏了。修法：加 `requireAdmin`。
  ✅ **已修（2026-10-15）**：`DeleteUpload` 加 `requireAdmin`（图片库为管理功能，非用户功能）。`go build/vet/test` ✅。
- [x] **P0-3 i18n key 拼错 → 用户看到原始键名 "statusLabel.pending"**
  代码用顶层 `t('statusLabel.*')`（`ListPage.tsx:213`、`ReadPage.tsx:224`、`PersonalCenterPage.tsx:282-284`），定义却在 `admin.statusLabel.*`（`i18n.tsx:754/1236`）；`t()` 未命中原样返回 key。第 25 轮引入。修法：三处改 `admin.statusLabel.`；顺手修 `i18n.tsx:265` 错注释。
  ✅ **已修（2026-10-15）**：三处调用点改 `admin.statusLabel.*`，并修正 `i18n.tsx:265` 注释。`tsc --noEmit` + `vite build` ✅。
- [x] **P0-4 审核流闭环断裂 → "修改后重新提交"不可达（核心卖点不可用）**
  四层全断：① `review_note` 前端零展示（i18n 却承诺"作者可见"）；② `services.go:876-883` `UpdatePost` 只接受 private/published，rejected 保存后仍 rejected；③ 无 rejected→pending 接口；④ 前端无重新提交入口。
  修法：后端允许作者 rejected→pending；前端 ReadPage/PersonalCenter/EditPage 展示意见 + PostEditor 加"重新提交"。
  ✅ **已修（2026-10-15）**：后端 `UpdatePost` 允许作者把 `rejected` 改回 `pending`（清 `review_note`）；前端 `PersonalCenterPage` 展示审核意见 + "重新提交审核"按钮、`PostEditor` 对 rejected 文章显示意见横幅且主按钮变"重新提交审核"（提交时 coerce 为 pending）、`EditPage` 透传 `review_note`；补 `editor.resubmit` / `center.resubmitted` / `center.resubmitFailed` i18n + 样式。回归 `TestAuthorResubmitsRejectedPost` 全绿。`tsc`+`vite build` ✅。
- [x] **P0-5 分类/标签计数虚高（软删 + banned + pending 全算进去）**
  `services.go:1403-1441` 用 `Table("categories")` 裸查询绕过 GORM 软删 scope，也无 status 过滤。修法：手写 `deleted_at IS NULL` + 视产品排除 banned/pending。
  ✅ **已修（2026-10-15）**：`ListCategoriesWithCount` / `ListTagsWithCount` 的 JOIN 条件加 `deleted_at IS NULL AND status <> 'banned'`（过滤写在 ON 子句，零计数分类仍返 0；tags 改用 `COUNT(posts.id)` 排除 banned 文章的孤儿 `post_tags` 行）。
- [x] **P0-6 `writeXML` 错误路径返回 200**
  `handlers/feed.go:189-192` marshal 失败时写错误体但**不设状态码**，且错误体不符合全项目 JSON 契约。修法：`WriteHeader(500)` + JSON 错误体。
  ✅ **已修（2026-10-15）**：`writeXML` marshal 失败改 `WriteHeader(500)` + `{"error":...}` JSON 体（与全站契约一致），且把 marshal 前移到写头之前避免已写 200。
- [x] **P0-7 备份导入/恢复是完整 root 接管路径（代码反推审计 Q7，第 30 轮）**：`ImportBackup`/`RestoreSnapshot` handler 仅 `requireAdmin`，而 service 的 `tx.Create(&u)` 不校验角色——admin 导入一份把自建用户设为 root（或替换 root 的 password_hash）的备份 JSON 即可接管站点，直接绕过"root 不可经 API 创建"红线（R2）；恢复快照同理替换全库。
  ✅ **已修（2026-10-19 第 30 轮）**：service 层 `ImportBackup`/`RestoreSnapshot` 加 `actor` 参数 + `IsRoot(actor)` 守卫（与 P0-1/P1-2 同构的纵深防御）；handler 改 `requireRoot` 并透传；前端导入区对非 root 禁用并说明（新 i18n 键 `admin.importRootOnly` 中英）。回归 `backup_root_gate_test.go` `TestImportAndRestoreAreRootOnly`（admin 403 / root 放行 / 恶意 root 行未落库）。`go build/vet/test` + `tsc` + `vite build` 全绿。

## 🟠 P1 — 架构收敛 / 数据一致性 / 明显缺陷（2026-10-15 新增）

- [ ] **P1-1 `roles.go` 集中权限层是死代码（架构失信头号案例）**
  `roles.go:35/:61` `canManageUser`/`canReviewPost` **零调用**，文件头却宣称"所有权限判断都走这里"。实际全靠手写 if-else，漏一个就是 P0-1。
  修法：用户管理 3 方法 + 审核 4 方法全部改调 helper，删手写链。
  🔶 **部分落地（2026-10-15）**：`ResetPassword` 已改调 `canManageUser`（权限层不再是全死代码，P0-1 根因消除）。`UpdateUser`/`DeleteUser`/`CreateUser`/`ReviewPost`/`BanPost` 仍用手写链——但审计 §6 已确认这些手写链**层级校验完整、行为正确**，整体重写为 helper 风险大于收益，暂缓（属架构美化，非缺陷）。
- [x] **P1-2 `CreateUser` service 层不校验 actor**（`accounts.go:264`）：普通 user 亦可建号，与"service 是权威"的承诺相反。
  ✅ **已修（2026-10-15 修复轮 3）**：`CreateUser` 入口加纵深防御 `if !IsAdminOrRoot(actor) { return 403 }`——handler 已有 `requireAdmin`，service 层不再信任 handler。
- [x] **P1-15 banned 文章未冻结（代码反推审计 Q2，第 28 轮）**：作者可 `UpdatePost`/`DeletePost` 自己已 banned 的文章（违背"banned 仅 root 管"）；作者软删后 root 的 `DeleteBannedPermanently` 因软删 scope 404，封禁数据永久残留。
  ✅ **已修（2026-10-19 第 28 轮）**：`UpdatePost` 对 banned 非 root 403；`DeletePost` 对 banned 全角色 403（出口收敛到 vault 的 send-back/永久删）；回归 `TestBannedPostFrozenForAuthor`（6 断言）。`go build/vet/test` + `tsc` + `vite build` 全绿。
- [x] **P1-16 disabled 账号无 Authenticate 门（代码反推审计 Q6，第 29 轮）**：`Login` 拦禁用账号，但已持有 JWT（最长 24h）在 `Authenticate` 无 disabled 检查 → 被禁用后所有会话（含 WithAuth 写操作）当天仍可用；`must_change_password` 有门而 disabled 没有，同类状态只做一半。
  ✅ **已修（2026-10-19 第 29 轮）**：`Authenticate` token_version 校验后加 `disabled → 403`，一处修覆盖 WithAuth/WithOptionalAuth；回归 `TestAuthenticateRejectsDisabledUser`（禁用 403 / 重新启用同 token 恢复）。`go build/vet/test` 全绿（本轮未触及前端）。
- [x] **P1-3 `DeleteUser` 与回收站文章数据不一致**（`accounts.go:485`）：Count 带软删 scope → 用户文章全在回收站时可删用户，留下孤儿 author_id。
  ✅ **已修（2026-10-15）**：Count 改 `Unscoped()`，回收站文章也计入，避免删用户后留下孤儿 `author_id`（`ListTrash.Preload("Author")` 不再 nil 回退）。
- [x] **P1-4 `DeleteBannedPermanently` 不清理 post_tags**（`services.go:1229`）：硬删 post 但关联行成孤儿，叠加 P0-5。
  ✅ **已修（2026-10-15）**：硬删前先 `WHERE post_id = ?` 删 `post_tags` 关联行（与 `DeleteTag` 对齐），避免标签计数继续虚高。
- [x] **P1-5 `WithOptionalAuth` 无 must_change_password 门**（`auth.go:293-312`）：被锁用户仍可浏览全部读接口。
  ✅ **已修（2026-10-15 修复轮 2）**：`WithOptionalAuth` 命中 `MustChangePassword` 时降级为零值 user（仅给公开读 scope，不给个人数据），与 `WithAuth` 首启锁定对齐；补 `TestOptionalAuthDowngradesMustChangePassword`（handler）回归。
- [x] **P1-6 `ImportBackup` 无 MaxBytesReader**（`admin.go:152`）：超大 body 可撑爆内存（upload 有 2MB 封顶，此处没有）。
  ✅ **已修（2026-10-15 修复轮 2）**：套 `http.MaxBytesReader`（20 MB 上限），超限经 `errors.As(*http.MaxBytesError)` 返回 413。
- [x] **P1-7 `RestoreSnapshot` 零审计**（`admin.go:171-188`）：破坏性数据库恢复无记录。
  ✅ **已修（2026-10-15 修复轮 2）**：恢复成功后 `recordAudit(r, "restore_snapshot", "snapshot:"+in.File, "ok")`。
- [x] **P1-8 双重审计**：moderation 4 个操作 handler + service 各记一条 action_log。
  ✅ **已修（2026-10-15 修复轮 2）**：删 handler 侧 4 条 `recordAudit`（`review/ban/send_back/delete_banned`），service 层 `RecordAction` 为唯一审计点。
- [x] **P1-9 前端 5 处"失败时弹成功文案"**：`PersonalCenterPage:230/:271`、`SettingsPage:120/:45`、`AdminSitePage:74`。
  ✅ **已修（2026-10-15 修复轮 2）**：5 处 catch 改独立失败 key：`list.visibilityFailed` / `profile.saveFailed` / `imageLib.deleteFailed` / `site.saveFailed`（新增 i18n 键 zh+en）。
- [x] **P1-10 前端"重试"死按钮**：`PersonalCenterPage:139/:299` `setPage((p) => p)` 不触发 effect。
  ✅ **已修（2026-10-15）**：两个组件各加 `reloadKey` state 并纳入加载 effect 依赖，重试按钮 `setReloadKey((k) => k + 1)` 真正重拉。
- [x] **P1-11 `ListPage` 计数无意义**：`:198` from/to 同值，渲染 "20–20 / 37"。
  ✅ **已修（2026-10-15 修复轮 2）**：`from = (page-1)*size+1`、`to = (page-1)*size+len(items)`，分页计数有意义。
- [x] **P1-12 前端 `canManage` 未排除自己**（`AdminUsersPage:85-88`）：对自己行禁用/删除可点但必被 403。
  ✅ **已修（2026-10-15 修复轮 2）**：`canManage` 加 `u.id !== me.id`，对本人行恒 false（与后端 self-protection 对齐）。
- [x] **P1-13 密码策略三套标准**：前端 `<6`（Register/AdminUsers）vs `<8`（PersonalCenter/ChangePassword）vs 后端统一 8。
  ✅ **已修（2026-10-15 修复轮 2）**：Register / CreateUser / ResetPasswordModal 统一为 `< 8` + 新 key `auth.passwordTooShort`（zh+en），与后端及 PersonalCenter/ChangePassword 一致。
- [x] **P1-14 `UpdatePost` private 分支不限角色**（`services.go:877`）：可把 pending/rejected 改成 private 绕过审核队列。
  ✅ **已修（2026-10-15 修复轮 2）**：private 分支分级——作者仅允许 `published → private`（正常取消公开）；pending/rejected/banned 改 private 被忽略（留在审核通道，无法绕过队列）；root/admin 编辑他人文章仍可自由设 private。补 `TestAuthorCannotEscapeQueueViaPrivate`（service）回归。

## 🔴 P1 — 安全 / 越权（2026-10-14 首批，已全部完成）

- [x] **P1-0 私密文章越权（二轮新发现，最高优先）**
  `services.GetPost(ctx, id)`（`GET /api/posts/{id}`，阅读页在用）只按 id 查、**不校验 private 归属** → 非作者/匿名凭文章 UUID 即可读他人私密文章全文。`ListPostsPage` 已正确排除他人 private，但 `GetPost` 这条漏了。
  修：`GetPost` 传入 `user`，若 `status=="private"` 且非作者且非 admin → 返回 404（与不存在一致，不泄露存在性）。
  ✅ 已修（2026-10-14）：`GetPost(ctx, user, id)` + 可见性守卫；补 `TestGetPostPrivateVisibility` 回归测试（作者/admin 可读、他人/匿名 404）。

- [x] **P1-1 `renderMD` 存储型 XSS 清洗不完整**
  只删 `<script>`，未剥 `on*` 事件属性、未拦 `iframe/object/embed/svg` 等。多作者高危、单作者低危。
  修：`renderMD` 输出前做“标签白名单 + 剥 on* 属性”（零新依赖）或引入 DOMPurify。
  ✅ 已修（2026-10-14）：`sanitizeHTML` 白名单过滤（保留常用排版标签，剥 `on*` 属性、禁 `javascript:`/`data:` 协议、移除危险标签）。

- [x] **P1-2 `must_change_password` 后端不强制**
  仅前端跳转 `/change-password`，后端不拦，可绕过 UI 直接调 API。首启临时密码机制没闭环。
  修：鉴权链判断该标志，除 `change-password`/`logout`/`GET me` 外一律 403。
  ✅ 已修（2026-10-14）：`WithAuth` 校验 `MustChangePassword`，白名单放行 `/api/auth/me`、`/api/auth/change-password`、`/api/auth/logout`，其余 403。

## 🟠 P2 — 应改进

- [x] **P2-1 CORS 白名单被 echo-back 绕过**（`middleware.go`）：受限模式下“未知 origin”分支仍 `ACAO: <origin>` 回显，白名单形同虚设（本项目用 Bearer 不用 cookie，实际危害低，但逻辑错）。修：未知 origin 在受限模式不设 ACAO / 直接拒绝。
  ✅ 已修（2026-10-14）：受限模式下未知 origin 不再回显。
- [x] **P2-2 `limiter.go` `clientIP` 未取 XFF 第一跳**：返回整串 `"c, p1, p2"`，与 `login_limiter`/`handlers` 的取法不一致。修：统一取第一跳。
  ✅ 已修（2026-10-14）：统一取 XFF 第一跳。
- [x] **P2-3 前端 401 处理不统一**：`api.ts` 无全局 401 拦截，过期 token 时部分页只弹 toast 不清 token/不跳登录。修：`request` 里 401 统一清 token + 跳 `/login`。
  ✅ 已修（2026-10-14）：`api.ts` 全局 401 拦截 → 清 token + 通知 auth 上下文跳登录。
- [x] **P2-4 注册无密码强度**：`services.Register` 只判非空，可设 1 位弱密码。修：最小长度 ≥8（与改密一致）。
  ✅ 已修（2026-10-14）：Register/CreateUser/ResetPassword 密码统一 ≥8；前端重置密码提示同步。
- [x] **P2-5 备份文件含 `password_hashes`**：导出/下载的 JSON 含全部 bcrypt 哈希（恢复需要，但文件=可离线爆破的敏感资产）。修：下载/导出处强提示妥善保管；长期可加密。
  ✅ 已修（2026-10-14）：导出 JSON 写入 `security_note` 字段（随文件走，泄露也能警示）；`BackupResult` 增加 `warning` 提醒后台；`AdminBackupPage` 顶部新增安全警告框（中英双语）。长期加密另议。
- [x] **P2-6 i18n 死键**：第 22 轮移除 draft/scheduled，`i18n.tsx` 仍留 `list.draft`/`list.scheduled`/`editor.schedule*` 等无用文案。清理。
  ✅ 已修（2026-10-14）：删 7 个真死键（接口声明 + zh/en 值块）：`list.draft`/`list.scheduled`、`editor.scheduleLabel`/`scheduleHint`/`scheduleRequired`/`saveDraftBtn`/`draftSaved`。**保留活键** `editor.restoreDraft`/`restoreDraftConfirm`/`restoreDraftDiscard`（PostEditor 浏览器 localStorage 草稿恢复仍在用）与 `list.published`/`allStatus`。前端 `tsc`+`vite build` ✅。
- [x] **P2-7 后台表格未统一横滚**：仅 `AdminUsersPage` 包了 `.table-scroll`，分类/标签/回收站/仪表盘未包（靠 `.admin-main{overflow-x:auto}` 兜底）。统一包一层。
  ✅ 已修（2026-10-14）：分类/标签/回收站三页已包 `.table-scroll`（用户管理页此前已包；仪表盘无表格）。

## 🟡 P2 — 代码债（2026-10-15 深度审计新增，不阻塞功能）

> 完整证据链见 `docs/AUDIT_2026-10-15.md` §3。按"先删死代码 → 再抽重复 → 最后统一写法"顺序做。

- [ ] **P2-1 死代码清理（9 处，零风险）**：`requireAdminOrRoot`（0 调用）、`var db *gorm.DB`（只写不读）、`var _ = models.Category{}`（占位 import）、`ImageUploader.tsx`（整个文件无人 import）、`Sidebar.tsx` 空 catch、`ThemeMode` 死类型（与 theme.tsx 冲突）、i18n 死键 15 个（`settings.*` 7 + `imgUpload.smms*` 8）、`UpdatePost` 注释重复两遍。
  ✅ **已修（2026-10-15 修复轮 2）**：删 `requireAdminOrRoot`、`var db`（Init 保留签名）、`var _ = models.Category{}` 及多余 import、`ImageUploader.tsx` 整文件、`types.ts` 的 `ThemeMode` 死类型、i18n `smms*`/`settings.*` 15 死键、`Sidebar` 空 catch 体改纯静默注释、`UpdatePost` 重复注释（修复轮 1 已清）。`go build`/`go vet`/`tsc`/`vite build` 全绿。
- [x] **P2-13 `CreatePostInput` 缺 `cover` 字段（跨栈契约漂移，第 33 轮）**：前端 `CreatePage` 创建文章时发 `cover`（PostEditor 封面功能），但后端 struct 无此字段——Go JSON decoder 静默丢弃未知字段，**创建时设的封面直接丢失**（仅编辑路径 `UpdatePostInput` 有 Cover）。这类漂移 `tsc`/`go build` 都发现不了，只能跨栈核对。
  ✅ **已修（2026-10-19 第 33 轮）**：`CreatePostInput` 加 `Cover string` + `CreatePost` 落库；回归 `post_cover_test.go` `TestCreatePostKeepsCover`（返回值 + DB 行双断言 + 空值不幻影）；新增 `scripts/contract_check.py`（前后端 method+path 自动核对工具，防复发）。`go build/vet/test` 全绿。
- [x] **P2-14 恢复快照前端无 UI（功能缺口，第 33 轮记录）**：后端 `POST /api/backup/restore-snapshot`（第 30 轮已改 root-only）存在，但前端零调用——root 想恢复快照只能 curl。AdminBackupPage 只有「导入 JSON」，没有「从快照恢复」入口（快照列表也没有展示）。建议：快照列表 + 恢复按钮（rootOnly）+ 确认对话框。
  ✅ **已修（2026-10-19 第 34 轮）**：后端新增 `GET /api/backup/snapshots`（root-only，`ListSnapshots` service + handler + router，回归 `backup_snapshots_test.go`：admin 403 / 缺目录返空 / 只列 snapshot-*.db / 大小正确）；前端 `api.ts` 加 `listSnapshots`/`restoreSnapshot`，AdminBackupPage 新增「快照恢复」区块（rootOnly：表格 + 每行恢复按钮 + 确认对话框 + 恢复后自动刷新列表）；i18n 11 个新键（zh/en，含 `admin.snapshotRestoreConfirm` 避免与回收站 `admin.restoreConfirm` 撞名——tsc 抓到的）。`go build/vet/test` + `tsc` + `vite build` 全绿；`contract_check.py` 确认 restore-snapshot 已接入（未调路由只剩 5 个合理项）。
- [x] **P2-15 `banPost` 死导出（第 35 轮记录）**：`api.ts` 的 `banPost(id, note?)` 全前端无调用（ban 入口不在文章列表/审核页 UI 上），其 `note` 参数后端 `BanPost` 也不接收（设计上 ban 不带 note，note 属于 send-back）。
  ✅ **已修（2026-10-19 第 35 轮）**：按审计建议删死导出（`tsc`+`vite build` 全绿）。**遗留产品决策**：封禁库功能当前**无 UI 入口可发起 ban**（只能在 `/admin/banned` 对已封禁文做 send-back/删除），即封禁库"只出不进"。若需要，应在某管理页（如审核页/文章管理）加 root-only 的"封禁"按钮并复用 `POST /api/admin/ban/{id}`（届时重新导出 `banPost`）。待产品决策。
- [x] **P2-2 重复代码抽取**：`load = useCallback` ×7、admin 表格三态 ×6、分页器 2 套实现、`writeError(400,"invalid request body")` ×15。
  🔶 **部分完成（2026-10-19 第 31+32 轮）**：
  - **fmtDate ×7 已全部收敛**（审计估值 ×7 正确：6 个 `fmtDate` + 1 个 `fmt`）——新建 `web/src/core/format.ts`（`fmtDate(iso, locale)` + `fmtDay(iso)`）；5 个 admin 页 + PersonalCenterPage 的本地定义全删；**AdminAuditPage 的 `fmt` 为有意差异化保留**（无效日期回显原值而非 "—"，审计行不隐藏源数据，已加注释），仅补 locale 一致性。**顺带修复**：原 7 处均不传 locale，英文界面日期仍按浏览器 locale 渲染（对齐 ListPage localeTag 模式）。
  - **Modal ×5 已全部收敛**（审计估值 ×5 正确：UserForm/ResetPassword/LoginHistory/ReviewReject/BanVaultSendBack，LoginHistory 为 `modal--wide` 宽变体）——新建 `web/src/ui/Modal.tsx`（title/onClose/children/footer/wide）；5 处本地 markup 全删。
  - `tsc` + `vite build` ✅。
  - **writeError(400,"invalid request body") ×16 已抽(第 36 轮)**:后端 handlers 16 处完全相同的调用收束为 `writeInvalidBody(w)` 辅助(`handlers/auth.go`),消除重复字符串;其余 `writeError` 非同质消息(如 backup too large / page must be integers / admin only)不动。`go build/vet/test` ✅。
  - **表格三态 ×6 已抽(第 37 轮)**:新建 `web/src/ui/TableState.tsx`(`loading`/`isEmpty`→`loadingText`/`emptyText`+`children`),6 个 admin 列表页(Categories/Tags/Trash/Review/BanVault/Users)的 `loading?spinner:empty?emptyMsg:table` 三元统一收束,消除"某页漏空态"隐患;`tsc`+`vite build` ✅。
  - **load ×7 / 分页器 2 套 经评估不做(第 37 轮)**:`load` 回调每个 fetch 不同数据/setter,且 Users 页在 `useEffect` 内按角色守卫后才 `load()`,抽泛型 hook 会改变行为、属过度抽象而非缺陷;分页器 2 套已在 P2-8 删传统分页器后实质收敛为无限滚动单一机制。
- [ ] **P2-3 同题异构统一**：limit 解析 3 种规则、删除响应风格 3 种、确认框 3 种用法、加载态命名 3 种、useEffect 依赖 2 种风格、异步竞态防护 2 种、解码错误消息 2 种、品牌回退值 3 处。
  🔶 **部分完成（2026-10-19 第 37 轮）**：
  - **limit 解析 3 种规则已统一(真改进)**:原 `auth.go`(`n>0&&n<=100`)/`accounts.go`(`err==nil` 无上下限)/`admin.go`(`n>0` 无上限)三套不一致,其中 2 处无上限、1 处无下限,纯靠 service 兜底=纵深防御缺口。新建 `handlers/query.go` 的 `parseLimit(q,key,def,max)`(clamp 到 `[1,max]`),三处统一调用,上限与 service 对齐(ListActions 200 / 登录历史 100),并加 `TestParseLimit`(9 断言,含 0/负数/超限/非数字回落)。`go build/vet/test` ✅。
  - **其余 7 类经核查为纯样式、行为已一致,不做机械统一(第 37 轮)**:① 删除响应——各端点返回结构本就不同(create→对象/update→对象/delete→`{ok}`),非 bug;② 确认框——前端已统一 `confirmDialog`/`window.confirm` 5 处一致;③ 加载态命名——`loading`/`logsLoading` 等命名差异不影响逻辑;④ useEffect 依赖——依赖数组写法两路但均正确(含 `reloadKey` 重拉,见 P1-10);⑤ 异步竞态防护——admin 列表页为单次加载无需请求 ID 守卫,ListPage 无限滚动用 `loadingRef` 防重复,各得其宜;⑥ 解码错误消息——`api.ts` 统一 `ApiError{status,data.error}` 已一致;⑦ 品牌回退值——站点名/描述 fallback 三处一致。均为"写法不统一但行为正确",统一收益低、改动面大,列为非缺陷。
- [x] **P2-4 硬编码文案**：`PostEditor` 2 处中文、`LoginPage` placeholder 泄露默认口令、`ReadPage` 字号标签、`AdminTrashPage` 唯一不走 i18n 的确认框。
  ✅ **已修（2026-10-15 修复轮 3）**：4 处全部接 i18n（新增 `auth.usernamePlaceholder/passwordPlaceholder/showPwd/hidePwd`、`editor.dragDivider`、`admin.restoreConfirm` 键，zh+en）；LoginPage 不再泄露默认口令提示。
- [x] **P2-5 `'draft'` CSS 类借用**：`post-status-draft` 已不是合法 PostStatus（3 处使用）。
  ✅ **已修（2026-10-15 修复轮 3）**：删 `index.css` 死类 `.post-status-draft`/`.post-status-scheduled`；`tsc`+`vite build` ✅。
- [x] **P2-6 `clientIP` 信任 XFF**：审计 IP 可伪造；生产经 Nginx 覆盖可缓解，建议加可信代理闸门。
  ⏳ **未修（降级评估）**：单实例直裸部署场景(本机/dev)无代理,XFF 本就来自客户端;生产部署走 Nginx 时由 Nginx 覆盖 XFF(见 deploy/nginx.conf),可信代理闸门需新增 `TRUSTED_PROXIES` 配置+中间件改造,收益随场景而变,列为 P2 待办。
- [x] **P2-7 `?page=abc` 静默落入 legacy 分支**：`posts.go:70-78` `Atoi` 错误被忽略 → 响应结构与请求意图相反。
  ✅ **已修（2026-10-15 修复轮 3）**：`ListPosts` 显式解析 `page/size/category`，任一显式提供但非数字 → 400；补回归 `TestListPostsRejectsBadPageParam`。
- [x] **P2-8 分页双机制并存**：`ListPage` 按钮分页 + IntersectionObserver 无限滚动互相触发。
  ✅ **已修（2026-10-19 第 27 轮）**：根因比描述更严重——翻页 effect 是 append 架构(page>1 即追加)，"上一页"按钮会把第 1 页**重复追加**进列表(真 bug)，且三入口(分页器/哨兵/加载更多)心智模型打架。修法：删传统分页器，统一为无限滚动(count 行显示进度 + 哨兵自动加载 + 加载更多按钮兜底)；清理死 CSS `.pagination`/`.page-info`。`tsc`+`vite build` ✅；UI 实机交互未验证(服务进程被会话回收，已知环境限制)，数据流由 handler 测试覆盖。
- [x] **P2-12 审核入口无状态前置校验（代码反推审计 Q1，第 35 轮）**：`ReviewPost`/`BanPost` 不检查当前状态——已 published 文章可被 reject 突然打回（公开内容无确认消失）；banned 可被 approve 直接平反跳 published（绕过 send-back 的作者修改环节）。
  ✅ **已修（2026-10-19 第 35 轮）**：`ReviewPost` 加状态前置——仅 `pending`/`rejected` 可审，对 `published`/`private`/`banned` 返回 409「post is not awaiting review」（设计取舍：不允许"重新审核已发布文章"，公开内容不可经审核入口被悄悄下线；banned 必须经过 send-back→作者修改→重新提交→再审，不能跳过作者修复直接平反）；`BanPost` 加"已 banned 不可重复 ban"守卫（409）。回归 `review_state_test.go` `TestReviewPostStatePrecondition`（published 拒 reject / private 拒审 / rejected 可再审 / banned 拒审 root+admin / 拒审后状态不变 / 重复 ban 409，共 9 断言）。`go build/vet/test` 全绿。
- [x] **P2-9 手写标准库函数**：`auth.go:126-142` 重造 `strings.IndexByte`/`LastIndexByte`。
  ✅ **已修（2026-10-15 修复轮 2）**：改标准库实现。
- [x] **P2-10 测试与生产包装不一致**：`handlers_test.go:205` 用 `WithAuth(ListPosts)`，router 用 `withOptionalAuth`。
  ✅ **已修（2026-10-15 修复轮 3）**：测试改 `WithOptionalAuth(ListPosts)` 对齐 router。
- [x] **P2-11 注释与实现漂移**：`models.go:18` 称 admin 可 ban 文章，实际 BanPost 是 root-only。
  ✅ **已修（2026-10-15 修复轮 3）**：注释修正为"admin 可审核 user 的文章，但不可 ban(ban 为 root 专属)"，与 `BanPost` 实现一致。（旧重复条目已合并至此）

## 🟡 P3 — 小瑕疵

- [x] **P3-1 tag 关联纳入事务（原子 + 错误传播）**：`CreatePost`/`UpdatePost` 的「主表写入 + 标签关联」现整体包在 `s.db.Transaction` 内；标签关联改用**手动写 `post_tags` 连接表**（`linkPostTagsTx`，避开 gorm `Association` 在事务回调内 panic 的坑），任何关联错误都会回滚整篇并返 500（不再 `_ =` 吞错）；`findOrCreateTagsTx` 新增并发唯一冲突回查。`handlers/feed_test.go` 之外新增 `services/post_tags_test.go` 回归（创建带空名/重复标签、更新替换/清空标签均正确）。
- [x] **P3-2 搜索 LIKE 通配符未转义**：`ListPostsPage` `"%"+q.Search+"%"`，搜 `%`/`_` 被当通配符（搜 `%` 退化为全表）。修：转义 `%` `_` 或用 `ESCAPE`。
  ✅ 已修（2026-10-14）：新增 `escapeLike`（转义 `\`/`%`/`_`），搜索配合 `LIKE ? ESCAPE '\'`；补 `TestEscapeLike` 单测。
- [x] **P3-3 限流桶淘汰是随机非 LRU**：`limiterFor` 超 512 时 `for k := range m { delete; break }` 随机删，可能误删活跃桶。改进：LRU 或环形。
  ✅ 已修（2026-10-14）：`Limiter` 改用 `container/list` 维护访问顺序（`lru` 链表 + `buckets` map 双向索引）；`limiterFor` 命中即 `MoveToFront`、超容删 `Back`（LRU）；`maxBuckets` 提为字段（默认 512，测试可缩）；新增 `TestLimiterLRUEviction`（验证 a→b→touch a→c 淘汰 b）+ `TestLimiterStaysBoundedUnderChurn`（500 个不同身份后缓存不超容，无内存泄漏）。
- [x] **P3-4 GORM `record not found` warn 刷屏**：首启播种/查询未命中一片红字。修：Logger 忽略 `gorm.ErrRecordNotFound` 或降 debug。
  ✅ 已修（2026-10-14）：`config.go` 新增 `quietLogger` 包装 gorm logger，`Trace` 中遇 `gorm.ErrRecordNotFound` 直接跳过不记录；其余错误/慢查询照常。
- [x] **P3-5 空文章**：`CreatePost` 不校验 body，可发空正文文章。按需决定是否要求正文非空。
  ✅ 已修（2026-10-14）：`CreatePost` 加 `strings.TrimSpace(in.Body)==""` 校验 → 400「post body must not be empty」。`UpdatePost` 保持部分编辑语义（空 body 不改），不加校验。补齐了测试里 12 处无 body 的 `CreatePost` 调用（`services_test.go`/`services_extra_test.go`/`post_tags_test.go`）加 `Body:"x"`。

## ✨ 功能缺口（增强方向，按价值排序）

- [x] **RSS / 站点地图**：博客刚需，利于订阅与 SEO，实现轻量。
  ✅ 已做（2026-10-14）：新增 `GET /feed`（RSS 2.0）+ `GET /sitemap.xml`（sitemap 0.9），公开、仅列 published 文章；`SiteConfig` 加 `site_url`（后台可填，留空则按请求 Host/X-Forwarded-Proto 推断绝对链接）；RSS `<description>` 为 markdown 剥离后的纯文本摘要；XML 由 `encoding/xml` 输出自动转义（防注入）；前端 footer 加 RSS/站点地图链接 + `<head>` RSS discovery；`handlers/feed_test.go` 回归（排除私密文、转义、sitemap 含链接）。
- [x] **安全中心落地**：补“修改密码 / 查看登录历史 / 登出所有设备”。
  ✅ 已做（2026-10-14）：① JWT 吊销——`User` 加 `token_version`，`SignToken` 写入 `ver` claim，`Authenticate` 校验版本（bump 后旧 token 401）；首部署无 `ver` 的旧 token 在 version=0 时仍兼容。② 新增 `svc.RevokeAllSessions` + `POST /api/auth/logout-all`（加白名单）+ 前端 `logoutAll()`。③ 自助登录历史 `GET /api/me/login-history`（`svc.ListUserLogins`，limit≤100）。④ 前端 `SecurityCenter` 由占位改为：账号信息（邮箱脱敏可切）+ 改密表单（复用 `changePassword`，前端校验 ≥8/一致）+ 登录历史列表（成功/失败 badge、IP/UA/时间）+「登出所有设备」（confirm→调 `logout-all`→清本地会话→跳登录）。i18n 中英双语文案 + `.sec-card`/`.login-list`/`.badge--ok/--fail` 样式。后端 `go build/vet/test` ✅、前端 `tsc`+`vite build` ✅；运行时冒烟验证吊销链路（旧 token 401、新登录 200）。
- [x] **图片管理**：目前只能上传，不能浏览/删除已上传图片。
  ✅ 已做（2026-10-14）：新增 `svc.ListLocalUploads`（列本地上传目录，按时间新→旧，空目录返 `[]` 非 null）+ `svc.DeleteLocalUpload`（按名删本地文件，严格校验防路径穿越，`..`/`/`/`\` 一律拒）；端点 `GET /api/uploads` + `DELETE /api/uploads/{name}`（均 JWT）。仅管本地存储图——SM.MS 远端图删除会留死链，故不在管理范围。前端 `SettingsPage` 加「图片库」区块（`ImageLibrary` 组件：网格预览 + 上传 + 逐张删除 + 大小/时间），i18n `imageLib.*` 中英 + `.image-grid`/`.image-card` 样式。运行时冒烟验证上传→列出→删除→再列 `[]`。
- [ ] **评论**：外接（Giscus/Disqus）或自建。
  📌 **未做（增强方向，非缺陷）**。个人博客场景下评论价值取决于读者互动意愿，且外接方案会引入第三方脚本/隐私考量。候选：Giscus（GitHub Discussions，零后端成本，但有 GitHub 账号门槛）/ 自建（需新表 + 审核 + 反垃圾，与第 25 轮审核流可复用）。**建议等有真实读者需求再启动。**
- [x] **操作审计日志**：目前只有登录历史；谁改了文章/配置无记录。
  ✅ 已做（2026-10-14）：新增 `ActionLog` 模型（ActorID/ActorName/Action/Target/Detail/IP/CreatedAt，AutoMigrate 建表）+ `svc.RecordAction`（best-effort 吞错，审计永不破坏业务操作）+ `svc.ListActions`（admin 端，limit 默认 50、上限 200，`created_at DESC, id DESC`）。内容侧 `CreatePost/UpdatePost/DeletePost` 成功后记审计；admin 侧 `CreateUser/UpdateUser/ResetPassword/DeleteUser/SetUserDisabled/UpdateSiteConfig/ExportBackup/ImportBackup/RestoreTrash/EmptyTrash` 全部经 `recordAudit` helper 注入（IP 取 `clientIP`）。端点 `GET /api/admin/actions`（`requireAdmin`）。前端新增 `AdminAuditPage`（管理员操作时间线：动作中文 verb 映射 + 目标 + 操作人 + IP + 时间，可搜索过滤）+ `/admin/audit` 路由 + 后台导航项 + i18n `audit.*` 中英（含 `verb` 子对象 13 动作名）。后端 `go build/vet/test` ✅、前端 `tsc`+`vite build` ✅。
- [x] **邮件通知**：改密/异地登录提醒（需 SMTP，低配机可选）。
  ✅ 已做（2026-10-14）：零新依赖的手写极简 SMTP 客户端（仅标准库 `net`/`crypto/tls`/`bufio`，纯文本邮件，支持 25/587/465 + STARTTLS + AUTH LOGIN）。`SiteConfig` 加 `SmtpHost/Port/User/Pass` 4 列（admin 后台可配，public 端点白名单不泄露密码），`runManualMigrations` 幂等补列（`0002_add_smtp_columns` gate，新库走 AutoMigrate、旧库 ALTER ADD，表不存在则跳过并记录 gate）。触发点：① **登录失败**（`Login` 失败分支，`SendMail(MailLoginFailure)`，按账号 15 分钟节流防爆破刷屏收件箱）② **改密**（`ChangePassword` 成功后）③ **异地登录**（`Login` 成功分支 `notifyNewLocation`：`meta.IP != user.LastLoginIP` 且非首次登录才告警，更新 last-login 状态前比对）。`SendMail` 全程后台 goroutine + 30s 超时 + 吞错——SMTP 故障永不阻塞/破坏主业务；`POST /api/admin/mail/test`（`svc.SendTestMail`）同步发一封到管理员自身邮箱以验证配置（未配置返 400、失败返 502，前端出 toast）。前端 `AdminSitePage` 加「安全通知（SMTP）」区块（host/port/user/password + 「发送测试邮件」按钮）+ i18n `site.mail*` 中英。验证：后端 `go build/vet/test` ✅（`mail_test.go` 9 用例 + `handlers/mail_test.go` 2 用例全绿）、前端 `tsc`+`vite build` ✅；**运行时冒烟**确认新库/旧库均正确加 4 列且重启幂等、登录失败在 SMTP 未配置时静默返回 401（best-effort 生效）、测试端点无 token 401。
- [x] **文章内容多语言**：~~UI 已 i18n，文章内容本身无多语言。~~ **已决（2026-10-14）：不做**——用户不面对多语言使用环境，此需求不适用，关闭该项。

## 📌 设计取舍（非缺陷，备查）

- 限流内存态（单实例）；搜索 `LIKE`（无 FTS5）；`X-Forwarded-For` 直连时可伪造（生产经 Nginx 覆盖可缓解）。JWT 吊销（`token_version`/`ver`）与操作审计（`ActionLog`）已补齐。
- **三层角色/审核流/封禁库**（第 25 轮，2026-10-15）：root 唯一且不可经 API 创建；banned 文章仅 root 可见；审核 reject 必带意见。规则见 `docs/specs/21-roles-moderation.md`。

---

## 实测确认“没问题”的项（避免重复排查）
- 后台接口后端鉴权：每个 admin handler 都调 `requireAdmin`/`requireRoot` → 403 ✅
- 软删：`Post.DeletedAt gorm.DeletedAt`，查询自动 `WHERE deleted_at IS NULL` ✅
- `view_count` 递增：`gorm.Expr("view_count + 1")` 原子，无竞态 ✅
- 分页边界：`page<1→1`、`size<1→20`、`>100→100` ✅
- 排序注入：`sort`/`order` 白名单 ✅
- 导入备份：事务内 Clear+Create，导入前自动 `VACUUM INTO` 快照可回滚 ✅
- 快照恢复：`filepath.Base` 防路径穿越 ✅
- 角色提权：`UpdateUser`/`CreateUser` 遇 `RoleRoot` 直接 403（root 不可经 API 创建）✅
- 封禁可见性：`canSeePost` 中 `banned` 仅 root 可达，作者本人与其他 admin 均被过滤 ✅
- 审核作用域：admin 队列只含 user 发文，admin/root 发文仅 root 可审 ✅

---

## 🗄️ 已归档（不再作为待办）

| 文档 | 归档时间 | 原因 |
|---|---|---|
| `docs/archive/2026-10-14-FUNCTIONAL_AUDIT.md` | 2026-10-15 | 首轮审计快照，结论已全部落入本文件并勾选；留档仅为追溯"当时发现了什么" |
