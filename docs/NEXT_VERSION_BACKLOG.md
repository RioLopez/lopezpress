> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# LopezPress v0.2 待规划问题汇总

> 来源：2026-10-20 用户使用反馈（部署后实测）。
> 性质：本文件是 **规划/讨论草稿**，不是已立项 spec。v0.2 启动时把各小节拆成 `docs/specs/NN-*.md` 并同步进 `docs/TODO.md`。
> 当前版本（v0.1.0）代码基线参考：`internal/services/services.go`、`internal/models/models.go`、`deploy/docker/nginx.conf`、`web/src/`。

---

> **第 52 轮更新**：本文件中若干项已在 2026-10-22/23 落地，落地项请以 `docs/TODO.md` 为准，
> 这里只保留**尚未立项**的内容。已消化：浏览量去重、纯分页、标签改名、后台文章管理+批量、
> 公开阅读开关、两层分类、归档分页、robots、永久链接、修订历史、Markdown 导入、统计趋势、每日自动备份、相关文章。
> 仍待规划：主题/配色体系、OG 分享卡片（需 SSR）、WXR 导入、媒体缩略图、2FA、定时发布。

## 0. 总览

| # | 主题 | 类型 | 严重度 | 现状一句话 | 建议方向 |
|---|------|------|--------|------------|----------|
| 1 | 浏览量去重 | Bug / 功能缺陷 | 中 | 每次读取文章都 `view_count+1`，零去重 | 新增"谁看过"记录，按账号/设备去重 |
| 2 | 分类 vs 标签 + 多级/搜索/管理 | 功能设计 + 缺口 | 中 | 分类有增删改、标签只有删除（无重命名/搜索） | 多级分类 + 标签搜索/重命名/合并 + 计数修正 |
| 3 | 本地图片（logo/ico/封面）不显示 | Bug（已定位） | 高 | nginx 正则 location 抢了 `/uploads/` 反代 → 图片 404 | 一行修复：`location ^~ /uploads/` |

---

## 1. 浏览量去重（防刷量）

### 现状
- `internal/services/services.go:646-649`（`GetPost`，需登录路径）与 `:741-743`（`GetPostPublic`，公开路径）在每次成功读取文章后无条件执行：
  ```go
  UpdateColumn("view_count", gorm.Expr("view_count + 1"))
  ```
- 前端每进入一次文章详情页就调一次读取接口；用户**退出重登、换账号反复进**，每次都 +1。
- 结果：浏览量可被单人对同一篇文章无限刷高，不能反映真实阅读数。

### 根因
计数逻辑只做"累加"，没有任何"这条阅读来自谁、是否已经计过"的记录，所以无法去重。

### 方案建议
新增一张去重表（或在 Redis/内存里记，但低配机优先用 SQLite 表，简单可靠）：

```
post_views (
  post_id    TEXT    NOT NULL,
  viewer_key TEXT    NOT NULL,   -- 登录用户= user_id；匿名= 服务端下发的设备匿名ID
  first_seen_at TIMESTAMP,
  PRIMARY KEY (post_id, viewer_key)
)
```

读取文章时改为：
1. 解析身份 —— 登录用户取 `user.ID`；匿名用户取请求里的 `anon_id` cookie（首次访问时由后端 `Set-Cookie` 下发一个 httpOnly、带签名/随机的 `anon_id`，写进 `post_views` 的 `viewer_key`）。
2. 用 `FirstOrCreate` 插 `(post_id, viewer_key)`：插入成功才 `view_count + 1`，已存在则跳过。
3. 这样同一账号（退出重登仍是同一 `user_id`）或同一浏览器（同一 `anon_id`）对同一篇只计一次。

### 待用户拍板
- **去重粒度**：严格"一人/一设备永久一次"，还是"N 小时内不重复计"的滑动窗口？（建议先做永久一次，最真实；存量老数据可把现有 `view_count` 当作基数保留，不回填 `post_views`。）
- 匿名 `anon_id` 是否要"换浏览器/无痕=新设备"即可重置（可接受，不必强求跨设备去重）。

### 验收标准
- [ ] 同一账号反复退出重登，浏览量不再增长。
- [ ] 同一浏览器匿名访问同一篇只 +1；清 cookie/换无痕算新设备（符合预期）。
- [ ] 公开路径（未登录）与登录路径都走同一去重逻辑。

---

## 2. 分类 vs 标签：概念、现状与下一步

### 先讲清两者的本质区别（设计原则）
- **分类（Category）**：一篇文章**只能属于一个**分类（`models.Post.CategoryID` 是单值外键，`models.go:110`）。本质是"栏目/归档树"，**互斥**——好比文件夹。天然适合做**层级**（父→子）。
- **标签（Tag）**：一篇文章可贴**多个**标签（`post_tags` 多对多，`models.go:117` / `PostTag`）。本质是"非互斥的关键词/标记"——好比书签。扁平更自然，用于横向聚合与检索。

> 一句话：分类用来"把文章放进哪个架子"（互斥、可分层），标签用来"这篇文章讲什么主题"（可多个、可交叉）。

### 现状（基于代码）
- **分类**：`CreateCategory` / `UpdateCategory`（重命名）/ `DeleteCategory` 三件套齐全（`services.go:1427/1444/1466`）。无层级（`Category` 模型无 `ParentID`）、无搜索（`ListCategories` 全量返回）。
- **标签**：`DeleteTag` 存在（`:1536`），但 **没有 `CreateTag` / `UpdateTag`（重命名）**——标签是在发帖时按名字隐式创建/关联的（`linkPostTagsTx` 用已有 tag ID）。**重命名、搜索均缺失**。
- 计数：审计已知 `ListCategories`/`ListTags` 的计数含软删 + banned 文章（`services.go` 附近裸表统计），需要修正为排除软删/banned。

### 方案建议（分两块）

#### 2.1 多级分类（推荐做）
- `Category` 模型加 `ParentID *int64`（自引用，0/NULL = 顶级）。
- 新增 `ListCategoryTree` 返回树结构；前端分类选择器改树形（可折叠）。
- 文章仍只挂一个分类（可挂任意层级，列表按祖先聚合展示面包屑）。
- 迁移：现有分类 `ParentID` 全 NULL → 自动成为顶级，向后兼容。

#### 2.2 多级标签（需拍板，建议 v0.2 暂不做层级）
标签本质是扁平标记，"多级标签"通常有两种实现，需你定：
- **方案 A — 标签组（Tag Group）**：新增 `tag_groups` 表，标签归属某个组。仍是扁平标签，只是分组展示/筛选。成本最低。
- **方案 B — 标签父子层级**：`Tag` 加 `ParentID`，变成标签树。语义上略重，且"一篇文章多标签 + 标签又有父子"会让前端筛选变复杂。
- **建议 v0.2 先不做标签层级**，把精力放在你明确要的"管理"上：
  - **搜索**：`ListTags` / `ListCategories` 增加 `?q=` 模糊匹配 `name`（已有 `escapeLike` 工具可直接复用，防注入）。
  - **重命名**：补 `UpdateTag`（参考 `UpdateCategory` 的查重逻辑）。
  - **删除/合并**：`DeleteTag` 已有；补 `MergeTags(fromID, toID)`——把 from 的 `post_tags` 关联迁到 to 并删 from（解决"同名/错字标签"清理）。
  - **计数修正**：分类/标签计数排除软删 + banned。
  - **前端**：新增"标签管理"页（搜索框 + 列表 + 重命名/删除/合并操作），分类管理页补树形与搜索。

### 验收标准
- [ ] 分类支持多级树，发帖可选层级节点。
- [ ] 标签可搜索、可重命名、可删除、可合并。
- [ ] 分类/标签计数不含软删/banned 文章。
- [ ] （若拍板做）标签层级按选定方案落地。

### 待用户拍板
- 标签要不要做多级？做的话用 **标签组(A)** 还是 **标签父子层级(B)**？还是 v0.2 先只做搜索/重命名/合并？

---

## 3. 本地图片（logo / ico / 封面）不显示 —— 已定位 bug

### 现状
- 上传接口返回 `{ "url": "/uploads/<name>" }`（`handlers/upload.go:9`）；站点 logo/favicon/封面都存这个 URL（`SiteConfig.SiteLogo` / `SiteFavicon` / `Post.Cover`）。
- 后端经 `ServeUploads` 提供 `/uploads/{file}`；`deploy/docker/nginx.conf:49` 已配置 `/uploads/` 反代到 `api:8080`。
- 前端也正确渲染：`AdminSitePage.tsx` 上传后写 `site_logo`/`site_favicon`；`site.tsx:44-53` 动态注入 `<link rel="icon">`；`Layout.tsx` / `AdminLayout.tsx` 用 `site_logo` 显示 logo；`PostEditor.tsx` 显示封面。

### 根因（已确认）
`nginx.conf` 的 location 优先级导致图片请求被"截胡"：

```nginx
# 正则 location（第 68 行）—— 优先级高于下面的前缀 location
location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
    expires 30d;
    add_header Cache-Control "public, immutable";
}

# 前缀 location（第 49 行）—— 本应把 /uploads/* 反代给后端
location /uploads/ {
    proxy_pass http://echo_api;
    ...
}
```

Nginx 匹配规则：**正则 location 优先于普通前缀 location**。所以 `/uploads/logo.png`、`/uploads/favicon.ico`、`/uploads/cover.jpg` 这些带图片后缀的请求，会命中上面的正则块，被当成静态文件去 nginx 自己的 `root /usr/share/nginx/html` 找（那里根本没有 uploads 目录）→ **404**，永远到不了后端 `ServeUploads`。这正好解释"logo/ico/封面都不显示，哪怕上传了"。

（注：`nginx.conf:48` 的注释提到"systemd 版 nginx 漏了 `/uploads/` 块"——若你还有非 Docker 部署，那边也要同步修。）

### 修复（一行，成本极低）
把前缀 location 改成 `^~`（前缀优先，可压过正则）：

```nginx
- location /uploads/ {
+ location ^~ /uploads/ {
    proxy_pass http://echo_api;
    ...
}
```

`^~` 让 `/uploads/` 及其子路径**优先于正则**匹配，图片请求稳定走反代。

### 验收标准
- [ ] 上传 logo / favicon / 封面后，前台页头、后台页头、浏览器标签图标均正常显示。
- [ ] `curl -I http://<host>/uploads/<name>` 返回 `200 image/...`（而不是 404）。
- [ ] 非 Docker（systemd）部署的 nginx 同步补 `/uploads/` 反代（如仍在使用）。

### 备注
这是明确的 bug，**不一定非要等到 v0.2**——改一行 nginx 重 build 即可。建议尽快修（甚至本版热修），不影响其他功能。

---

## 4. 下一步动作
1. 用户拍板：浏览量去重粒度、标签是否做多级（A/B/暂不做）。
2. v0.2 启动时把上述三节转为正式 spec（`docs/specs/`）并录入 `docs/TODO.md`。
3. 图片 bug 可独立提前修复，不必等 v0.2。
