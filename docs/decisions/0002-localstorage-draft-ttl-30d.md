> 类别 A · AI 指挥/约束（母文档族）— 由根目录 `AGENTS.md` 统管，新对话须先读。

# 编辑器草稿 localStorage TTL 30 天 + 保存后回收

> 状态：Accepted
> 日期：2026-09-20
> 影响 spec：03-editor（新建/编辑）
> 轮次：第 14 轮

## Context

编辑器自动保存把未提交的草稿写到 `localStorage`（`echo.draft` / `echo.draft.<id>`），但没有过期机制。长期运行下，已发布/删除文章的草稿键会累积，浪费存储且可能误导用户（再次打开时恢复陈旧草稿覆盖已保存版本）。

## Decision

引入 `DRAFT_TTL_DAYS=30`：挂载时全量清理过期 draft 键；恢复 draft 前校验 `ts` 时间戳，过期即丢弃回退服务端版本；保存成功后主动删除对应 draft 键（`/create` 删 `echo.draft`，`/edit/<id>` 删 `echo.draft.<id>`）。

## 考虑的选项

| 选项 | 优点 | 缺点 |
|------|------|------|
| 固定 TTL 30 天 | 简单、可预测、自动卫生 | 用户可能 30 天后回来发现草稿没了 |
| 无 TTL（现状） | 草稿永存 | 累积膨胀 + 陈旧草稿覆盖风险 |
| 跟随会话（关闭浏览器即清） | 最干净 | 跨天写作场景丢失草稿，伤体验 |

## Consequences

- 变得容易的：localStorage 容量可控，陈旧草稿不再误导
- 变难的：30 天内的"僵尸草稿"仍保留（可接受）
- 反悔成本：改 TTL 常量一处即可

## 参考

- `web/src/pages/PostEditor.tsx`（`DRAFT_TTL_DAYS`、`pruneOldDrafts`）
- `web/src/pages/CreatePage.tsx` / `EditPage.tsx`（保存后回收）
