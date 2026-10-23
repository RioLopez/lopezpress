> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# Markdown 批量导入

> 状态：Implemented（第 51 轮）｜关联：`docs/COMPETITIVE_GAP.md` G-5（最大的销售阻碍）
> 最后更新：2026-10-23

## 1. 背景

没有导入器，有存量博客的人**换不过来**——只能一篇篇复制粘贴。这是买家视角里评分最低的一项（3/10）。

## 2. 目标与非目标

**目标**

- 一次选中多个 `.md` 文件上传，批量建文章。
- 支持 YAML frontmatter（`title` / `date` / `category` / `tags` / `draft`），没有 frontmatter 时用**第一个一级标题**或**文件名**当标题。
- **保留原始发布时间**（`date` 或文件 mtime 不可得时用现在），这样归档的时间线不乱。
- 分类与标签按名称**复用或新建**，不制造重复。
- 逐文件报告成功/跳过及原因，部分失败不影响其他文件。

**非目标**

- 不支持 WordPress WXR（XML）——体积大、格式脏，留作下一轮。
- 不做图片迁移（正文里的外链图片保持原样；本地图片需要另外处理）。
- 不做 zip 上传（先做最直接的"多选文件"）。

## 3. API

`POST /api/admin/import/markdown`（auth + admin/root，multipart/form-data，字段名 `files`）

- 单文件上限 2 MB，单次最多 200 个文件。
- 响应：`{ created: N, skipped: [{name, reason}] }`
- 权限：root 创建的文章直接 `published`；admin 创建的进 `pending`（沿用既有审核规则，不破例）。

## 4. 解析规则

| 输入 | 结果 |
|---|---|
| frontmatter `title: X` | 用之 |
| 无 title，正文首个 `# 标题` | 用之（并从正文移除该行） |
| 都没有 | 用文件名（去扩展名、`-`/`_` 转空格） |
| frontmatter `date: 2020-01-02` | 作为 `created_at`（支持 `YYYY-MM-DD` 与 RFC3339） |
| frontmatter `category: 技术笔记` | 按名找分类；不存在则在顶层新建 |
| frontmatter `tags: a, b` 或 `[a, b]` | 逗号/方括号都支持，按名复用或新建 |
| frontmatter `draft: true` | 建成 `private`（我们无 draft 态，私密是最近语义） |
| 正文为空 | 跳过，reason `empty body` |

## 5. 验收标准（EARS）

- **AC-1.1**：WHEN 上传 3 个带 frontmatter 的 md THEN 建出 3 篇文章，标题/分类/标签/时间与 frontmatter 一致
- **AC-1.2**：WHEN md 无 frontmatter 但有 `# 标题` THEN 用该标题，且正文不保留这行
- **AC-1.3**：WHEN 同名分类/标签已存在 THEN 复用，不新建重复
- **AC-1.4**：WHEN 某文件正文为空 THEN 该文件进 skipped，其余照常导入
- **AC-1.5**：WHEN admin（非 root）导入 THEN 文章进入待审队列
- **AC-1.6**：WHEN 导入完成 THEN 每篇都有 slug 与永久链接

## 6. 边界

| 情况 | 行为 |
|---|---|
| 超过 200 个文件 | 400 `too many files (max 200)` |
| 单文件 > 2 MB | 跳过该文件，reason `file too large` |
| 非 `.md`/`.markdown` | 跳过，reason `unsupported extension` |
| 文件名与已有文章重复 | 允许（标题不唯一） |
| 上传被中断 | 已处理的文件保留（逐文件提交，无全局事务） |

## 7. 测试要点

- frontmatter 全字段解析；
- 无 frontmatter 的标题回退（`# 标题` → 文件名）；
- 分类/标签复用；
- 空正文跳过；
- 时间保留（`created_at` 等于 frontmatter date）。
