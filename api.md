> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# API 参考

Base URL：`http://localhost:8080`（可用 `PORT` 环境变量覆盖）

需要鉴权的接口带请求头：`Authorization: Bearer <token>`。

> **事实源**：`internal/router/router.go`。本文件是给人和 AI 看的索引 + 关键端点细节；
> 两者不一致时以 router.go 为准，并请回来修正本文件。
> 当前：**85 条路由**（`scripts/contract_check.py` 会核对前端调用是否都存在于后端）。

鉴权标记：

| 标记 | 含义 |
|---|---|
| 公开 | 无需 token；但站点关闭「公开阅读」时会被 `WithOptionalAuth` 统一拦成 401 |
| 可选 | 匿名可读，带 token 则能看到更多（私密/待审/自己的文章） |
| 登录 | `withAuth`，必须有效 JWT |
| admin | admin 或 root |
| root | 仅超级管理员 |

---

## 1. 端点总览

### 1.1 系统 / 发现

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/` | 公开 | 不存在「服务信息」端点：未匹配的路径（含 `/`）统一返回 **404** `{"error":"not found"}`。以前这里回一张 JSON 端点表，等于给扫描器免费一份路由清单 |
| GET | `/health` | 公开 | 存活探针，**只返回 `{"status":"ok"}`**，不碰数据库。版本与 commit 默认不暴露（匿名接口在公网上等于自报构建号）；私有环境需要时设 `HEALTH_SHOW_VERSION=true` 才附带。查版本用 `lopezpress -version` 或启动日志 |
| GET | `/feed` | 公开 | RSS 2.0（链接优先 `/p/{slug}`） |
| GET | `/sitemap.xml` | 公开 | 站点地图（slug 优先，无 slug 回退 `/read/{id}`） |
| GET | `/robots.txt` | 公开 | 跟随 `public_read`：锁站时输出 `Disallow: /` |
| GET | `/uploads/` | 公开 | 本地上传的图片。真实存在的文件带 `Cache-Control: public, max-age=2592000`（30 天，**不是** `immutable`：撤下的违规图必须能在合理期限内对已看过的浏览器失效）。不存在或已被硬删的文件返回 404 且**不带**缓存头，否则从备份恢复的同一文件会被浏览器长期记住 |

### 1.2 认证 / 账号

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/register` | 公开 | 注册（受 `allow_register` 开关限制）。开启邮箱验证时须带 `code`。密码策略见 1.2.1 |
| POST | `/api/auth/login` | 公开 | 登录，返回 `{token, user}`。`email` 与 `username` 等价，任一填了即可（字段名仍叫 `email`）。**必须带验证码**：`captcha_id` + `captcha_code`（4 个字符，大小写不敏感）。缺或错 → **400** `{error, captcha_required: true, captcha: {...}}`，响应体已带一张新验证码，前端无需再请求。**状态码不是 401**，因为登录限流把 401/403 计入账号锁定，验证码错误不能变成锁死管理员的更便宜路径 |
| GET | `/api/auth/captcha` | 公开 | 取登录验证码，返回 `{enabled, captcha}`。`captcha` 只有一张 4 位字符图（`image` 是 PNG data URI，200×50 画布 + 左右各 4px 留白 = 208×50），没有第二种交互。**验证码状态全部在服务端**：120s 过期、一次性（成功即销毁，失败也销毁）、最多错 3 次、绑定发起请求的客户端 IP（答案换台机器用一律拒绝）、每客户端每分钟最多取 30 张（近期有登录失败的收紧到 10 张，超了 **429** + `Retry-After: 60`）。响应带 `Cache-Control: no-store`。`enabled:false` 表示服务端已关闭该功能 |
| GET | `/api/auth/register/open` | 公开 | 注册是否开放 |
| POST | `/api/auth/email-code` | 公开 | 索取一次性验证码 `{email, purpose}`，purpose 只接受 `register` / `reset_password`。**同邮箱 60s 冷却（429）+ 同 IP 每小时 10 封**；SMTP 未配置返回 503；无论邮箱是否已注册回答一致 |
| POST | `/api/auth/password/reset` | 公开 | 自助重置密码 `{email, code, password}`。成功 bump `token_version`（所有会话失效），旧密码立即作废 |
| GET | `/api/auth/me` | 登录 | 当前用户 |
| PUT | `/api/auth/me` | 登录 | 改资料 |
| POST | `/api/auth/logout` | 登录 | 登出 |
| POST | `/api/auth/change-password` | 登录 | 改密码（`current_password` + `new_password`）。**返回 `{message, token, expires}`**：bump `token_version` 后旧 token 全部失效，新 token 供当前会话无缝续用 |
| POST | `/api/auth/logout-all` | 登录 | 全设备登出（bump token version） |
| POST | `/api/me/email-change/request` | 登录 | 换绑邮箱第一步：验证码发到**新**地址 `{email}` |
| POST | `/api/me/email-change/confirm` | 登录 | 换绑邮箱第二步 `{email, code}`，成功返回更新后的用户；旧地址会收到一封变更通知 |
| GET | `/api/me/stats` | 登录 | 我的统计 |
| GET | `/api/me/login-history` | 登录 | 我的登录记录 |

### 1.2.1 密码策略（所有写密码的端点共用）

五个入口——注册、`POST /api/users`、`PUT /api/users/{id}/reset-password`、
`POST /api/auth/change-password`、`POST /api/auth/password/reset`——走同一个闸
`services.ValidatePassword`，任何一条不满足都返回 **400** `{error}`，`error` 里写明是哪一条：

| 规则 | 典型 `error` |
|---|---|
| 去掉首尾空白后不能为空 | `password is required` |
| ≥ 8 个**字符**（不是字节，中文按字算） | `password must be at least 8 characters` |
| ≤ 72 **字节**（bcrypt 硬上限；约 24 个汉字） | `password is too long (max 72 bytes; …)` |
| 不能是最常见密码（含 `p@ssw0rd` 这类变形） | `password is too weak: it is one of the most commonly used passwords` |
| 不能是单一字符重复 / 纯数字 ≤ 12 位 / 简单序列 | `password is too weak: a single repeated character` 等 |
| 不能包含自己的用户名或邮箱前缀 | `password must not contain your username` |

**不做**"必须含大写 + 数字 + 符号"的复杂度要求。这是 NIST SP 800-63B 的取舍：
复杂度规则把人推向 `Password1!`，而它恰好就在每一份字典的第一页；长度加黑名单才
真正抬高离线爆破的成本。除 `POST /api/auth/password/reset` 外，其余四处还会校验
"不能包含自己的用户名/邮箱前缀"（该端点在校验时尚未解析出账号，通用规则先行，
避免弱密码白白烧掉一个验证码）。

### 1.3 文章 — 读

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/posts` | 可选 | 列表。`?page=&size=&q=&category=&tag=&status=&sort=&order=&author=`；**页面不传 `size`，由站点设置决定**。每项附 `cover_thumb`（封面是本站图片且缩略图已生成时才出现，否则省略，客户端回退 `cover`——列表一屏十张封面，用原图是站点最大的图片浪费）。`category` 收**分类名**（`?category=PHP`）或数字 id（旧链接）；名字也可读，且父分类连带其子分类的文章。`tag` 大小写不敏感。未知分类返回**空页**，不会退化成"不筛选" |
| GET | `/api/posts/recent` | 可选 | 最近 5 篇已发布文章；**响应不含 `reviewed_by` / `reviewed_at`**（匿名可读，不下发审核员身份） |
| GET | `/api/posts/archive` | 可选 | 归档（旧版，全量；前端已改用 paged） |
| GET | `/api/posts/archive/paged` | 可选 | 归档分页（默认 50 / 上限 200 / `body` 恒空） |
| GET | `/api/posts/{id}` | 可选 | 单篇（**不计数**，计数走 `/view`） |
| GET | `/api/posts/{id}/public` | 公开 | 仅 published |
| GET | `/api/posts/{id}/nav` | 可选 | 上一篇 / 下一篇。锚点文章先过 `canSeePost`，读不到返回 404（与详情一致，不泄露存在性） |
| GET | `/api/posts/{id}/related` | 可选 | 相关文章（同分类或同标签，≤5）。锚点文章先过 `canSeePost`，读不到返回 404 |
| GET | `/api/permalinks/{slug}` | 可选 | 永久链接解析（`/p/slug` 用） |
| GET | `/api/categories` | 公开 | 分类（两层树，含 `parent_id`） |
| GET | `/api/tags` | 公开 | 标签 |
| GET | `/api/authors/{username}` | 公开 | 作者主页数据 |

### 1.4 文章 — 写

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/posts` | 登录 | 新建（自动生成 slug） |
| PUT | `/api/posts/{id}` | 登录 | 更新；改标题不动 slug，显式传 `slug` 才改；**更新前自动存修订**。字段「不传」≠「传空」：`cover` 不传=不动、传 `""`=删封面；`category_id` 不传=不动、传 `0`=改为无分类；`tags` 不传=不动、传 `[]`=清空标签 |
| DELETE | `/api/posts/{id}` | 登录 | 软删（进回收站，7 天） |
| POST | `/api/posts/{id}/view` | 可选 | **唯一的浏览量入口**（指纹去重 + 作者/爬虫不计） |
| GET | `/api/posts/{id}/revisions` | 登录 | 修订历史（≤20 条/篇） |
| POST | `/api/posts/{id}/revisions/{revID}/restore` | 登录 | 回滚到某版本（只回标题/正文） |

### 1.5 分类 / 标签 / 图片

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/categories/with-count` | 登录 | 带文章数 |
| POST | `/api/categories` | admin | 新建（可带 `parent_id`） |
| PUT | `/api/categories/{id}` | admin | 改名 / 改父级 |
| DELETE | `/api/categories/{id}` | admin | 删除（子级上移） |
| POST | `/api/admin/categories/preset` | admin | 应用预设分类树（幂等） |
| GET | `/api/tags/with-count` | 登录 | 带文章数 |
| PUT | `/api/tags/{id}` | admin | 改名（保留文章关联） |
| DELETE | `/api/tags/{id}` | admin | 删除（解除关联） |
| POST | `/api/upload` | 登录 | 上传图片 → `{id, name, url, size, width, height, mime, kind, deduped, remote}`。`kind` 为 `post`（默认，≤5 MB）/ `avatar`（≤512 KB）。扩展名白名单 **+ magic bytes 内容校验**，HTML 伪装成 `.png` 返回 400。**相同字节只存一份**，第二次上传返回 `deduped:true` 并指向已有行（不计配额、不写盘）。**每账号限流** 20 张/分、200 张/时（突发 5），超限返回 **429** + `Retry-After`，拒绝发生在解析 body 之前 |
| GET | `/api/me/uploads` | 登录 | **本人**的上传（分页 24，`?kind=&status=&page=&size=`），附 `stats` 与配额用量。普通用户唯一的图库入口 |
| DELETE | `/api/me/uploads/{name}` | 登录 | 撤下本人的图（**软删**：文件仍服务已发布文章，但脱离图库、不可再被去重复用）。站长关掉「用户可删自己的图」时返回 403 |
| GET | `/api/uploads` | **admin** | 全站图库（`?owner=&kind=&status=live\|deleted\|all&ref_status=used\|unused&page=&size=`）。每行带 `thumb_url`（后台队列生成的 480px 缩略图，未生成时为空串，客户端回落 `url`）与 `thumb_status` / `thumb_attempts`（`pending\|ready\|skip\|failed`；失败自动重试一次，`failed` 时可走 `thumb-retry`） |
| GET | `/api/uploads/{name}/refs` | **admin** | 单图引用详情（B3）：返回 `[{kind, post_id, post_title, user_id, user_name}]`，`kind` ∈ `body\|avatar\|site`。与删除警告读同一本台账 |
| POST | `/api/admin/uploads/batch-delete` | **admin** | 批量删除（B3）：`{"names": [...], "hard": false}`，上限 100。逐条走单图删除逻辑，**部分成功**：`{"affected": N, "skipped": [{"name","reason"}]}`；`hard=true` 仍要求 admin/root |
| DELETE | `/api/uploads/{name}` | **admin** | 软删（默认）或**硬删**（`?hard=1`，连文件与缩略图一起删，即使文章仍在引用；响应 `affected_refs` 告知破坏了几个引用）。`affected_refs` 来自 `upload_refs` 台账，涵盖正文、封面、头像、站点 logo/favicon 四种来源；**它只是告知，不阻止删除**——撤下一张违规图不应需要引用者的同意 |
| POST | `/api/uploads/{name}/restore` | **admin** | 恢复已撤下的图。若期间有人上传了相同字节，返回 409（唯一索引不允许两条活跃记录） |
| POST | `/api/uploads/{name}/thumb-retry` | **admin** | 重新排队生成缩略图（M-8）。后台队列对失败图自动重试一次；两次仍失败才进入终态 `failed`，此接口把 `thumb_status` 重置为 `pending` 并清零 `thumb_attempts`。已 `ready` 的图与已撤下的图返回 400 |
| POST | `/api/admin/uploads/rebuild-refs` | **root** | 从正文、封面、头像、站点 logo/favicon 重算全部 `upload_refs` 与 `ref_count`（M-9）。仅 `root`；正常经 API 的保存/删除已自动维护台账，此接口用于“有人直接改库”后的对账，可重复执行。响应 `{"refs": N}` |

### 1.6 后台 — 内容

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/admin/dashboard` | admin | 统计（含 30 天趋势、待审数、Top5） |
| POST | `/api/admin/posts/batch` | admin | 批量（6 个 action，逐条复用单条逻辑） |
| POST | `/api/admin/import/markdown` | admin | Markdown 批量导入（multipart，≤200 文件） |
| GET | `/api/admin/pending` | admin | 待审队列 |
| POST | `/api/admin/review/{id}` | admin | 审核（approve / reject + note） |
| POST | `/api/admin/ban/{id}` | root | 封禁 |
| GET | `/api/admin/banned` | root | 封禁库 |
| POST | `/api/admin/banned/{id}/send-back` | root | 退回作者 |
| DELETE | `/api/admin/banned/{id}` | root | 永久删除 |
| GET | `/api/trash` | admin | 回收站。**按调用者可见范围过滤**：admin 只看到自己与普通用户的，root 看全部 |
| POST | `/api/trash/{id}/restore` | admin | 恢复。目标属于 root/其他 admin 时 **403**（与 `DELETE /api/posts/{id}` 同一套规则） |
| DELETE | `/api/trash/{id}` | admin | **单条永久删除**。权限同 restore；不受 7 天恢复窗口限制（窗口外的行已不可恢复，更不该不可删除）；删除后同事务清理孤儿图片台账并重算 `ref_count`；不是回收站里的行返回 400，已不存在返回 404 |
| POST | `/api/trash/empty` | admin | 清空。**只清调用者有权管理的那些**，返回实际删除数 |
| GET | `/api/admin/actions` | admin | 审计日志。可选 `q`（操作人/操作/对象/详情关键词，大小写不敏感）与 `actor`（用户 ID） |
| GET | `/api/admin/actions/export` | admin | 审计日志 CSV 导出（UTC+8 服务器本地时间，UTF-8 BOM，公式注入已转义；与列表同口径，最多 20000 行） |

### 1.7 后台 — 用户 / 站点 / 备份

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/users` | admin | 用户列表。非 root 调用时 root 那行的 `email`/`last_login_ip` 为空 |
| POST | `/api/users` | admin | 新建用户（不可造 root） |
| PUT | `/api/users/{id}` | admin | **只改 `role`**。`disabled` 字段已移除（前端从未使用），启用/禁用只有 `PUT /api/users/{id}/disabled` 一个入口——否则 admin 可以用它撤销 root 对另一名 admin 的停用 |
| PUT | `/api/users/{id}/disabled` | admin | 启用/禁用。admin 对 admin 双向拒绝（含"启用"），root 不可被禁用 |
| POST | `/api/users/{id}/reset-password` | admin | 重置密码（`canManageUser` 守卫）。重置后 bump 目标 `token_version`，其所有旧会话立即失效 |
| DELETE | `/api/users/{id}` | admin | 删除 |
| GET | `/api/users/{id}/logins` | admin | 该用户登录记录。目标为 root 时仅 root 可读（403） |
| GET | `/api/site-config` | 公开 | 公开配置（品牌、`footer_text`、`theme_color`、`home_layout`、`page_size`、`public_read`） |
| GET | `/api/admin/site-config` | admin | 全量配置。非 root 的 `smtp_pass`/`img_host_token` 一律返回空串 |
| PUT | `/api/admin/site-config` | admin | 保存内容字段。非 root 提交时继承 `allow_register`/`smtp_*`/`img_host_token`/`auto_backup`/`backup_keep`（字段级授权）；`friend_links[].url` 必须是 http(s)，否则 400；`site_url` 必须是不带路径的**裸 origin**（`scheme://host[:port]`），带 path/query/fragment 一律 400——它是 `/feed` 与 `/sitemap.xml` 里绝对链接的前缀，带 `/admin` 这类路径会让每篇文章链接都变成 404；`theme_color` 只接受 `#rrggbb`（空串 = 恢复内置配色），`home_layout` 只接受 `list`/`cards` |
| POST | `/api/admin/mail/test` | admin | 测试邮件 |
| GET | `/api/admin/menus` | admin | 后台侧边栏全部行（含隐藏项），按 `sort,id` 排序。每行含 `key`（内置项稳定标识）/ `label_key`（i18n 键）/ `label`（覆盖文案）/ `group_key` / `root_only` / `visible` |
| POST | `/api/admin/menus` | root | 新增自定义链接：`{"label","path","group_key"}`。`path` 必须以单个 `/` 开头（`//`、`http(s)://`、含空白一律 400）；`group_key` ∈ `content\|users\|system\|""` |
| POST | `/api/admin/menus/reorder` | root | `{"ids":[...]}`，必须列出**全部**菜单项且不重复，否则 400；按数组顺序重写 `sort` |
| PUT | `/api/admin/menus/{id}` | root | 内置项只接受 `label`（空串 = 回落 i18n）与 `visible`，`path`/`group_key` 被忽略；自定义项可改 `label`/`path`/`group_key`/`visible` |
| DELETE | `/api/admin/menus/{id}` | root | 仅自定义链接可删；内置项返回 400（只能隐藏，避免丢失页面入口与角色门） |
| POST | `/api/backup` | **root** | 导出 JSON 备份（含 bcrypt 哈希，故仅 root） |
| POST | `/api/backup/import` | **root** | 导入：先 `?dry_run=1` 预检，应用需载荷带 `confirm:"REPLACE-ALL-DATA"` |
| GET | `/api/backup/download` | **root** | 下载备份文件（JSON 与 `auto-*.db` 快照均含密码哈希） |
| GET | `/api/backup/snapshots` | root | 快照列表（`snapshot-*` 与 `auto-*`） |
| POST | `/api/backup/restore-snapshot` | root | 从快照恢复（恢复后全站 `token_version` +1） |
| GET | `/api/admin/backup/auto` | root | 每日备份状态 |
| POST | `/api/admin/backup/auto` | root | 立即生成今日快照（幂等） |

---

## 2. 关键端点细节

### `POST /api/backup/import`（root）

整体替换数据库，因此有三道闸门，缺一即 400 且不写库：

1. **结构闸**：`users` 不能为空；每行必须有 `id`/`username`/`email`；必须至少有一个 `role:"root"`。
   （空对象 `{}` 曾在 2026-10-24 一次请求清空全站，此闸即为它而设。）
2. **预检闸**：`?dry_run=1` 只返回 `stats` 与实际将写入的行数，不落盘、不建快照。
3. **确认闸**：非 dry-run 必须携带 `confirm:"REPLACE-ALL-DATA"`，否则 400。

应用成功后：先 `VACUUM INTO` 生成 `snapshot-*.db`（保留 3 份），再在单事务里清表重建，
并为**所有**账号 `token_version + 1` —— 恢复备份等于撤销全站会话（否则恢复会复活旧 token）。


### `POST /api/posts/{id}/view`

```json
{ "counted": true, "view_count": 13 }
```

| 情况 | 结果 |
|---|---|
| 当天该访客首次读该文 | `counted: true`，`view_count` 为最新值 |
| 同一访客同一天再读 | `counted: false`（`duplicate`） |
| 作者本人 | `counted: false` |
| 爬虫 UA | `counted: false` |

指纹 = `sha256(IP + UA + JWT_SECRET)`，`UNIQUE(post_id, visitor_hash, day)`。
**`GET /api/posts/{id}` 永远不加数**——这是第 43 轮定下的不变量。

### `GET /api/posts/{id}/related`

返回 `Post[]`（最多 5 条），只含 published，排除自身：

```json
[{ "id": "…", "title": "Go 后端最佳实践", "slug": "go-后端最佳实践", "view_count": 8 }]
```

### `GET /api/permalinks/{slug}`

解析永久链接。未发布/无权限 → **404**（与"不存在"同码，不泄露存在性）。

### `POST /api/admin/posts/batch`

```json
{ "ids": ["uuid1", "uuid2"], "action": "publish", "category_id": 3, "tags": ["go"] }
```

`action` ∈ `publish | private | set_category | add_tags | remove_tags | delete`（上限 100 条）。
逐条执行，**部分成功**：

```json
{ "affected": 7, "skipped": [{ "id": "uuid3", "reason": "forbidden" }] }
```

`publish` 的门槛与审核队列对齐（`services.canPublish`）：root 可发布任意文章；admin 只能发布普通用户的文章，admin/root 自己的文章仍为 root-only（越权条目记为 `skipped` 而非整批失败）。

### `POST /api/admin/import/markdown`

`multipart/form-data`，字段 `files`（可多个）。限额：单文件 2 MB / 总计 24 MB / 最多 200 个。
frontmatter 支持 `title / date / category / tags / draft`；无 frontmatter 用首个 `# 标题`，再退到文件名。

```json
{ "created": 12, "skipped": [{ "name": "bad.md", "reason": "empty file" }] }
```

### `GET /api/admin/dashboard`

```json
{
  "posts_total": 13,
  "views_total": 48,
  "pending_total": 0,
  "view_trend": [{ "day": "2026-10-22", "views": 2 }],
  "top_posts": [{ "id": "…", "title": "…", "slug": "…", "view_count": 12 }],
  "recent_posts": [],
  "recent_logins": [],
  "views_today": 2,
  "views_7d": 9,
  "status_counts": { "published": 9, "pending": 0, "draft": 1, "private": 3, "banned": 0 },
  "users_total": 3,
  "users_disabled": 0,
  "users_new_7d": 1,
  "media_live_count": 24,
  "media_live_bytes": 3456789,
  "app_version": "1.0.19+0000000",
  "auto_backup": { "enabled": true, "keep": 3, "last_at": "2026-11-07T03:00:00Z", "files": 3 },
  "recent_actions": []
}
```

`view_trend` **恒为 30 项**（无阅读的日子是 0，服务端补齐）。`recent_posts` 的 `body` 恒为空。

- `views_today` / `views_7d` 与趋势同源（`post_views` 的去重日记录），不是 `view_count` 的求和。
- `status_counts` 统计全部状态的可见文章；**`banned` 仅 root**（普通 admin 的响应里没有这个键，封禁库不属于他们的可见范围）。
- `media_live_*` 只统计 `deleted_at IS NULL` 的图库行；软删图的字节仍占磁盘但不计入「可用媒体占用」。
- `app_version` 由进程的构建身份写入（可能与磁盘上的 `VERSION` 文件不同，以实际运行的二进制为准）。
- `auto_backup` 仅 root 返回；普通 admin 的响应里没有这个键。
- `recent_actions` 与 `/api/admin/actions` 可见口径一致，取最近 5 条。

### `GET|POST /api/admin/backup/auto`

```json
{ "enabled": true, "keep": 3, "last_at": "2026-10-22T18:03:46Z", "files": 1 }
```

`POST` 返回 `{ "file": "auto-20261022.db" }`；当天已有快照时 `file` 为空字符串（幂等，不是失败）。

---

## 3. 错误格式

```json
{ "error": "post not found" }
```

| 状态码 | 含义 |
|---|---|
| 400 | 参数错误（英文短句） |
| 401 | 未登录 / token 失效 / 站点不允许匿名读 |
| 403 | 角色不足 / 必须先改密码 |
| 404 | 不存在，或"存在但你看不到"（刻意同码） |
| 409 | 冲突（重名标签/分类等） |
| 413 | 上传超限 |
| 429 | 限流 |
| 500 | 服务端错误 |

错误文案一律英文（红线 7），界面翻译由前端 i18n 负责。

---

## 3.1 响应裁剪约定（第 72 轮加）

文章类响应里的 `author` 对象是**裁剪过的** `User`：只保留 `id` / `username` / `display_name` / `avatar` / `bio` / `role`，`email` / `last_login_ip` / `last_login_at` / `disabled` / `must_change_password` / `token_version` 一律置空（`models.Post.SanitizeAuthor`）。

- 覆盖所有出站文章载荷：`/api/posts`（列表）、`/api/posts/{id}`、`/public`、`/nav`、`/archive*`、`/feed`、`/permalinks/{slug}`、写接口回显。
- **例外**：`GET /api/admin/posts/pending`（审核队列）**保留** `email`，审核员需要据此识别作者；该端点本身 admin 限权。
- 不裁剪 `username` / `role`：作者主页链接与角色徽章要用。

---

## 3.2 可见性矩阵（私密对运营角色开放）

判定唯一入口 `services.canSeePost`；列表过滤在 `ListPostsPage` 的 `applyFilters` 里用同一套规则表达（root 不加状态过滤、admin 排除 `banned`、登录用户 = `published` 或自己的），两者必须同步修改，否则会出现「列表有、点进去 404」。

| 状态 | 匿名 | 作者本人 | 其他普通用户 | admin | root |
|---|---|---|---|---|---|
| `published` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `private` | ❌ | ✅ | ❌ | ✅ | ✅ |
| `draft`（第 22 轮已废的遗留值） | ❌ | ✅ | ❌ | ✅ | ✅ |
| `pending` | ❌ | ✅ | ❌ | ✅ | ✅ |
| `rejected` | ❌ | ✅ | ❌ | ✅ | ✅ |
| `banned` | ❌ | ❌ | ❌ | ❌ | ✅ |

- 看不到时**一律 404**，不区分"不存在"与"存在但无权看"，避免泄露文章存在性。
- `private` 对 admin/root 可见：它不进公开区，也不进审核队列（没有读者就没有审核的必要），但运营角色必须能打开正文，否则无法处理举报、核对下架是否正确、回答作者提问。编辑器里的说明据此写成「不出现在公开列表，游客和其他读者访问会显示 404；网站管理员可以看到」。
- **可见 ≠ 可写**：admin 能读别人的 `private`，但仍不能改它的标题/正文/封面/slug（`UpdatePost` 的 R5-05 拦截），也不能把 `pending`/`rejected` 拉成 `private`（那等于替作者做决定；`resolveStatusChange` 只允许 `published → private` 这一个下架方向）。
- 修订史跟随同一判定（`assertCanReadHistory` 调 `canSeePost`）：能看正文的人就能看历史，看不到的人拿 404。
- **改内容就要重新审核**：非 root 作者保存 `published` 文章时，只要 `title`/`body`/`cover` 有实质变化，状态转 `pending`，文章立刻从公开区下线，直到再次通过审核。审核结论只覆盖被审过的那一版文字——否则「已审核」等于「一次通过、此后随意改写」，审核形同虚设。该规则按**内容**判定而不是按请求里的 `status`，所以不带 `status` 的 `PUT` 也绕不过去；只改 `slug`/分类/标签不触发。root 是审核者，不受此限。
- 后果：后台「文章管理」对 admin 会显示别人的 private/draft 行，与详情接口保持一致。

---

## 4. 变更日志

只记录**接口契约**的变化；安全加固、内部实现与测试细节不在此列。

| 版本 / 轮次 | 契约变化 |
|---|---|
| 1.0.29 – 1.0.26 | 纯前端响应式改造，接口无变化 |
| 1.0.25 | 新增 `GET /api/admin/actions/export`（CSV，admin，最多 20000 行）；`GET /api/admin/actions` 新增可选 `q` |
| 1.0.19 | `GET /api/admin/dashboard` 只增字段：`views_today`、`views_7d`、`status_counts`、`users_total`、`users_disabled`、`users_new_7d`、`media_live_count`、`media_live_bytes`、`app_version`、`auto_backup`、`recent_actions` |
| 1.0.18 | 无新增端点；权限校验集中到统一 helper，状态码与错误语义不变 |
| 1.0.17 | 新增 `DELETE /api/trash/{id}`（权限同恢复，不受 7 天窗口限制） |
| 1.0.16 | `POST /api/auth/login` 要求 `Content-Type: application/json`（否则 415），带 `Origin` 时校验同源（不符 403） |
| 1.0.15 | 无变化 |
| 114 | Markdown 导入不再接受 `draft` 字段；历史 `draft` 归一为 `private` |
| 113 | 无变化（响应字段脱敏与引用台账一致性修复） |
| 112 | `PUT /api/posts/{id}`：`cover` 传 `""` 删封面、`category_id` 传 `0` 清分类；未传仍表示“不动” |
| 111 | `PUT /api/users/{id}` 移除 `disabled` 字段，启用/禁用只用 `PUT /api/users/{id}/disabled` |
| 109 | 非 root 作者修改已发布文章的 title/body/cover 会重新进入 `pending`；只改分类/标签/slug 不触发 |
| 95 | 密码策略：≥ 8 字符、≤ 72 字节、常见密码黑名单、不得包含用户名或邮箱前缀 |
| 79 | 新增 `POST /api/auth/email-code`、`POST /api/auth/password/reset`、`POST /api/me/email-change/{request,confirm}`；邮箱验证开启时 `PUT /api/auth/me` 改邮箱返回 403 |
| 78 | `GET /api/uploads` 每行新增 `thumb_url` |
| 77 | 新增 `GET/DELETE /api/me/uploads[/{name}]`、`POST /api/uploads/{name}/restore`；`DELETE /api/uploads/{name}` 支持 `?hard=1`；`POST /api/upload` 新增 `kind` 与 `deduped` |
| 73 | `site-config` 新增存储/头像策略字段；非 root 读不到 `smtp_pass` 与 `img_host_token` |
| 72 | `POST /api/auth/change-password` 返回 `{token, expires}`；`login` 支持 `username` 作为标识 |
| 52 | 新增 `GET /api/posts/{id}/related`、`GET / POST /api/admin/backup/auto` |
| 51 | 新增 `GET /api/posts/{id}/revisions`、`POST /api/posts/{id}/revisions/{revID}/restore`、`POST /api/admin/import/markdown` |
| 48 | 新增 `GET /api/permalinks/{slug}` |
| 47 | 新增 `GET /api/posts/archive/paged`、`GET /robots.txt` |
| 46 | 新增 `site_configs.public_read`、分类 `parent_id`、`POST /api/admin/categories/preset` |
| 43 | 新增 `POST /api/posts/{id}/view`、`PUT /api/tags/{id}`；`GET /api/posts/{id}` 不再自增浏览量 |
