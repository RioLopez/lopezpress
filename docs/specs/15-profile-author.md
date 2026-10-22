> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 个人资料与作者主页

> 状态：Draft（第 19 轮新增）
> 路由：`/settings`（资料编辑，登录）、`/u/:username`（公开作者页）
> 权限：`/settings` 登录；`/u/:username` 公开
> 轮次：第 19 轮
> 最后更新：2026-09-22

## 1. 目标

用户编辑自己的资料（显示名 / 简介 / 头像）；提供**公开作者主页** `/u/:username` 展示该用户资料 +
其已发布文章。单人站即站点主人页，为多用户预留。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为用户，我要编辑个人资料。
  - **AC-1.1**：WHEN 打开 `/settings` THEN 表单回显 `display_name` / `bio` / `avatar`
  - **AC-1.2**：WHEN 保存 THEN `PUT /api/auth/me` 更新 + 成功 toast
  - **AC-1.3**：WHEN 上传头像 THEN 走 `/api/upload` 回填路径
- **US-2**：作为访客，我要看公开作者主页。
  - **AC-2.1**：WHEN 访问 `/u/:username` THEN 显示该用户 `display_name`/`bio`/`avatar` + 其 `published` 文章列表（按时间倒序）
  - **AC-2.2**：WHEN 用户不存在 / 被禁用 THEN 404
- **US-3**：作为访客，我要从文章页看到作者。
  - **AC-3.1**：WHEN 阅读页渲染 THEN 显示作者名并链接到 `/u/:username`（候选，本期可实现）

## 3. 前端

- **布局**（ASCII）：

```
/public 作者页 /u/:username
┌──────────────────────────────┐
│ 头像 显示名                   │
│ 简介                         │
│ ─── 他的文章 ───             │
│ 文章卡片列表（published）     │
└──────────────────────────────┘

/login 资料页 /settings
┌──────────────────────────────┐
│ 个人资料                      │
│ 头像 / 显示名 / 简介          │
│ [保存]                        │
└──────────────────────────────┘
```

- **组件树**：`Layout > AuthorPage`（公开）/ `Layout > ProfilePage（/settings）`
- **关键交互**：
  - ProfilePage：受控表单 + 头像上传 + 保存（busy 锁）
  - AuthorPage：`GET /api/authors/:username` → 渲染；空态/404
- **路由**：`/settings`（登录，取代旧版仅图床的设定，图床已迁至 spec 05）、`/u/:username`（公开，不 `Guarded`）
- **i18n 键**：`profile.*`、`author.*`
- **共享组件依赖**：`Layout` / `i18n` / `auth` / `Toast` / `UploadImage`

## 4. 后端

- **API 端点**：
  - `PUT /api/auth/me`（登录）→ `{ display_name?, bio?, avatar? }` 更新自身
  - `GET /api/authors/:username`（**公开**）→ `{ user:{display_name,bio,avatar,username}, posts:[{id,title,created_at,...}] }`（仅 published）
- **Service 方法**：`svc.UpdateMe(ctx, id, ...)` / `svc.GetAuthor(ctx, username)`（posts 过滤 `status='published'` 且 `deleted_at IS NULL`）
- **数据模型**：`users` 加 `display_name` / `bio` text / `avatar` text（AutoMigrate 加列）；`username` 已有 `uniqueIndex`（作者页寻址用）
- **权限检查**：`/settings` 需登录；`/u/:username` 公开
- **错误处理**：作者不存在 / 禁用 → 404；`PUT /me` 未登录 401

## 5. 数据流

```
资料：打开 /settings → 当前 user → 表单 → PUT /api/auth/me → toast
作者页：/u/:username → GET /api/authors/:username → 渲染资料 + published 文章
```

- **可能失败点**：头像上传失败 → 保留原值；用户名不存在 → 404
- **回退**：资料空时显示名回退 `username`

## 5.5 性能与资源

- **前端包体积**：两个页面复用卡片，影响 < 2 kB
- **后端查询量级**：作者页 1 用户 + 其文章（索引 `author_id` + `status`）
- **内存 / 磁盘**：无
- **慢点在哪**：作者文章到万级时加分页（候选）
- **是否需重新评估**：无

## 6. 边界情况

- 被禁用用户：作者页 404（不暴露）
- 显示名为空：回退 `username`
- 未登录访问 `/settings`：`Guarded` 跳 `/login`

## 7. 已知限制 / 未来方向

- 头像沿用 `UPLOAD_MAX_BYTES` 限制
- 候选：作者页 RSS、文章页作者链接、用户自查登录历史

## 8. 相关

- 依赖 spec：`specs/05-settings.md`（头像上传同走 `/api/upload`）、`specs/04-auth.md`
- 相关 spec：`specs/18-personal-center.md`（个人中心 `/me` 的「编辑资料」复用本页 `/settings`）、`specs/17-user-menu.md`（菜单「账号设置」入口指向本页）
- 相关决策：无
