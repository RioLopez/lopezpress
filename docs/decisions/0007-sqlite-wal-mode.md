> 类别 A · AI 指挥/约束（母文档族）— 由根目录 `AGENTS.md` 统管，新对话须先读。

# 决策 0007：SQLite 改 WAL + 启动打印实际生效的 pragma

> 时间：2026-10-24（第 81 轮）｜状态：Accepted｜起因：真机测试中全库锁死 6 分钟

## 背景：一次真实的停摆

用户在本机测试上传功能时，后端从 `21:03:48` 起进入全面故障，持续约 6 分钟直到进程被重启：

```
21:03:55 [7208ms]  INSERT INTO upload_files ...        database is locked (5) (SQLITE_BUSY)
21:04:02 [14179ms] INSERT INTO action_logs ...         database is locked (5) (SQLITE_BUSY)
21:04:17 [17415ms] SELECT id FROM upload_files ...     WARN thumbnail queue: scan failed
21:05:57 [7310ms]  SELECT COUNT(*) FROM posts ...      GET /api/posts 500
21:09:40 [7307ms]  SELECT * FROM categories ...       GET /api/categories 500
```

三个特征决定了排查方向：

1. **读写全挂**：连 `SELECT * FROM categories` 都 500，不只是写。
2. **`/health` 一直是 200**：它不碰数据库，说明进程活着、HTTP 栈正常。
3. **不自行恢复**：持续 6 分钟，直到旧进程退出、新进程（`21:10:03` 起）接管才恢复。

## 实测到的基线（改之前）

用一次性诊断程序读**服务自己连接池里**的连接，而不是猜：

| pragma | 实际值 | 评价 |
|---|---|---|
| `journal_mode` | `delete` | 默认回滚日志，每次提交要建/删日志文件 |
| `busy_timeout` | `5000` | 驱动默认，够用（解释了日志里 ~7s 的等待） |
| `synchronous` | `2` (FULL) | 每次提交两次 fsync，写放大最严重的一档 |
| `foreign_keys` | `0` | 外键未启用（**本轮未改**，见下文） |
| `cache_size` | `-2000` (2 MB) | 低配 VPS 下可接受 |

根因指向很清楚：`sqlite.Open(cfg.DBPath)` **没有带任何 pragma**，全部吃驱动默认值。而第 78 轮新加的缩略图 worker 让后台写从「偶发」变成「每 150 ms 一次」，把一个潜伏的配置问题放大成了可见故障。

## 被证伪的假设（重要，别再重复排查）

排查过程中有两个"看起来很像"的解释，都**用实验推翻了**：

| 假设 | 怎么验的 | 结论 |
|---|---|---|
| 有漏关的 `Rows()` 游标长期持有读锁 | `grep -rn "\.Rows()" internal/ cmd/` → **0 处** | 证伪 |
| 事务内部调 `s.audit()` / `s.db`（走另一条连接）导致池死锁 | 逐个事务体扫描 `posts/posts_edit/media_refs/trash/auth_email/accounts_profile/backup/restore` → 事务内**无** `s.db`/`s.audit` 调用 | 证伪 |
| "delete 模式下写者会饿死读者" | 外部进程 `BEGIN IMMEDIATE` 持写锁 **10 秒**，期间连续请求 `/api/posts` → 全程 **22 ms、12 次全 200** | 证伪（RESERVED 锁不挡读，只有提交瞬间挡） |

同时**没能合成复现**：30 并发上传连打两轮、再叠加 137 张待处理缩略图、再跑 225 次持续上传，全部 0 个 5xx。所以"某个具体请求把它打挂"不成立。

剩下的解释只能在**进程层面**，无法从日志唯一确定：

- **两个进程同时开同一个 SQLite 文件**（会话里后台跑的实例 + 手工 `LopezPress.bat` 再起一个）——第二个会先跑完迁移/回填（都是写），再因端口占用退出；
- 某个写者被 `kill` 在提交中途，留下热日志（hot journal），后续所有连接都要先回滚它却抢不到排他锁。

两种都属于"写锁被长期占住"，与观测到的"读写全挂 + 重启即恢复"完全吻合。**这一点必须诚实标注为未证实。**

## 决策

### D1 换成 WAL，并把它当成文件属性只设一次

```go
// internal/config/config.go
db, err := gorm.Open(sqlite.Open(sqliteDSN(cfg.DBPath)), ...)
db.Exec("PRAGMA journal_mode = WAL")
```

WAL 让**读不被写阻塞**，且崩溃恢复变成"开库时重放"，不再有"热日志抢不到锁就全库僵死"这一类失效。写者再卡，代价是延迟而不是整站不可用。

**关键细节**：`journal_mode` 是**文件属性**，只设一次；`busy_timeout` / `synchronous` 是**连接属性**，必须每条池连接都设。前者若放进每连接的 `_pragma`，在别的连接持读锁时执行 `PRAGMA journal_mode=WAL` 本身会返回 `SQLITE_BUSY`，把连接创建搞挂——所以两者必须分开处理：

```go
func sqliteDSN(path string) string {
    sep := "?"
    if strings.Contains(path, "?") { sep = "&" }
    return path + sep + "_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
}
```

### D2 `synchronous` 降到 NORMAL

WAL + NORMAL 是 SQLite 官方推荐的组合：断电最多丢最后一个事务，**不会损坏数据库**。收益是提交不再 fsync 主库文件，实测延迟见下。

### D3 启动就把生效值打到日志

```go
log.Printf("sqlite: journal_mode=%s busy_timeout=%dms synchronous=%d", ...)
```

SQLite 的 pragma 是**静默的、分层的**（有的按文件、有的按连接），"配置了"和"真的生效了"能长期不一致却毫无症状。这次排查的第一小时就卡在"到底跑在什么模式"，所以把这一行变成启动标配——下次直接从日志读答案，不再写临时诊断程序。

### D4 明确"一个库只许一个进程"

这是本轮没写代码、但必须写进纪律的一条：`LopezPress.bat` 双击两次 = 两个进程开同一个库。开发机无所谓，生产由 systemd 保证。**凡是并发写 SQLite 的场景，先确认只有一个写者进程。**

## 实测收益（同库、同测试、只差 journal 模式）

30 并发上传，隔离实例 + 隔离库 + 同一批 1.4 MB 噪声 PNG：

| 指标 | 改前（delete + FULL） | 改后（wal + NORMAL） |
|---|---|---|
| 30 上传总墙钟 | 1.76 s / 0.90 s | **0.22–0.26 s** |
| 上传延迟 p50 | 1286 ms / 602 ms | **189–214 ms** |
| 上传延迟 max | 1750 ms / 856 ms | **236 ms** |
| 5xx | 0 | 0 |

持续负载 225 次上传 + 全程读探针：**0 个 5xx**，读 p50 29–119 ms。

启动日志实测输出：`sqlite: journal_mode=wal busy_timeout=5000ms synchronous=1`。

## 代价与已知不足

- **`-wal` / `-shm` 成为数据库的一部分**。任何"直接拷 `data/app.db`"的备份从此都是残的。仓库内脚本已核过没有这种用法；备份走 `VACUUM INTO` 快照或 JSON 导出两条正规路径。**手工备份也必须连带 `-wal`，或先停服务。**
- **断电丢最后一个事务**（NORMAL 的固有语义，非本方案缺陷）。
- **`foreign_keys` 仍是 0**：开外键会改变删除路径的行为（`hardDeleteUpload`、`DeleteUser` 的删除顺序），属于独立议题，不在"止血"里顺手改。
- 原始故障的**具体持锁者仍未证实**。本决策保证的是"同类故障不再拖死全站 + 下次能从日志看出模式"，不是"已定位到某一行为"。

## 验收证据

- `go vet ./...` + `go test ./...` 全绿（config 2.4 s / handlers 13.0 s / services 15.5 s）。
- `arch_check.py` 无规则回升（R2.funcs 增加 `sqliteDSN` / `logPragmas` 后仍在基线内）。
- `contract_check.py` 83 路由 / 76 前端调用无变化。
- 隔离实例（`:18085`，独立 `DB_PATH`/`UPLOAD_DIR`/`BACKUP_DIR`）验证后已清理，未污染 `data/app.db`。
