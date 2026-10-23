> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 文章永久链接（slug）

> 状态：Implemented（第 48 轮）
> 决策：用户 2026-10-22 选择**方案 A** —— 新增 `/p/{slug}`，**旧 `/read/{uuid}` 永久保留**
> 最后更新：2026-10-22

## 1. 背景

文章的唯一地址是 `/read/{uuid}`。对机器没问题，对人毫无用处：念不出来、印不出来、搜索引擎也看不出主题。这是"个人博客"和"业余站点"之间最直观的差别之一。

## 2. 目标与非目标

**目标**

- 每篇文章有一个可读的永久链接 `/p/{slug}`。
- **旧链接永不失效**（方案 A）：`/read/{uuid}` 继续工作，sitemap/RSS 优先使用 slug。
- slug 在创建时自动生成；**改标题不会改链接**（永久链接的价值就在"永久"）。
- 显式传入 slug 时以传入值为准（给将来"手动改链接"留好接口）。
- 历史文章启动时自动补 slug。

**非目标**

- 不做 301 重定向（方案 A 不需要：两个地址都有效）。
- 本轮**不做**编辑器里的 slug 编辑框（需改 PostEditor 表单结构，留待单独一轮）；接口已支持，只是没有 UI。
- 不做拼音转换（中文 slug 直接用汉字，URL 里合法且可读）。

## 3. 数据模型

```
posts.slug  TEXT  (index)        -- 迁移 0009
```

- 空字符串 = 该行没有 slug（理论上只会出现在回填失败时）。
- **不是**数据库唯一索引：去重在 service 层做，便于"冲突时自动加 -2"而不是报错。

## 4. 生成规则（`Slugify`）

| 输入 | 输出 |
|---|---|
| `Hello World` | `hello-world` |
| `Go 的 并发模型` | `go-的-并发模型` |
| `Post #1: Getting Started!` | `post-1-getting-started` |
| `!!!` | `""` → 回退为文章 id |
| 超长标题 | 截断到 80 个字符 |

规则：小写化；保留 ASCII 字母数字与中日韩字符；其余连续字符折叠成单个 `-`；去首尾 `-`。

冲突处理：同名已存在则依次尝试 `-2`、`-3`……（最多 200 次）。

## 5. API

| 端点 | 说明 |
|---|---|
| `GET /api/permalinks/{slug}` | **新增**，按 slug 取文章；可见性由 `canSeePost` 判定，不可见 → 与不存在同为 404 |
| `POST /api/posts` | 接受可选 `slug`；缺省由标题生成 |
| `PUT /api/posts/{id}` | 接受可选 `slug`；**缺省不动原链接** |
| `GET /sitemap.xml`、`GET /feed` | 有 slug 用 `/p/{slug}`，无 slug 回退 `/read/{id}` |

> ⚠️ 端点路径刻意**不在** `/api/posts/` 子树内：`/api/posts/by-slug/{slug}` 与既有 `/api/posts/{id}/public` 在 ServeMux 里互为歧义（两者都能匹配 `/api/posts/by-slug/public`），注册时直接 panic。这是本轮实际踩到的启动崩溃。

## 6. 用户故事 + 验收标准（EARS）

- **US-1**：作为作者，我要一个能念出来的文章地址。
  - **AC-1.1**：WHEN 创建标题为 `Hello World` 的文章 THEN slug 为 `hello-world`，`/p/hello-world` 可访问
  - **AC-1.2**：WHEN 再次创建同名标题 THEN 第二篇 slug 为 `hello-world-2`，两篇互不覆盖
  - **AC-1.3**：WHEN 标题不含任何可 slug 化字符（如 `!!!`）THEN slug 回退为文章 id，永不为空
- **US-2**：作为作者，我改标题时不想把链接改坏。
  - **AC-2.1**：WHEN 只改标题 THEN slug 不变
  - **AC-2.2**：WHEN 显式传入新 slug THEN 链接更新（冲突时自动去重）
- **US-3**：作为读者，旧链接必须还能用。
  - **AC-3.1**：WHEN 访问 `/read/{uuid}` THEN 正常打开（无论该文是否有 slug）
  - **AC-3.2**：WHEN 文章无 slug THEN sitemap 与 RSS 使用 `/read/{id}`
- **US-4**：作为访客，我不能用 slug 探测未公开文章。
  - **AC-4.1**：WHEN 未登录访问 private/pending/rejected/banned 文章的 slug THEN 404（与 id 路由一致）

## 7. 边界

| 情况 | 行为 |
|---|---|
| 历史文章（无 slug） | 启动时 `BackfillSlugs` 补齐（幂等，每次启动一次索引查询） |
| slug 含中文 | 保持中文，浏览器自动百分号编码 |
| slug 冲突 200 次 | 500 `could not allocate a unique slug`（实际不可能触发） |
| 软删除的文章占用 slug | 可以复用（查重只看未删行） |
| 改 slug 后旧 slug | 404（方案 A 只保证 `/read/{uuid}` 永久，不保证旧 slug） |

## 8. 测试要点

- `Slugify` 表驱动 7 例（含中文、纯符号、下划线/点号）；
- 创建同标题两篇 → `hello-world` / `hello-world-2`；
- 纯符号标题 → slug == id；
- 显式 slug 优先于标题；
- 改标题不改链接、显式改 slug 生效；
- 匿名读 private 的 slug → 404，作者读 → 200；
- sitemap 用 `/p/`，无 slug 时回退 `/read/`（两个用例）。

## 9. 已知限制

- 编辑器里没有 slug 输入框（接口支持，UI 未做）。
- 英文以外的混合标题（如 `iPhone 15 评测`）会得到 `iphone-15-评测`，可读但不算漂亮；不做拼音转换是刻意取舍。
