> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# Spec 33 — 仪表盘统计（趋势 / 待审 / 最多阅读）

> 状态：Implemented（第 52 轮）｜关联：`docs/COMPETITIVE_GAP.md`

## 1. 问题

第 49 轮的买家审查给"后台"打了 8 分，但指出一个具体缺口：**仪表盘只有两个数字（文章总数、总浏览量）**，看不出"有没有人读、今天要不要处理什么"。而 `post_views` 表里已经存着每一条去重后的阅读记录，只是从来没有聚合过。

## 2. 目标

一次 `GET /api/admin/dashboard` 回答三个问题：

| 问题 | 字段 |
|---|---|
| 有没有人在读？ | `view_trend`：近 30 天逐日去重阅读数 |
| 今天要我处理什么？ | `pending_total`：待审核文章数 |
| 大家读什么？ | `top_posts`：浏览量前 5 的已发布文章 |

## 3. 行为

- **WHEN** 读取仪表盘 **THEN** `view_trend` 恰好 30 个元素，按日期升序，最后一个是今天（UTC）。
- **WHEN** 某天没有阅读 **THEN** 该项仍存在且 `views = 0`（服务端补全，前端不负责填洞）。
- **WHEN** 统计最多阅读 **THEN** 只考虑 `status = 'published'` 且 `view_count > 0` 的文章，按 `view_count DESC` 取 5 条。
- **WHEN** 读取仪表盘 **THEN** `recent_posts` 的 `body` 必须为空（第 49 轮定下的规则，此处作为回归保护）。

## 4. 边界

| 场景 | 结果 |
|---|---|
| 全新站点，无任何阅读 | `view_trend` 30 个 0；前端显示"近 30 天还没有阅读记录" |
| 全部文章浏览量为 0 | `top_posts = []`；前端显示"还没有读过任何文章" |
| 有 pending 文章 | `pending_total` 反映真实值；非 root/admin 不可访问该端点 |

## 5. 非目标

- 来源渠道（referrer）：`post_views` 没有该列，加列等于加一次上报负载，暂不做。
- 图表库：前端用手写 SVG（30 个 `rect`），不引入任何依赖。

## 6. 验收

| 编号 | 条件 | 证据 |
|---|---|---|
| AC-1 | 趋势恒为 30 项且含 0 值 | `TestDashboardTrendAndTopPosts` |
| AC-2 | 今日阅读数正确 | 同上（种 2 条今日记录 → 末项 = 2） |
| AC-3 | 待审计数正确 | 同上（1 篇 pending → `pending_total = 1`） |
| AC-4 | 最多阅读排除非published | 同上 |
| AC-5 | 真机返回 30 点 | 第 52 轮实测：`trend points = 30`，top 5 全部为已发布文章 |
