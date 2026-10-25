> 类别 A · AI 指挥/约束（母文档族）— 由根目录 `AGENTS.md` 统管，新对话须先读。

# lopezpress 待办清单（2026-10-15 建立 · 2026-10-24 第 79 轮校准）

> 来源：功能逻辑审计 + 二轮严格自查 + **2026-10-15 全栈深度审计**（均基于读实现，非猜测）。
> 完成后把 `[ ]` 改 `[x]`。
> **本文件是唯一权威待办清单**。首轮审计快照已归档至 `docs/archive/2026-10-14-FUNCTIONAL_AUDIT.md`（勿再当作待办）。
>
> **当前状态（2026-10-24 第 79 轮后，勾选状态与正文结论逐条对齐）**：
> - 早期 P1/P2/P3 与功能缺口全部完成（见下方各节勾选）；
> - **P0 级 7 项已全部修复**（P0-1～P0-6 修复轮 1，P0-7 第 30 轮）；
> - **P1 级仅剩 P1-1 余部**（roles.go helper 全量化，架构美化非缺陷，暂缓）；P1-17（root 密码恢复）第 39 轮已修；P1-15/P1-16 第 28/29 轮已修；P1-11 的计数行（写死 20）第 43 轮随分页改造修掉；
> - P2 级仅剩 **P2-3 剩余 7 类（经核查为纯样式，不做）**；**P2-6（可信代理闸门）第 43 轮已修**；P2-16（无用依赖）第 39 轮已修；
> - **架构债 AR-0～AR-8 全部完成**（第 54–67 轮），只剩 AR-1b 状态码语义化——用户 2026-10-24 决定跳过；
> - **媒体与存储（M 系列，第 77–78 轮）**：主体完成，剩 3 项非阻塞改进（缩略图无自动重试、台账不对账、回收站只能整清）；
> - **邮箱验证（E 系列，第 79 轮）**：三条流程完成且默认关闭，剩 3 项（真实 SMTP 未验、无退信处理、IP 闸是内存态）；
> - 「评论」仍为唯一明确不做的方向（用户 2026-10-22 决定）；SEO/OG 因纯 SPA 抓不到卡片，暂缓。
>
> **第 43 轮新增待办（产品功能，用户已拍板，尚未实现）**：
> - [x] **F-1 后台文章管理页 `/admin/posts` + 批量操作** ✅ 第 44 轮完成（spec 26）
> - [x] **F-2 后台分区导航**（内容 / 用户 / 系统三组）✅ 第 44 轮完成
> - [x] **F-3 前台首页编辑/删除按钮越权显示** ✅ 第 69–70 轮完成：个人中心「内容管理」先给作者编辑入口（第 69 轮），首页 `ListPage` 再加 `canEditPost`/`canDeletePost` 条件渲染（第 70 轮）——不再对所有人显示，非作者不会再点出 403。
> - [x] **F-4 `/archive` 全量加载** ✅ 第 47 轮完成（spec 28）：`GET /api/posts/archive/paged`（默认 50 / 上限 200 / body 恒空），前端改用服务端分页。
> - [ ] **F-5 站点名仍是 `Echo`**（`site_configs.site_name`，DB 现值），改名 LopezPress 属数据修正，非代码问题。
> - [ ] **F-6 一条 `draft` 状态脏数据**（文章「封面测试」，第 22 轮已废 draft 五态）。第 73 轮起 `draft` 与 `private` 同档（作者可见、他人 404），不再出现「列表里有、点进去 404」的不一致；剩下只是这条数据本身该迁移为 published/private。
> - [x] **F-7 前台匿名阅读** ✅ 第 46 轮完成（spec 27）：改为 `site_configs.public_read` 开关，管理员在「站点设置 → 显示设置」切换；前后端同时执行（匿名关闭时 API 直接 401）。
> - [x] **A-06 分类分层级** ✅ 第 46 轮完成（spec 27）：`parent_id` 两层树 + 预设分类树 + 删除父级时子级上移 + 父级筛选含子级。
> - [x] **A-05 封禁闭环** ✅ 第 48 轮完成（spec 30）：后台文章管理页加 root 专属「封禁」按钮，封禁库不再是单向门。
> - [x] **I-03 文章永久链接** ✅ 第 48 轮完成（spec 29，方案 A）：`/p/{slug}` + 旧 `/read/{uuid}` 永久保留；历史文章启动回填；sitemap/RSS 用 slug。
> - [x] **F-8 仪表盘只有两个数字** ✅ 第 52 轮完成（spec 33）：30 天去重浏览量趋势（内联 SVG，不引图表库）+ 待审计数 + 最多阅读 Top5。
> - [x] **F-9 备份只能手动点** ✅ 第 52 轮完成（spec 34）：每日自动快照（root 开关 + 保留份数 1–90），启动时 + 每 30 分钟惰性检查，`auto-YYYYMMDD.db` 与导入快照互不干扰。
> - [x] **F-10 读完一篇没有出口** ✅ 第 52 轮完成（spec 35）：相关文章（同分类/同标签，≤5 篇）+ 阅读页面包屑。
> - [x] **F-11 历史数据 `posts.category_id = 0`** ✅ 第 52 轮修正为 `NULL`（3 篇）：0 不指向任何分类，属于无效外键，会让后台分类列显示空。
> - [ ] **F-12 WXR（WordPress XML）导入**：Markdown 批量导入已覆盖自建站迁移；WXR 格式脏、体积大，暂未做。
> - [ ] **F-13 编辑器内 slug 编辑框**：接口已支持（`PUT /api/posts/{id}` 带 `slug`），前端 `PostEditor` 表单未加输入框。
> - [x] **隐藏 bug：保存站点设置 500** ✅ 第 46 轮发现并修：`site_configs.site_url` 列在旧库中缺失（AutoMigrate 不碰已存在的表）。修法为迁移 `0008` + 启动时的 `checkSchemaDrift` 漂移告警。

## 🔒 安全（2026-10-24 第 72 轮：外部渗透报告核实后新增）

> 报告 11 项 → 9 真 / 5 假阳性。假阳性理由见 `PROGRESS.md` 第 72 轮，**不要重复排查**。

- [x] **S-1 文章载荷泄露作者 `email`/`last_login_ip`**（HIGH 级）：`models.Post.SanitizeAuthor()` 覆盖 9 个出站口，仅 `ListPending` 保留 email。
- [x] **S-2 全站缺安全响应头**：`middleware.SecurityHeaders`（nosniff / `X-Frame-Options` / Referrer-Policy / CSP / HSTS 仅 https）。
- [x] **S-3 上传只验扩展名**：加 `http.DetectContentType` magic-bytes 校验，HTML 伪装 `.png` → 400。
- [x] **S-4 改密码不失效旧 token**：`ChangePassword` bump `token_version` 并回发新 token；前端 `adoptToken()` 续用会话。
- [x] **S-5 管理员重置密码踢不动会话**：`ResetPassword` bump 目标 `token_version`。
- [x] **S-6 登录只认 `email` 字段**：`LoginInput.Username` + 统一 identifier。
- [x] **S-7 dev 下 `/uploads` 返回 `index.html`**：`vite.config.ts` 补 4 条 proxy。
- [x] **S-8 生产 nginx 漏反代**（报告未测出）：`/uploads/` `/feed` `/sitemap.xml` `/robots.txt` 各自 `location`。
- [x] **S-9 nginx 静态 location 丢安全头**（报告未测出）：`add_header` 不继承，需重复声明。
- [ ] **S-10 普通用户编辑器暴露「私密」选项但存不下来**（`CreatePost` 强制 pending）——**等用户拍板**：隐藏选项 or 明确提示「先过审核」。第 73 轮把 `private` 收成严格作者可见后，这个入口更矛盾：用户选了私密，实际进审核队列，通过后变公开。
- [ ] **S-11 `PostEditor` 把 `pending` 误显示为「公开」**——**等用户拍板**。
- [x] **S-12 「私密」对管理员可见**（用户真机发现 admin 能读他人 private）：`canSeePost` 拆出独立分支只判作者，`PostDraft` 同档；`ListPostsPage` 用同一份 `authorOnly` 名单驱动过滤。⚠️ **改可见性必须同时改列表过滤与单篇判定**，否则出现"列表有、点进去 404"。

## 🔒 安全（2026-10-24 第 75 轮：外部渗透报告 root/admin 权限面，全部处理完）

> 报告 15 项 → 8 类真问题已修 / 4 项假阳性（假阳性理由见 `PROGRESS.md` 第 75 轮，**不要重复排查**）。

- [x] **S-13 备份导入可清库**（灾难级）：`{}` 曾把全站表删空。现三道闸——结构闸（users 非空 + 每行有 id/username/email + 至少一个 root）、`?dry_run=1` 预检、`confirm:"REPLACE-ALL-DATA"` 确认词。
- [x] **S-14 备份导出/下载未收归 root**：导出 JSON 含 `password_hashes`，`auto-*.db` 含完整 users 表；两个端点改 `rootRoles`，前端导出按钮仅 root 渲染。
- [x] **S-15 `site-config` 泄露 SMTP 明文口令**：非 root 读到 `smtp_pass`/`img_host_token` 为空串；`PUT` 时这些字段与 `allow_register`/`auto_backup`/`backup_keep` 一起继承库值；前端字段 `disabled`。
- [x] **S-16 `friend_links` 可存 `javascript:`**：`UpdateSiteConfig` 强制 http(s)、上限 50 条/500 字符。
- [x] **S-17 普通用户可自设置顶**：`CreatePost`/`UpdatePost` 只允许 admin/root 把 pinned 由 false 改 true；`ReviewPost` 通过 user 文章时清 pin；编辑器对普通用户隐藏开关。
- [x] **S-18 用户名 Unicode 同形字 / 大小写变体**：`isValidUsername` ASCII 3–32；`LOWER(username)`/`LOWER(email)` 查重。
- [x] **S-19 上传只校验魔数**：PNG/JPEG/GIF 追加 `image.DecodeConfig`（真实可解码 + 边长 ≤12000 + 像素 ≤4000 万）；WebP 仍只有魔数。
- [x] **S-20 恢复备份复活旧 token**：`ImportBackup`/`RestoreSnapshot` 事务末尾全站 `token_version + 1`，恢复后必须重新登录。
- [x] **S-21 admin 可窥 root 登录记录与邮箱/IP**：`GET /api/users/{root}/logins` 对非 root 403；用户列表对非 root 置空 root 的 email/ip。
- [x] **S-22 XFF 取最左一跳 → nginx 透传时可伪造**：**第 81 轮修复**。`clientip.Client` 改从右往左剥离可信代理、取最右非可信地址，所以 `$proxy_add_x_forwarded_for` 的追加写法下客户端伪造的前缀变成惰性；`deploy/nginx.conf` 的覆盖写法仍可保留作为纵深防御。第 R3-04 轮实测：伪造前缀下 520 次登录 4.4s 全部放行，修复后同一伪造被归并到同一个 IP 桶。
- [x] **S-25 admin 可改写并发布他人文章正文 + 读全部修订**：**第 82 轮已修（决策：收紧）**。`UpdatePost` 现在对 admin 修改他人 `title`/`body`/`cover` 一律 403（提示改用审核意见让作者改）；admin 保留全部审核动作（下架为 private、改分类、改标签、approve/reject）。修订权限拆成两个闸：`assertCanReadHistory`（作者/admin/root，审核需要看上下文）与 `assertCanWriteContent`（仅作者/root，因为恢复旧版等于把正文塞回已发布文章）。前端同步：`ListPage.canEditPost` 只对作者/root 生效（新增 `canModeratePost` 保住删除权），`EditPage` 加权限守卫避免 admin 直连 `/edit/{id}` 撞 403。跨用户操作现在在 `action_logs` 里记 `by=<操作者>; author_id=<作者>`。

- [x] **S-26 恢复快照只回滚 5 张表**：**第 82 轮已修（决策：不回滚是正确设计，但要修它带来的副产物）**。审计日志/登录日志**不应该**被恢复覆盖 —— 否则一次恢复就能抹掉攻击痕迹，所以"内容回滚、痕迹不回滚"是有意为之。真正的问题是副产物：恢复替换了 `posts` 却没动 `upload_refs`/`ref_count`，图库的"删除这张图会破坏 N 篇文章"提示变成假话（要么吓退安全删除，要么放行破坏性删除）。新增 `rebuildUploadRefsTx` 在 Import/Restore 事务末尾按新正文**批量重建**引用台账（一次读映射 + 批量插入 + 一条 SQL 重算，与文章数无关）。

- [x] **S-27 账号锁定可被武器化**：**第 82 轮已修（决策：分级锁定 + 放宽 IP 桶）**。锁定从"5 次错口令 → 固定 15 分钟"改为**升级制**：第 1 次 60 秒、第 2 次 5 分钟、第 3 次起 15 分钟；成功登录清零并回到 60 秒档。理由：完全豁免管理员会让爆破速率放大 450 倍（锁定期内 0 次 → 0.5 rps 持续尝试），而 60 秒首档已足以挡住脚本、又不至于把管理员锁在门外。同时 IP 桶 burst 3→10，缓解同 NAT 互相连坐（爆破防线在账号桶 + 锁定，不靠 IP 桶）。锁定时记 `slog.Warn`。

- [x] **S-28 审核/删除/回收站并发竞态**：**第 82 轮已修**。新增 `transition.go`：状态前置条件放进 UPDATE 的 `WHERE`，`RowsAffected == 0` 即"别人先动了"→ 409，绝不静默重试。覆盖 `ReviewPost`、`BanPost`、`SendBackPost`、`DeleteBannedPermanently`（条件硬删 + 事务内清 tag）、`DeletePost`（条件软删）、`RestoreTrash`（用读到的 `deleted_at` 精确匹配）。另修两个同类问题：`UpdatePost` 原本整行 `Save` 会把读到的 status 写回，作者改错别字时会**静默撤销管理员的 approve**；改为只写作者字段 + 状态走独立条件转换（顺带修好"编辑覆盖 view_count"）。全部用文件库并发测试验证（`newFileTestService`）。

- [x] **S-23 admin 可删任意用户上传的媒体**：**第 77 轮解决**——上传有了 `upload_files.owner_id` 元数据，`services.DeleteUpload` 按"谁"判定：非 admin 只能删自己的且受 `allow_user_delete_upload` 开关限制，硬删仅 admin/root 且返回受影响引用数。同名双路由（`/api/uploads/{name}` admin、`/api/me/uploads/{name}` 本人）都进同一个 service 方法。
- [ ] **S-24 发布权限两套口径**：`POST /api/admin/review/{id} approve` 允许 admin，`POST /api/admin/posts/batch` 的 `publish` 却 root-only。同一效果两个门槛，建议抽 `canPublish()` 统一（当前维持严格侧，未改）。

## 🧩 分类 / 标签体验（2026-10-24 第 74 轮，全部完成）

- [x] **T-1 分类链接是数字**（`?category=2`，而标签是 `?tag=Go`）：`?category=` 改收分类名，数字 id 兼容；未知分类返回空页而非全量；`categoryFilterIDs` 负责解析。
- [x] **T-2 前台看不出分类层级**：侧边栏父/子拉开字号字重 + 缩进 + 肘形引导线；筛选与编辑器下拉用 `　└ ` 前缀；卡片 chip 与阅读页面包屑显示「父 › 子」。逻辑抽到 `core/taxonomy.ts`。
- [x] **T-3 标签只能手输、大小写会分裂**：编辑器改自动补全（↑↓/回车/Esc/Backspace）+ 无匹配时「创建标签」入口；后端 `findOrCreateTagsTx` 改 `LOWER(name)` 查重、`?tag=` 与 `UpdateTag` 改 `COLLATE NOCASE`。补全逻辑抽到 `core/tags.ts`。
- [ ] **T-4 编辑器三栏在 1440 宽窗口下右侧面板溢出被裁**（真机走查发现，非第 74 轮引入）：标签输入框只露出约 80px，等于没法用。待重排网格或加容器查询。

## 💾 媒体与存储（2026-10-24 第 77–78 轮，spec 36）

> 设计依据 `docs/specs/36-media-library.md`（Accepted）。核心不变式：**`live_key` 唯一索引**——活跃=sha256、软删=`d:<id>`、历史=`legacy:<name>`。因此"已撤下的图不会复活给下一个人"由数据库保证，不靠调用方记得判断。

- [x] **M-1 上传无元数据、文件名可枚举、无法溯源**：新增 `upload_files`（sha256 / live_key / owner_id / ip / 尺寸 / 原名 / kind）与 `upload_refs`（文章引用台账）；文件名改 `crypto/rand` 16 字节 → base62 22 字符，URL 不泄露上传时间、用户、原名。
- [x] **M-2 同一张图重复上传会存多份**：sha256 去重，命中返回 `deduped=true` 与原文件名，不写盘不占配额。
- [x] **M-3 磁盘无上限**：三层配额（单文件分级：正文 5 MB / 头像 512 KB；单账号 20 MB；全站 500 MB）。全站统计**含软删行**（字节仍在磁盘），个人统计只算活跃行。顺序是"去重 → 配额 → 写盘"，被拒的请求不留半截文件。
- [x] **M-4 用户传错了删不掉**：软删即"撤下"——文件仍被 `GET /uploads/{name}` 服务（已发布文章不破图），但离开所有图库列表且**永不可被他人复用**（`live_key` 已轮换）。管理员可在后台看到"被谁撤下"并执行硬删（即便有文章引用）。
- [x] **M-5 后台看不到谁传了什么**：`/admin/media` 图库页：统计条（含"已撤下仍占用"）、状态/类型筛选、缩略卡片 + 引用数徽标、详情弹窗（上传者/IP/时间/尺寸/sha256/引用列表）、撤下/恢复/彻底删除、分页 24。
- [x] **M-6 头像可指向任意 URL**：默认只接受本站图库 `/uploads/<live-name>`；站长显式开启 `allow_external_avatar` 后才允许 http(s) 外链。前端 canvas 1:1 裁剪 + 拖拽微调 → 512×512 WebP，后端只校验不修图。
- [x] **M-7 图库列表加载原图很重**：异步缩略图队列。**待办状态在数据库**（`thumb_status`），内存 channel 只做唤醒且满了就丢；单 goroutine + 每张之间 150 ms；启动扫描 200 条封顶 + **30 s sweep 补积压**。长边 480，CatmullRom，不透明→JPEG q82 / 有透明→PNG。
- [ ] **M-8 缩略图失败即终态**：`failed` 不会被自动重试，需管理员手工清 `thumb_status`。够用但不够优雅（解码失败的图本来也不值得重试，暂时压着）。
- [ ] **M-9 引用台账只在通过 API 保存/删除文章时同步**：绕过 API 直接改库会产生孤儿引用与 `ref_count` 偏差。spec 36 §8 提过"夜间全目录对账"，**未做**。
- [ ] **M-10 回收站清空才清台帐**：单条永久删除（T-5）还没有，所以用户只能"删进回收站 → 再清空"两步。

## ✉️ 邮箱验证（2026-10-24 第 79 轮，spec 37，**实验性**）

> 默认关闭，界面标「测试功能」，开关 root 专属。设计依据 `docs/specs/37-email-verification.md`。

- [x] **E-1 注册不校验邮箱真实性**：开关打开时 `POST /api/auth/register` 必须带 `code`。
- [x] **E-2 忘记密码无路可走**（此前只能找管理员重置）：`/forgot-password` 页 + `POST /api/auth/password/reset`；成功后 bump `token_version`，所有会话失效。
- [x] **E-3 换邮箱会绕过验证**：`/settings` 改为"向新地址发码 → 确认"两步（收到码本身就是所有权证明），成功后给**旧**地址发变更通知；同时 `PUT /api/auth/me` 在开关打开时改邮箱直接 **403**——否则一次 PUT 就能绕过整套验证码。
- [x] **E-4 六位码可爆破**：每码最多 5 次错误、一次性、绑用途（HMAC 参与 `purpose`）、库里只存 HMAC（key=`JWT_SECRET`）；同邮箱 60 s 冷却（存表，重启不清零）+ 同 IP 每小时 10 封。
- [x] **E-5 "发送成功"但其实没配 SMTP**：`requireSMTP` 显式返回 503 并说明原因，不让前端显示"已发送"却收不到信。
- [x] **E-6 开关权限**：`email_verify_enabled` 与两个时限 root 专属；普通 admin 保存时三字段从库值继承（`allow-*` 类字段一律"未提交 = 继承"，否则旧表单会静默关掉）。
- [ ] **E-7 未用真实 SMTP 验证过**：只用本地假服务器（`scripts/fakesmtp.py`，故意不实现 STARTTLS）走过全流程。上线前必须用真邮箱（QQ / Gmail / 自建 Postfix）验一遍 TLS 与退信。
- [ ] **E-8 无退信处理**：地址填错时用户只能等 60 s 重发，系统不知道信没送到，`email_codes` 里那条记录仍然"有效"。
- [ ] **E-9 "同 IP 每小时 10 封"是内存固定窗口**：重启清零、多实例各算各的。当前单实例部署，可接受。


## 🏗️ 架构债（2026-10-23 第 53 轮架构复审新增）

> 完整证据链 + 目标分层 + 验收指标见 `docs/ARCH_REVIEW_2026-10-23.md`（§2 一致性问题、§3 架构问题、§6 迁移路线）。
> **与下方 P0/P1/P2 的区别**：下面是"功能/安全 bug"，这里是"代码组织债"——不改也能跑，但每加一个功能就更难改。
> **顺序不能乱**：先 P0 立规矩（否则后面每轮都会长出新写法），再搬代码。

- [x] **AR-0 立机械门禁 `scripts/arch_check.py`（零行为变化，最高性价比）** ✅ 2026-10-23 第 54 轮完成
  已实现 **7 项计数 + 1 条待补**（R6 错误构造需 AST 级检查，随 AR-2 落地）：R1 传输符号、R2.files/R2.funcs 文件与函数长度、R3 service 包 struct 堆积、R4 handler 内 `require*`、R5 handler 内 `recordAudit`、R8 前端裸 `fetch`、R9 注释块内重复行。
  **R3 口径演进**：第 66 轮将 24 个散落在 15 个业务文件的 DTO 全部收敛进 `dto.go`（现 44 个 struct = 全 API 契约单一入口），R3 门禁口径从「services 包 struct 总数」改为「散落在 `dto.go` 之外的 struct」（更精准拦「DTO 漂移」），R3 从 44 → **0**；`dto.go` 因纯声明文件超 350 行，加 R2.files 豁免（声明文件长度=API 面，非逻辑堆积）。
  基线存 `scripts/arch_baseline.json`，棘轮语义（只拦变多）。
  已接入 `AGENTS.md` §4 验证门禁（改代码必跑）。已顺手修两个小缺陷：`checkSchemaDrift` 改由新增的 `models.All()` 驱动（原先手写 9/11，漏 `PostRevision` 与 `SchemaMigration`，且与 AutoMigrate 的清单是两份独立副本）；4 处 doc comment 重复（`router.New`、`Service.Register`、`ImportBackup`、`handlers.Register`）。`GetPostPublic` 的 "Increments view_count" 注释已在此前轮次修正，无需再动。
  门禁：`go build/vet/test` 全绿（29 个包/8 个测试包）；`arch_check.py` 退出 0。
- [x] **AR-1 拿回分层边界** ✅ 2026-10-23 第 55 轮完成
  已完成：`ServeBackup` → `ResolveBackupFile(name) (path, size, err)`；`ServeUploads` → `ResolveUploadFile(name) (path, err)`；`UploadImage(ctx, r *http.Request)` → `UploadImage(ctx, filename, data)`（`_ = ctx` 消失，ctx 真正传进 `GetSiteConfig` 与图床调用）；出站 SM.MS 客户端抽出 `internal/imghost` 包。
  **R1 口径同时修正**（不是"调低门槛"）：旧口径数"import net/http 的文件数"=11，其中 10 个只是 `http.StatusXxx` 整数常量——换成自定义常量是改名，零架构收益。新口径数**真正耦合传输的符号**（`ResponseWriter` / `*http.Request` / `ServeFile` / `NotFound` / `http.Client` / `NewRequest` / `MaxBytesReader` / `http.Handler`），P1 前 12 处 / 1 个文件 → 现在 **0**。剩下 11 个文件只 import 状态码常量，记为新观察项 **R1b=11**（只显示、不阻断）。
  行为验证（真跑，非推断）：上传→取回字节一致 + `Content-Type: image/png`；`Range: bytes=0-9` → 206；`If-Modified-Since` → 304；非图片扩展名 → 400 白名单文案；缺 `file` 字段 → 400；未授权 → 401；`/uploads/../go.mod`、`%2f` 编码、子路径 → 全 404；备份下载 200 + `Content-Length` 与实际字节一致 + `Content-Disposition` 正确；备份名穿越（`../app.db`、`a%2Fb.json`、空名）→ 全 404。
  **测试移动**：`services/upload_test.go` 随被测代码迁到 `imghost/imghost_test.go`，断言不变，另加 1 条 `TestUploadTimeoutIsBounded`（Q11 的防挂死属性）。
- [ ] **AR-1b（可选，低优先级）状态码语义化**：295 处 `http.StatusXxx` 让领域层仍在说 HTTP 数字。真正做法是把 `newErr(404, ...)` 换成 `newErr(ErrNotFound, ...)` + 单一映射表，而不是复制一份整数常量。收益是领域可脱离 HTTP 表达错误，成本是 295 处替换 + 全量测试复核，故不并入 AR-1。**2026-10-24 用户决定跳过**（收益不足，风险不低）。
- [x] **AR-1c（真 bug）上传超限返回 400 而非 413** ✅ 2026-10-23 第 63 轮完成
  原：`MaxBytesReader(maxUploadBytes())` 在 `ParseMultipartForm` 阶段就截断，导致 `header.Size > maxUploadBytes()` 的 413 分支走不到（死代码），用户看到的是 `invalid multipart form`（400）。
  修：`handlers/upload.go` 的 `ParseMultipartForm` 错误处理里，用 `errors.As(*http.MaxBytesError)` 单独映射成 413 `"file too large"`（与 `ImportBackup` 的 P1-6 修法同构）；`header.Size` 检查保留（防御"声明超限但 body 未超"的伪造）。
  回归 `handlers/upload_test.go` `TestUploadImageOversizeReturns413`（超限 multipart → 413）。
- [x] **AR-2 审计单点化** ✅ 2026-10-23 第 56 轮完成
  已完成：新建 `internal/services/audit.go` 的 `func (s *Service) audit(ctx, actor, action, target, detail string)`，是**全库唯一** `models.ActionLog` 构造点（R10 门禁拦其余文件）；`WithActor`/`ActorFromContext`/`ClientIPFromContext`/`SystemActor` 把「谁在动、从哪来」经 context 从 JWT 中间件传给 service（handler 不再传 `*http.Request` 给审计）。
  handler 侧 18 处 `recordAudit` 全部下沉为 service 写方法内的 `s.audit(...)`（35 处调用），`recordAudit`/`RecordAction` 双轨删除——R5 18→0、R10=0。
  8 个缺口全部补齐：分类增删改（`create/update/delete_category`）/ 上传（`upload_image`）/ 删图（`delete_upload`）/ 发文章（`create_post`）/ 预设分类（`apply_preset_categories`）/ 恢复修订（`restore_revision`）/ 批量操作（`batch_*`）/ 导入 Markdown（`import_markdown`）。
  审计是 best-effort（插入失败只 `slog.Warn` 不破坏主操作）；刻意不审 Login/Register/UpdateMe（Login 已有 `login_log`），改密（admin 强制 + 自助）例外都审。
  回归 `internal/services/audit_test.go` 3 用例：AC-1 调用写用例即产生审计行、AC-2 显式 actor 优先于 context、AC-3 无请求身份（cron）也写行且 actor/IP 为空。
- [x] **AR-3 拆 `services.go`（2,671 行）** ✅ 2026-10-23 第 57 轮完成
  纯移动（不改函数体、172 个测试零改动仍全绿）。`services.go` 消失，拆成 15 个域文件：`service.go`（Service/New/ServiceError/StatusOf/MsgOf 核心）、`dto.go`（25 个 DTO + `toPublic`）、`auth.go`（注册/登录/JWT）、`posts.go`（读+创建）、`posts_list.go`（分页列表）、`posts_edit.go`（改/删）、`moderation.go`（审核/封禁库）、`tags.go`（标签关联事务）、`taxonomy.go`（分类/标签 CRUD）、`nav.go`（上一篇/下一篇）、`users.go`（ListUsers/SetUserDisabled/UserStats）、`backup.go`（导出/导入）、`restore.go`（快照恢复）、`trash.go`（回收站）、`media.go`（图片上传/管理/文件解析）。
  顺带拆掉另外两个超长文件：`accounts.go`（749→313 行）拆成 `site_config.go`（站点配置+登录记录）+ `accounts.go`（用户 CRUD）+ `accounts_profile.go`（资料/作者/仪表盘）；`mail.go`（420→257 行）拆成 `mail.go`（高层）+ `mail_smtp.go`（SMTP 底层协议）。
  结果：services 包最大文件 314 行（`backup.go`），全部 ≤350 行；**R2.files 4→1**（仅剩 `config.go` 1023 行，属 config 包，非本轮范围）。顺手修 `dto.go` 里一处错位注释（`UserPublic` 前误挂的 `CategoryWithCount` 注释）。
- [x] **AR-4 路由表声明化** ✅ 2026-10-23 第 58 轮完成
  76 条路由改为 `[]route{method, path, auth, roles, handler}` 集中声明（`internal/router/router.go` 的 `var routes`）；`roles` 用 `models.RoleAdmin/RoleRoot`（`adminRoles` = admin|root、`rootRoles` = root）。`/admin/banned` 等 9 个 root-only 端点（ban/list-banned/send-back/delete-banned/import-backup/restore-snapshot/list-snapshots/get-run-auto-backup）由表声明 `roles: rootRoles`。
  权限从 handler 上移到路由表：新增 `handlers.RequireRole(roles...)` 中间件（从 context 读 actor 角色，放行或 403），删掉 handlers 里 30 处 `requireAdmin` + 9 处 `requireRoot` 及两个函数定义——**R4 39→0**。`requireAdmin`/`requireRoot` 语义保留：root-only 报 `"root only"`、其余报 `"admin only"`。
  纵深防御不变：service 层 `IsRoot`/`IsAdminOrRoot`/`canReviewPost`/`canManageUser` 仍逐用例校验（`UpdateSiteConfig` 里「非 root 不能改 AutoBackup」的字段级授权保留在 handler，非路由守卫）。现有越权测试（`TestBanPostRequiresRoot`/`TestListBannedHiddenFromAdmin` 等）不改仍全绿；新增 `handlers/guard_test.go` 6 用例锁定 `RequireRole` 行为。
  顺带修 `contract_check.py`：解析路由表格式 + 两处既有解析误报（`importMarkdown` 裸 fetch 默认 GET、`archive/paged` 嵌套反引号），「方法不匹配」恢复 0。（前端路由同层表达仍在 AR-7。）
- [x] **AR-5 迁移注册表** ✅ 2026-10-23 第 59 轮完成
  9 段克隆迁移函数收敛为「1 张注册表 + 1 个泛型助手」：新建 `internal/config/migrate.go`，`var migrations = []migration{{version, up}}` 有序注册表（10 条），`runManualMigrations` 循环 + `apply` 集中门禁（原每个函数各抄一遍的「查 gate → 判表 → 判列 → ALTER → record」删掉），8 个 `addXxxColumn` 收敛成泛型 `ensureColumns(model, cols, after...)`。`dropScheduledAt`/`promoteFirstAdminToRoot` 保留为具名 `up` 函数（去掉各自 gate+record）。漂移检查 `checkSchemaDrift` 本就遍历 `models.All()`（P0 已改），无手写清单。
  `config.go` 1023 → 557 行（迁移代码拆出），`migrate.go` 250 行 ≤350。新增 `arch_check.py` **R7 门禁**（`add*Column` 克隆函数计数，目标 0，已归零锁基线）。
  **顺带修一个真 bug**：双库冒烟（新库 + 拷贝 `app.db.bak-20261014` 旧库各跑一次 `InitDB`）发现 `users.token_version` **自 v0.1.0 起就在模型里、却从未写过迁移**，旧库登录/登出会 `no such column` 500（漂移检查 WARN 早已提示）。加 `0011_add_token_version` 一条注册表项即修复——正是本阶段验收「新增列只需 2 行」的实证。冒烟后新库=旧库=89 列、零缺列、零真类型差异（余下 `VARCHAR≡TEXT`、`bool→numeric≡INTEGER` 均为 SQLite 亲和性别名，良性）。
  测试 173→176：`mig_test.go` 重写 `TestDropScheduledAt`（纯 up）+ 新增 `TestApplyMigrationGate`（集中门禁）/ `TestRunManualMigrationsFresh`（注册表全量）/ `TestEnsureColumns`（泛型助手）。
- [x] **AR-6 前端数据层** ✅ 2026-10-23 第 60 轮完成（数据层核心 + 旗舰页）
  新增 `core/http.ts`（唯一网络出口，单 `fetch` 调用）：`ApiError` + `request`/`upload`/`download` 三基元 + `errMessage`/`errStatus` 两个错误提取助手（全库仅此 2 处 `instanceof ApiError`）。`api.ts` 改成纯类型化 facade，删内部 3 处裸 fetch（`importMarkdown`/`downloadBackup`/`uploadImage` 改走 `upload`/`download`），re-export `ApiError`/`setTokenGetter`/`setOnUnauthorized`/`errMessage`/`errStatus` 保持 `auth.tsx` 与页面兼容。新增 `core/useQuery.ts`（`useQuery(fn, deps) → {data,loading,error,reload,setData}` + `useMutation`，零依赖、含竞态防护）。
  结果：**R8 3→0**（`fetch` 只剩 `http.ts` 一处）、`instanceof ApiError` 54→2、`tsc --noEmit` + `vite build` 全绿（产物 233.35→231.99 kB）。顺带修 `contract_check.py`（解析 `upload()`/`download()` 两个新基元，3 个端点恢复识别，「方法不匹配」维持 0）。
  **旗舰页示范**：`PersonalCenterPage` 623→**52 行**——三个区块拆到 `pages/personal/{MyHomepage,ContentManage,SecurityCenter}.tsx`（共享 `Pager`+`PAGE_SIZE`），列表三态机换成 `useQuery`（乐观更新用暴露的 `setData`）。**其余页面（AdminUsers 525 / AdminPosts 353 等）的 `useQuery` 铺开是下一增量**（`load` 回调+刷新、toast-on-error 形态与旗舰页略异，逐个冒烟）。
- [x] **AR-7 i18n 与守卫收敛** ✅ 2026-10-23 第 62 轮完成
  `i18n.tsx` 1,775 行 → **113 行**入口 + `core/locales/{nav,auth,center,reader,editor,admin,site,common}.ts` 8 个域文件（机械拆，`type Texts` 交叉组合，键值零改动）；`t()` 未命中在 dev `console.warn`、prod 回退英文（不再原样回显 key）。新建 `core/roles.ts`（`isAdmin`/`isRoot`/`isAdminOnly` 三个 helper，对应后端 `services/roles.go`）。守卫收敛到路由表：`AdminGuarded` 增加 `roles` 参数（默认 `['admin','root']`），`/admin/banned` 在路由层声明 `roles={['root']}`；删 7 个 admin 页面（Categories/Posts/Review/Tags/Trash/Users/Backup）+ BanVault 各自的重复 `user?.role !== 'admin' && ...` 守卫。**`role ===` 字面量 25→3**（仅 roles.ts 实现）+ 4 处注释；`window.confirm` 仅剩 `app.tsx` 的 `confirmDialog` 底层实现（页面 0 处）。验收：`tsc --noEmit` + `vite build`（产物 230.30kB）全绿，arch_check/contract_check/doc_check 全过。
- [x] **AR-8 前端最小测试** ✅ 2026-10-23 第 63 轮完成
  引入 `vitest@^2.1.9`（1 个 devDependency，与 vite 5 兼容，不进产物）。新增 3 个纯函数测试模块（显式 `import { describe,it,expect } from 'vitest'`，因 tsconfig `types:["vite/client"]` 不含 vitest globals）：`format.test.ts`（fmtDate/fmtDay，6 用例）、`md.test.ts`（escapeHTML/highlight/extractTOC，11 用例，只测无 UI 依赖纯函数，不碰 `DOMParser`）、`http.test.ts`（ApiError/errMessage/errStatus，5 用例）。共 **22 用例全绿**。
  `package.json` 加 `"test": "vitest run"`；新建 `vitest.config.ts`（`environment:'node'`，不加载 preact 插件/代理）。
  验收：`npm test` 22/22、`tsc --noEmit`、`vite build`（产物 230.30kB，`.test.ts` 不进产物）全绿，arch_check/contract_check/doc_check 全过。
  **坑 1**：`extractTOC(src: string | null | undefined)` 参数不带 `?`，是必填形参（类型允许 undefined 但不允许省略），测试须传 `undefined` 而非 `extractTOC()`。
  **坑 2**：本机 WorkBuddy 沙箱经 `NODE_OPTIONS` 注入 fs shim，拦截 vitest 写 `%TEMP%` 的 vite-node ssr 缓存（EPERM，随机漏跑一个测试文件）——验证时用 `NODE_OPTIONS= npm test` 清空注入；用户在真实终端跑 `npm test` 无此问题。
- [x] **useQuery 铺开收尾（AR-6 后续增量）** ✅ 2026-10-23 第 63 轮完成
  9 页手写三态机 → `useQuery`：6 管理页（BanVault/Categories/Dashboard/Review/Tags/Trash，同构批量）+ 3 读者页（Archive 分页+error / Author notFound / List 多参数 URL 驱动）。
  **ReadPage 保留手写**：其加载是「主请求 getPost + 并行 nav/related + fire-and-forget reportView 上报 + 失败 navigate('/')」，`useQuery` 单请求模型不匹配，强行拆分会更复杂（side effects 需额外 useEffect 监听 data）。
  验收：`tsc --noEmit` + `vite build`（229.81kB）+ vitest 22/22 + arch_check/contract_check/doc_check 全绿；grep 确认无遗留 `load()`/`setLoading` 三态机代码。
- [x] **B7 查询参数解析统一（3 套 → 1 套）** ✅ 2026-10-23 第 67 轮完成
  `handlers/query.go` 新增 `parseInts(q, label, keys...)`（批量解析可选查询整数，任一显式非数字 → 400）与 `parseID(r, name, label)`（路径参数，非法 → 400），替代 `ListPosts`/`ListArchivePaged` 手写 `strconv.Atoi` + `RestoreRevision`/`categories.go` 的手写 `ParseInt`。
  **顺带修 4 处静默吞错**：`categories.go` 的 `UpdateCategory`/`DeleteCategory`/`UpdateTag`/`DeleteTag` 原 `id, _ := strconv.ParseInt` 把非法 id 吞成 0 再查库 not found，现改 `parseID` 直接 400。
  `parseLimit`（宽松 clamp）保留，与严格语义区分。新增 `TestParseInts`/`TestParseID`。grep 确认 handlers 内 `strconv.Atoi`/`ParseInt` 只剩 `query.go` 3 处辅助函数。
  **门禁反证**：第一版用单参数 `parseIntStrict` 写了三段重复错误处理，把 `ListPosts` 撑到 64 行触发 R2.funcs 16→17 拦截，重构为 `parseInts` 批量（单次检查）后回落 16。
- [x] **B6 错误构造三套 — 核否为过时误报** ✅ 2026-10-23 第 67 轮核对（无需改）
  档案原称「裸 `fmt.Errorf` 会把内部措辞直落 500 暴露给前端」。当前代码已不成立：`service.go:MsgOf` 对非 `ServiceError` 错误统一返回 `"internal server error"`（不泄漏）；`imghost.go:79` "smms upload failed" 被 `media.go` 静默吞掉走本地兜底；`auth.go:213` "invalid token signature" 被 `Authenticate` 的 401 兜住。剩余裸 `fmt.Errorf` 全是正确的内部 wrap（`%w`），非缺陷。**不做。**

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
  🔶 **持续推进（2026-10-23 第 63 轮）**：`canManageUser` 已由 `ResetPassword` 调用（P0-1 根因消除）；`canReviewPost` 由 `ReviewPost` 改调——原手写 author 角色判断收敛为 helper，回归 `TestReviewPostScopeCentralised` 锁定「admin 不能审 admin 文章、root 能」。`UpdateUser`/`DeleteUser`/`CreateUser` 的用户管理链、`BanPost`/`SendBackPost`/`DeleteBanned` 的 root-only 守卫仍手写——但审计 §6 已确认层级校验完整、行为正确，且 root-only 守卫 P4 已上移路由表（service 层 IsRoot 纵深防御），整体重写风险大于收益，暂缓。
- [x] **P1-17 root 密码无恢复路径（第 38 轮文档核对发现，运维缺口）**
  `config.go` seed 写入的首启账号是 `Role: RoleRoot`（README 曾误写为 admin）。P0-1 修复后 `ResetPassword` 经 `canManageUser` 拦截「admin 重置 root」，root 自身改密需提供当前密码——**root 忘记密码后没有任何 API 路径可恢复**，只能直改 DB 的 `password_hash` 或删库重建（`ADMIN_INITIAL_PASSWORD` 只在无 admin 账号时生效）。
  ✅ **已修（2026-10-22 第 39 轮）**：新增离线恢复入口 `lopezpress -reset-root-password`（spec `docs/specs/22-root-password-recovery.md`）。`internal/config/recover.go` 的 `ResetRootPasswordForCLI` 查唯一 root → 生成 22 字符随机密码 → 单条 UPDATE 同时改 `password_hash` / 置 `must_change_password` / `token_version+1`（吊销既有会话）；`cmd/server/main.go` 用 `-reset-root-password` 标志在**启动 HTTP 之前**拦截，先 `os.Stat(DBPath)` 拒绝不存在的库（否则 InitDB 会静默 seed 出一个假站点）。刻意不做 HTTP 端点（= 远程接管通道，违背 R2）。回归 `internal/config/recover_test.go` 3 用例（轮换+吊销+强制改密+角色不变 / 只动 root / 无 root 报错且不凭空造 root）；端到端实测：旧密码 401、新密码 200 且 `must_change_password:true`、JWT `ver:1`。
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

- [x] **P2-1 死代码清理（9 处，零风险）**：`requireAdminOrRoot`（0 调用）、`var db *gorm.DB`（只写不读）、`var _ = models.Category{}`（占位 import）、`ImageUploader.tsx`（整个文件无人 import）、`Sidebar.tsx` 空 catch、`ThemeMode` 死类型（与 theme.tsx 冲突）、i18n 死键 15 个（`settings.*` 7 + `imgUpload.smms*` 8）、`UpdatePost` 注释重复两遍。
  ✅ **已修（2026-10-15 修复轮 2）**：删 `requireAdminOrRoot`、`var db`（Init 保留签名）、`var _ = models.Category{}` 及多余 import、`ImageUploader.tsx` 整文件、`types.ts` 的 `ThemeMode` 死类型、i18n `smms*`/`settings.*` 15 死键、`Sidebar` 空 catch 体改纯静默注释、`UpdatePost` 重复注释（修复轮 1 已清）。`go build`/`go vet`/`tsc`/`vite build` 全绿。
- [x] **P2-13 `CreatePostInput` 缺 `cover` 字段（跨栈契约漂移，第 33 轮）**：前端 `CreatePage` 创建文章时发 `cover`（PostEditor 封面功能），但后端 struct 无此字段——Go JSON decoder 静默丢弃未知字段，**创建时设的封面直接丢失**（仅编辑路径 `UpdatePostInput` 有 Cover）。这类漂移 `tsc`/`go build` 都发现不了，只能跨栈核对。
  ✅ **已修（2026-10-19 第 33 轮）**：`CreatePostInput` 加 `Cover string` + `CreatePost` 落库；回归 `post_cover_test.go` `TestCreatePostKeepsCover`（返回值 + DB 行双断言 + 空值不幻影）；新增 `scripts/contract_check.py`（前后端 method+path 自动核对工具，防复发）。`go build/vet/test` 全绿。
- [x] **P2-14 恢复快照前端无 UI（功能缺口，第 33 轮记录）**：后端 `POST /api/backup/restore-snapshot`（第 30 轮已改 root-only）存在，但前端零调用——root 想恢复快照只能 curl。AdminBackupPage 只有「导入 JSON」，没有「从快照恢复」入口（快照列表也没有展示）。建议：快照列表 + 恢复按钮（rootOnly）+ 确认对话框。
  ✅ **已修（2026-10-19 第 34 轮）**：后端新增 `GET /api/backup/snapshots`（root-only，`ListSnapshots` service + handler + router，回归 `backup_snapshots_test.go`：admin 403 / 缺目录返空 / 只列 snapshot-*.db / 大小正确）；前端 `api.ts` 加 `listSnapshots`/`restoreSnapshot`，AdminBackupPage 新增「快照恢复」区块（rootOnly：表格 + 每行恢复按钮 + 确认对话框 + 恢复后自动刷新列表）；i18n 11 个新键（zh/en，含 `admin.snapshotRestoreConfirm` 避免与回收站 `admin.restoreConfirm` 撞名——tsc 抓到的）。`go build/vet/test` + `tsc` + `vite build` 全绿；`contract_check.py` 确认 restore-snapshot 已接入（未调路由只剩 5 个合理项）。
- [x] **P2-15 `banPost` 死导出（第 35 轮记录）**：`api.ts` 的 `banPost(id, note?)` 全前端无调用（ban 入口不在文章列表/审核页 UI 上），其 `note` 参数后端 `BanPost` 也不接收（设计上 ban 不带 note，note 属于 send-back）。
  ✅ **已修（2026-10-19 第 35 轮）**：按审计建议删死导出（`tsc`+`vite build` 全绿）。**遗留产品决策**：封禁库功能当前**无 UI 入口可发起 ban**（只能在 `/admin/banned` 对已封禁文做 send-back/删除），即封禁库"只出不进"。若需要，应在某管理页（如审核页/文章管理）加 root-only 的"封禁"按钮并复用 `POST /api/admin/ban/{id}`（届时重新导出 `banPost`）。待产品决策。
- [x] **P2-16 前端 4 个零 import 依赖（第 38 轮依赖核对发现）**：`web/package.json` 声明了 `@preact/signals`、`@preact/signals-core`、`preact-router`、`preact-render-to-string`，全仓库 `grep` 零 import；实际运行依赖只有 `preact` / `wouter` / `marked`。既增安装体积又让"极简依赖"的说法失真（`AGENTS.md` 原写"2 个 npm 包"）。修法：`npm uninstall` 这 4 个 + 复跑 `tsc`/`vite build`。
  ✅ **已修（2026-10-22 第 39 轮）**：`npm uninstall` 移除 4 个（连 `@preact/signals` 带入的间接包共清掉 3 个 node_modules 目录），`package.json` 依赖收敛为 `marked` / `preact` / `wouter` 三个；`tsc --noEmit` + `vite build` 全绿，产物大小 63 模块 / JS 209.09 kB 与清理前**完全一致**（反证这些包确实没进构建）。
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
  ✅ **已修（第 43 轮）**：新增 `internal/clientip` 包 + `TRUSTED_PROXIES`（逗号分隔 IP/CIDR，默认仅 `127.0.0.1/32,::1/128`）。只有 `RemoteAddr` 命中可信代理才采信 `X-Forwarded-For`，否则一律用 `RemoteAddr`；非法配置启动即 `log.Fatalf`。`middleware` 与 `handlers` 两份重复的 `clientIP` 已并入该包。5 个表驱动测试覆盖「伪造头被忽略 / 可信代理被采纳 / 自定义 CIDR / 非法配置报错」。
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
  ⬆️ **第 77–78 轮升级为完整图库**（spec 36）：元数据表 + sha256 去重 + 三层配额 + 引用台账 + 软删撤下 + 后台 `/admin/media` 溯源 + 异步缩略图。此条保留作历史，**新能力看本文件「💾 媒体与存储」节**。
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
- 导入备份：事务内 Clear+Create，导入前自动 `VACUUM INTO` 快照可回滚；第 75 轮起再加结构闸 + 确认词 + 全站 token 失效 ✅
- 快照恢复：`filepath.Base` 防路径穿越 ✅
- 角色提权：`UpdateUser`/`CreateUser` 遇 `RoleRoot` 直接 403（root 不可经 API 创建）✅
- 封禁可见性：`canSeePost` 中 `banned` 仅 root 可达，作者本人与其他 admin 均被过滤 ✅
- 审核作用域：admin 队列只含 user 发文，admin/root 发文仅 root 可审 ✅
- 软删图不会复活给下一个人：`live_key` 唯一索引，撤下即轮换为 `d:<id>`，去重查询结构上查不到 ✅（第 77 轮实测：撤下后重传同一份字节，返回新文件而非复用）
- 软删图仍能服务已发布文章：`DeletedAt` 是普通 `*time.Time` 而非 `gorm.DeletedAt`，`GET /uploads/{name}` 不看该字段 ✅
- 开着邮箱验证时 `PUT /api/auth/me` 改邮箱：403，无法绕过验证码 ✅（有单测）

---

## 🗄️ 已归档（不再作为待办）

| 文档 | 归档时间 | 原因 |
|---|---|---|
| `docs/archive/2026-10-14-FUNCTIONAL_AUDIT.md` | 2026-10-15 | 首轮审计快照，结论已全部落入本文件并勾选；留档仅为追溯"当时发现了什么" |
