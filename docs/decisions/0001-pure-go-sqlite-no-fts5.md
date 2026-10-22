> 类别 A · AI 指挥/约束（母文档族）— 由根目录 `AGENTS.md` 统管，新对话须先读。

# 纯 Go SQLite 驱动不支持 FTS5，搜索保留 LIKE

> 状态：Accepted
> 日期：2026-09-20
> 影响 spec：01-list-page（搜索）, 02-read-page
> 轮次：第 13 轮

## Context

第 13 轮计划引入 SQLite FTS5（contentless 表）替代 LIKE 搜索，以支撑未来文章量增长。实测创建 `CREATE VIRTUAL TABLE ... USING fts5(..., contentless=1)` 时报 `SQL logic error: unrecognized option: "contentless"`。根因：当前驱动的纯 Go 实现未编译 FTS5 扩展。

## Decision

保留 LIKE 搜索（标题 / 正文 / 作者名），不引入 FTS5；测试库改用 `:memory:` 消除 Windows 文件句柄问题；在 `services.go` 注释标明将来换驱动可平滑切换。

## 考虑的选项

| 选项 | 优点 | 缺点 |
|------|------|------|
| 换 `mattn/go-sqlite3`（cgo，支持 FTS5） | 真 FTS 全文搜索 | 需 `CGO_ENABLED=1` + gcc 工具链；二进制 10→20 MB；失去纯 Go 零 C 依赖卖点；VPS 需装 `build-essential` |
| 保留 LIKE | 零额外资源，千级文章内毫秒级 | 万级以上变慢 |
| 换其他纯 Go 驱动（如 `modernc.org/sqlite` 直连并启用 FTS5） | 保留纯 Go | 该驱动也未内置 FTS5 扩展，同样不可行 |

## Consequences

- 变得容易的：部署保持纯 Go 单二进制，无 C 依赖
- 变难的：文章量到 10k+ 时 LIKE 性能下降（需重新评估）
- 反悔成本：将来换驱动需迁移 FTS 表 + 重建索引 + 改搜索分支，约 1 轮工作量

## 参考

- 第 13 轮 `services.go` 搜索分支注释
- `PROGRESS.md` §六 第 5 项
