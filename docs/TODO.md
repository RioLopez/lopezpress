> 类别 A · 唯一待办清单 — 只列还没做完的。已完成的不在这里，也不留勾选历史。

# 待办清单（18 条）

> **维护规则：做完一条 → 从本文件删掉这一行**，不要打勾保留。
> 上一版有 409 行 / 79 KB，156 个条目里 139 个是已完成记录——它长胖的原因就是这个。
> 做完的事记在 `.workbuddy/memory/YYYY-MM-DD.md`（一行摘要 + 验证结果），那里才是历史。
>
> 编号（`T-` `S-` `F-` `M-` `E-` `P1-`）沿用历史编号，方便和代码注释、memory 日志对上。

---

## 一、现在就该做（5 条）

> 判据：真 bug、数据错误、或上线前置条件。

- [ ] **T-4 编辑器三栏在 1440 宽窗口下右侧面板被裁** —— 标签输入框只露出约 80px，等于没法用（真机走查发现）。待重排网格或加容器查询。**这是唯一一条「功能不可用」级别的。**
- [ ] **F-5 站点名仍是 `Echo`** —— `site_configs.site_name` 的 DB 现值。改名 LopezPress 属**数据修正不是代码问题**，后台「站点设置」改一次即可。
- [ ] **F-6 一条 `draft` 状态脏数据** —— 文章「封面测试」（第 22 轮已废 draft 五态）。第 73 轮起 `draft` 与 `private` 同档（作者可见、他人 404），「列表里有、点进去 404」的不一致已消除；剩下只是这条数据该迁成 published 或 private。
- [ ] **E-10 正确的验证码也会消耗猜测额度** —— `verifyEmailCode` 的 Step 1 在比对哈希**之前**就 `attempts+1`，于是并发或双击提交时，落后的请求也被扣一次；极端情况下 8 个并发请求直接把 5 次额度烧光，让一个**输对了**的码作废。修法：先比对 HMAC，只在**不匹配**时条件占额度（爆破防护不变，条件 UPDATE 仍守住 5 次上限），匹配则直接走条件消费。`TestEmailCodeCannotBeSpentTwice` 现在只能断言 `1..N`，修完可改回严格等于 1。
- [ ] **E-7 邮箱验证未用真实 SMTP 验证过** —— 目前只用本地假服务器（`scripts/fakesmtp.py`，故意不实现 STARTTLS）走过全流程。**若上线要开邮箱验证，必须先用真邮箱（QQ / Gmail / 自建 Postfix）验一遍 TLS 与退信。**

## 二、可以排期（7 条）

- [ ] **S-24 发布权限两套口径** —— `POST /api/admin/review/{id} approve` 允许 admin，但 `POST /api/admin/posts/batch` 的 `publish` 却 root-only。同一效果两个门槛，建议抽 `canPublish()` 统一（当前维持现状，属一致性债不是漏洞）。
- [ ] **P1-1 `roles.go` 集中权限层仍有手写分支** —— 文件头宣称「所有权限判断都走这里」，实际部分靠手写 if-else，漏一个就是 P0。第 63 轮已收口 `canManageUser`（`ResetPassword` 调用）与 `canReviewPost`（`ReviewPost` 调用），剩用户管理 2 方法与审核其余分支。回归由 `TestReviewPost*` 守着。
- [ ] **F-13 编辑器内 slug 编辑框** —— 接口已支持（`PUT /api/posts/{id}` 带 `slug`），前端 `PostEditor` 只是没加输入框。工作量小。
- [ ] **M-10 回收站缺单条永久删除** —— 现在只能「删进回收站 → 再清空」两步，单条走不通（连带 M-9 的台账也只在清空时才清）。
- [ ] **M-8 缩略图失败即终态** —— `thumb_status = failed` 不会自动重试，需管理员手工清状态。解码失败的图本来也不值得重试，暂时压着可接受。
- [ ] **M-9 引用台账只在通过 API 保存/删除文章时同步** —— 绕过 API 直接改库会产生孤儿引用与 `ref_count` 偏差。曾计划做「夜间全目录对账」，**未做**。第 108 轮把四处引用来源（正文、封面、头像、站点 logo/favicon）都接进了台账，并让整库重建与回收站恢复都走重建，所以正常操作路径已闭合；**剩下的是"有人直接改库"这一种**，仍靠手工跑 `rebuildUploadRefsTx`。
- [ ] **前端行数门禁** —— `AdminUsersPage` / `AdminSitePage` 各 522 行。后端有 `R2.files ≤350` 的棘轮，前端没有对应规则。

## 三、决定不做 / 暂缓（6 条，别再提）

> 这几条已经被否过。**没有新理由不要重新提议**，否则等于让用户重复拍板。

- [ ] **AR-1b 状态码语义化** —— 295 处 `http.StatusXxx` 让领域层仍在说 HTTP 数字。真正做法是 `newErr(ErrNotFound, …)` + 单一映射表，而不是复制整数常量。**用户 2026-10-24 决定跳过**（收益不抵改动面）。
- [ ] **P2-3 同题异构其余 7 类** —— limit 解析 3 种已统一（真改进）；其余 7 类经第 37 轮核查**为纯样式、行为已一致**，不做机械统一。
- [ ] **E-8 无退信处理** —— 地址填错时用户只能等限流窗口过去再重发，系统不知道信没送到。属增强，不是缺陷。
- [ ] **E-9「同 IP 每小时 10 封」是内存固定窗口** —— 重启清零、多实例各算各的。当前单实例部署，可接受。
- [ ] **评论** —— 外接 Giscus/Disqus 或自建。个人博客场景下取决于真实读者互动意愿，且外接会引入第三方脚本与隐私考量。**建议等有真实读者需求再启**。
- [ ] **F-12 WXR（WordPress XML）导入** —— Markdown 批量导入已覆盖自建站迁移；WXR 格式脏、体积大，暂未做。

---

## 四、参考区（不是待办，但别删）

### 4.1 实测确认「没问题」的项（重新怀疑前先看这里）

> 已经验过并有证据。**重复怀疑会白花时间**——先看这里有没有。

- 后台接口后端鉴权：每个 admin handler 都调 `requireAdmin`/`requireRoot` → 403
- 软删：`Post.DeletedAt gorm.DeletedAt`，查询自动 `WHERE deleted_at IS NULL`
- `view_count` 递增：`gorm.Expr("view_count + 1")` 原子，无竞态
- 分页边界：`page<1→1`、`size<1→20`、`>100→100`；`sort`/`order` 走白名单
- 导入备份：事务内 Clear+Create，导入前自动 `VACUUM INTO` 快照；结构闸 + 确认词 + 全站 token 失效
- 快照恢复：`filepath.Base` 防路径穿越
- 角色提权：`UpdateUser`/`CreateUser` 遇 `RoleRoot` 直接 403（root 不可经 API 创建）
- 封禁可见性：`canSeePost` 中 `banned` 仅 root 可达，作者本人与其他 admin 均被过滤
- 审核作用域：admin 队列只含 user 发文，admin/root 发文仅 root 可审
- 软删图不会复活给下一个人：`live_key` 唯一索引，撤下即轮换为 `d:<id>`（实测：撤下后重传同一份字节，返回新文件而非复用）
- 软删图仍能服务已发布文章：`upload_files.DeletedAt` 是普通 `*time.Time`，`GET /uploads/{name}` 不看该字段
- 开着邮箱验证时 `PUT /api/auth/me` 改邮箱：403，无法绕过验证码（有单测）

### 4.2 设计取舍（非缺陷，备查）

- 限流内存态（单实例）；搜索 `LIKE`（无 FTS5，纯 Go 驱动不支持）；`X-Forwarded-For` 直连时可伪造（生产经 Nginx 覆盖可缓解）。JWT 吊销（`token_version`）与操作审计（`ActionLog`）已补齐。
- **三层角色 / 审核流 / 封禁库**：root 唯一且不可经 API 创建；banned 文章仅 root 可见；审核 reject 必带意见。完整规则在 `dev/ai-dev/MASTER.md` 第 5.1 节。
- **不做复杂度密码规则**：按 NIST SP 800-63B 走「长度 + 常见密码表」，不加「必须含大写/符号」。理由：复杂度规则把人推向 `Password1!`，而它就在字典第一页。

### 4.3 历史去哪了

| 内容 | 位置 |
|---|---|
| 139 项已完成待办的完整记录 | `C:\Users\LopezPC\Desktop\Lopez\_doc_backup_20261027_before_prune\docs\TODO_ARCHIVE.md`（仓库外） |
| 每轮改了什么、怎么验的 | `.workbuddy/memory/YYYY-MM-DD.md`（按日）；第 1–95 轮细节在仓库外备份的 `dev/ai-dev/PROGRESS*.md` |
| 被删的 86 份文档清单与理由 | `dev/ai-dev/MASTER.md` 第 12 节 |
