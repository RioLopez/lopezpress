> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# API 参考

Base URL：`http://localhost:8080`（可用 `PORT` 环境变量覆盖）

需要鉴权的接口带请求头：`Authorization: Bearer <token>`。

> **事实源**：`internal/router/router.go`。本文件是给人和 AI 看的索引 + 关键端点细节；
> 两者不一致时以 router.go 为准，并请回来修正本文件。
> 当前：**76 条路由**（`scripts/contract_check.py` 会核对前端调用是否都存在于后端）。

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
| GET | `/` | 公开 | 服务信息（JSON） |
| GET | `/health` | 公开 | 健康检查 |
| GET | `/feed` | 公开 | RSS 2.0（链接优先 `/p/{slug}`） |
| GET | `/sitemap.xml` | 公开 | 站点地图（slug 优先，无 slug 回退 `/read/{id}`） |
| GET | `/robots.txt` | 公开 | 跟随 `public_read`：锁站时输出 `Disallow: /` |
| GET | `/uploads/` | 公开 | 本地上传的图片 |

### 1.2 认证 / 账号

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/register` | 公开 | 注册（受 `allow_register` 开关限制） |
| POST | `/api/auth/login` | 公开 | 登录，返回 `{token, user}`。`email` 与 `username` 等价，任一填了即可（字段名仍叫 `email`） |
| GET | `/api/auth/register/open` | 公开 | 注册是否开放 |
| GET | `/api/auth/me` | 登录 | 当前用户 |
| PUT | `/api/auth/me` | 登录 | 改资料 |
| POST | `/api/auth/logout` | 登录 | 登出 |
| POST | `/api/auth/change-password` | 登录 | 改密码（`current_password` + `new_password`）。**返回 `{message, token, expires}`**：bump `token_version` 后旧 token 全部失效，新 token 供当前会话无缝续用 |
| POST | `/api/auth/logout-all` | 登录 | 全设备登出（bump token version） |
| GET | `/api/me/stats` | 登录 | 我的统计 |
| GET | `/api/me/login-history` | 登录 | 我的登录记录 |

### 1.3 文章 — 读

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/posts` | 可选 | 列表。`?page=&size=&search=&category=&tag=&status=&sort=&order=`；**页面不传 `size`，由站点设置决定** |
| GET | `/api/posts/recent` | 可选 | 最近文章 |
| GET | `/api/posts/archive` | 可选 | 归档（旧版，全量；前端已改用 paged） |
| GET | `/api/posts/archive/paged` | 可选 | 归档分页（默认 50 / 上限 200 / `body` 恒空） |
| GET | `/api/posts/{id}` | 可选 | 单篇（**不计数**，计数走 `/view`） |
| GET | `/api/posts/{id}/public` | 公开 | 仅 published |
| GET | `/api/posts/{id}/nav` | 可选 | 上一篇 / 下一篇 |
| GET | `/api/posts/{id}/related` | 可选 | 相关文章（同分类或同标签，≤5） |
| GET | `/api/permalinks/{slug}` | 可选 | 永久链接解析（`/p/slug` 用） |
| GET | `/api/categories` | 公开 | 分类（两层树，含 `parent_id`） |
| GET | `/api/tags` | 公开 | 标签 |
| GET | `/api/authors/{username}` | 公开 | 作者主页数据 |

### 1.4 文章 — 写

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/posts` | 登录 | 新建（自动生成 slug） |
| PUT | `/api/posts/{id}` | 登录 | 更新；改标题不动 slug，显式传 `slug` 才改；**更新前自动存修订** |
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
| POST | `/api/upload` | 登录 | 上传图片 → `{url}`。扩展名白名单 **+ magic bytes 内容校验**（`http.DetectContentType` 只认 png/jpeg/gif/webp），HTML 伪装成 `.png` 返回 400 |
| GET | `/api/uploads` | 登录 | 图片库 |
| DELETE | `/api/uploads/{name}` | admin | 删除图片 |

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
| GET | `/api/trash` | admin | 回收站 |
| POST | `/api/trash/{id}/restore` | admin | 恢复 |
| POST | `/api/trash/empty` | admin | 清空 |
| GET | `/api/admin/actions` | admin | 审计日志 |

### 1.7 后台 — 用户 / 站点 / 备份

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/users` | admin | 用户列表 |
| POST | `/api/users` | admin | 新建用户（不可造 root） |
| PUT | `/api/users/{id}` | admin | 改用户（不可提升为 root） |
| PUT | `/api/users/{id}/disabled` | admin | 启用/禁用 |
| POST | `/api/users/{id}/reset-password` | admin | 重置密码（`canManageUser` 守卫）。重置后 bump 目标 `token_version`，其所有旧会话立即失效 |
| DELETE | `/api/users/{id}` | admin | 删除 |
| GET | `/api/users/{id}/logins` | admin | 该用户登录记录 |
| GET | `/api/site-config` | 公开 | 公开配置（品牌、`page_size`、`public_read`） |
| GET | `/api/admin/site-config` | admin | 全量配置（含 SMTP、图床 token） |
| PUT | `/api/admin/site-config` | admin | 保存（非 root 提交时继承 `auto_backup`/`backup_keep`） |
| POST | `/api/admin/mail/test` | admin | 测试邮件 |
| POST | `/api/backup` | admin | 导出 JSON 备份 |
| POST | `/api/backup/import` | admin | 导入（`dry_run` 支持） |
| GET | `/api/backup/download` | admin | 下载备份文件 |
| GET | `/api/backup/snapshots` | root | 快照列表（`snapshot-*` 与 `auto-*`） |
| POST | `/api/backup/restore-snapshot` | root | 从快照恢复 |
| GET | `/api/admin/backup/auto` | root | 每日备份状态 |
| POST | `/api/admin/backup/auto` | root | 立即生成今日快照（幂等） |

---

## 2. 关键端点细节

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
  "recent_logins": []
}
```

`view_trend` **恒为 30 项**（无阅读的日子是 0，服务端补齐）。`recent_posts` 的 `body` 恒为空。

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

## 3.2 可见性矩阵（第 73 轮收紧「私密」）

判定唯一入口 `services.canSeePost`；列表过滤在 `ListPostsPage` 的 `applyFilters` 里用同一份 `authorOnly` 名单（`private` + `draft`），两者必须同步修改。

| 状态 | 匿名 | 作者本人 | 其他普通用户 | admin | root |
|---|---|---|---|---|---|
| `published` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `private` | ❌ | ✅ | ❌ | ❌ | ❌ |
| `draft`（第 22 轮已废的遗留值） | ❌ | ✅ | ❌ | ❌ | ❌ |
| `pending` | ❌ | ✅ | ❌ | ✅ | ✅ |
| `rejected` | ❌ | ✅ | ❌ | ✅ | ✅ |
| `banned` | ❌ | ❌ | ❌ | ❌ | ✅ |

- 看不到时**一律 404**，不区分"不存在"与"存在但无权看"，避免泄露文章存在性。
- admin/root 读不到别人的 `private`：私密文章永不进入公开区，没有治理需求；只要能读，「仅自己可见」就是假话。审核需要看的 `pending`/`rejected` 仍对 admin/root 开放。
- 后果：后台「文章管理」对 admin 会少显示别人的 private/draft 行，这是预期行为。

---

## 4. 变更日志

| 轮次 | 变更 |
|---|---|
| 72 | 安全加固：① 文章载荷裁剪 author 敏感字段（见 §3.1）② `change-password` 响应新增 `token`/`expires`，旧 token 立刻失效 ③ `login` 支持 `username` 当标识 ④ `upload` 加 magic-bytes 内容校验 ⑤ 全站响应加安全头（`X-Content-Type-Options`/`X-Frame-Options`/`Referrer-Policy`/CSP/HSTS） |
| 52 | 新增 `GET /api/posts/{id}/related`、`GET/POST /api/admin/backup/auto`；dashboard 增加 `pending_total`/`view_trend`/`top_posts` |
| 51 | 新增 `GET /api/posts/{id}/revisions`、`POST .../revisions/{revID}/restore`、`POST /api/admin/import/markdown` |
| 48 | 新增 `GET /api/permalinks/{slug}`；封禁补前端入口（端点本已存在） |
| 47 | 新增 `GET /api/posts/archive/paged`、`GET /robots.txt` |
| 46 | 新增 `site_configs.public_read`、分类 `parent_id`、`POST /api/admin/categories/preset` |
| 43 | 新增 `POST /api/posts/{id}/view`、`PUT /api/tags/{id}`；`GET /api/posts/{id}` 不再自增浏览量 |
