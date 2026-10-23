> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 读者链路补全（归档分页 / robots / 列表封面 / 筛选空态）

> 状态：Implemented（第 47 轮）
> 关联：`docs/FEATURE_DESIGN_MATRIX.md` 第 1 批「读者链路」、`docs/TODO.md` F-4
> 最后更新：2026-10-22

## 1. 背景

第 45 轮的覆盖矩阵把"读者链路"列为第一批缺口。真正动手时发现：**实现度比文档度高**——阅读页其实已经有目录（TOC）、阅读进度条、阅读时长、字数、图片灯箱、代码复制、上一篇/下一篇，只是从未被 spec 记录过（所以矩阵里被我判成 ❌）。

因此本轮的清单收窄为**四个确有其缺的点**：

1. **归档页全量加载**（TODO F-4）：`GET /api/posts/archive` 一次返回所有 published 文章（正文虽已清空，但 `Author`/`Category`/`Tags` 三张关联全 Preload）。12 篇没事，2000 篇会拖垮 1 vCPU。
2. **没有 robots.txt**：唯一能声明 sitemap、并让爬虫别进 `/admin`、`/api` 的地方。
3. **封面选了没效果**：`posts.cover` 字段早就有、编辑器也能选，但任何列表都不渲染它。
4. **筛选无结果时文案错**：搜索没命中会显示"暂无文章，点击上方按钮写第一篇"——对一个只是搜错词的访客是误导。

## 2. API 契约

### `GET /api/posts/archive/paged?page=&size=`（optional auth）

- `size` 默认 50，上限 200；`page` 从 1 开始。
- 响应 `PostPage`：`{items, total, page, page_size}`，`items[].body` 恒为空串。
- 只包含 `published`。
- `page`/`size` 显式传入但非数字 → `400 page and size must be integers`。
- 旧的 `GET /api/posts/archive`（全量数组）**保留**，供兼容与测试，前端不再使用。

### `GET /robots.txt`（公开）

- `public_read = true`：
  ```
  User-agent: *
  Allow: /
  Disallow: /admin
  Disallow: /api/
  Disallow: /login

  Sitemap: <base>/sitemap.xml
  ```
- `public_read = false`：`User-agent: *\nDisallow: /`（站点锁了就不要向爬虫广告 401 的 URL）。

## 3. 用户故事 + 验收标准（EARS）

- **US-1**：作为访客，我要能翻完整个归档而不卡。
  - **AC-1.1**：WHEN 打开 `/archive` THEN 只请求 50 条，底部出现分页
  - **AC-1.2**：WHEN 文章总数 < 50 THEN 不显示分页控件
  - **AC-1.3**：WHEN 翻到最后一页 THEN 展示剩余条目，月份分组不跨页断裂（按页内月份分组）
- **US-2**：作为站主，我要爬虫正确抓取且不打扰后台。
  - **AC-2.1**：WHEN 访问 `/robots.txt` THEN 返回 `text/plain`，含 sitemap 绝对地址与 `/admin`、`/api/` 的 Disallow
  - **AC-2.2**：WHEN `public_read=false` THEN robots 全站禁止抓取
- **US-3**：作为访客，我要看到文章封面。
  - **AC-3.1**：WHEN 文章有 `cover` THEN 列表项右侧显示 104×70 缩略图（`object-fit: cover`，懒加载）
  - **AC-3.2**：WHEN 文章无 `cover` THEN 不占位、不显示占位框
- **US-4**：作为访客，搜不到东西时我要知道"是没匹配"而不是"站里没文章"。
  - **AC-4.1**：WHEN 有任一筛选（关键词/状态/分类/标签）且结果为空 THEN 显示"没有找到匹配的文章"+"清除筛选"按钮
  - **AC-4.2**：WHEN 无任何筛选且结果为空 THEN 显示原有"暂无文章"文案
  - **AC-4.3**：WHEN 点"清除筛选" THEN 关键词与状态清空、分类与标签清空、回到第 1 页

## 4. 边界

| 情况 | 行为 |
|---|---|
| `size=5000` | 钳到 200 |
| `page=99`（越界） | 空数组 + 真实 `total`，不报错 |
| 归档页无文章 | 空状态文案（不再区分登录与否——归档本就是公开内容） |
| 封面 URL 失效 | 浏览器显示破图（与正文图片一致，未做占位兜底） |
| `robots.txt` 在锁站状态 | 只有 `Disallow: /`，不泄露 sitemap |

## 5. 测试要点

- 服务层：分页切片正确、`body` 被清空、非 published 不出现、默认值 1/50、`size` 上限 200。
- 手测：`/api/posts/archive/paged?page=3&size=5` 返回剩余 2 条（12 篇时）；`?page=abc` → 400；`/robots.txt` 内容随开关变化。

## 6. 本轮未做（仍留在矩阵里）

- **文章 slug / 永久链接**：URL 仍是 `/read/{uuid}`。改它涉及 sitemap、RSS、旧链接兼容，需产品决策，未擅自改。
- **SEO 元信息（OG / canonical）**：本站是纯 SPA，社交平台抓取不执行 JS，客户端设置 meta 对微信/Twitter 无效；真要做需要服务端渲染或预渲染层，成本显著高于收益，先记录不实施。
- **定时发布**：需要与五态状态机重新协调。
