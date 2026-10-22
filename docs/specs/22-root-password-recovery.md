> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# root 密码离线恢复（CLI）

> 状态：Implemented（第 39 轮实现完成）
> 入口：`lopezpress -reset-root-password`（二进制 / `go run ./cmd/server -reset-root-password`）
> 权限：本机文件系统访问权（无需 HTTP、无需旧密码）
> 轮次：第 39 轮（对应 `docs/TODO.md` P1-17）
> 最后更新：2026-10-22

## 1. 背景

第 25 轮把角色升级为 `root` / `admin` / `user`，首启账号是**唯一 root**（`seedDemoData` 写入 `RoleRoot`）。P0-1 修复后，`POST /api/users/{id}/reset-password` 经 `canManageUser` 拦截「admin 重置 root」，root 改自己的密码又必须提供当前密码——于是形成死角：

> **root 忘记密码 = 全站失去最高权限，且没有任何 API 路径能救回来。**

此前只能直改数据库 `password_hash`（需要知道 bcrypt 怎么生成）或删库重建（数据全丢）。本 spec 提供一个受控的离线恢复入口。

## 2. 目标与非目标

**目标**

- 运维在本机（能访问 DB 文件）执行一条命令，即可把 root 密码换成新的随机值。
- 复用既有安全约定：新密码随机生成、只在 stderr 打印一次、不落日志；重置后强制改密。
- 重置同时**吊销 root 的既有会话**（可能正是他人冒用，恢复动作必须顺手清场）。

**非目标**

- 不做 HTTP 端点（那等于给出远程接管通道，违背 R2「root 不可经 API 创建/提升」的精神）。
- 不支持指定任意密码（避免密码进入 shell 历史 / 进程参数）。
- 不处理「root 账号本身被删」——那需要从备份恢复，属另一路径。

## 3. 用户故事 + 验收标准（EARS）

- **US-1**：作为站点所有者，我忘记 root 密码时能在服务器上自救。
  - **AC-1.1**：WHEN 执行 `lopezpress -reset-root-password` 且 DB 中存在 root THEN 生成新的 22 字符随机密码、更新 `password_hash`、在 stderr 打印一次横幅、进程以 0 退出且**不启动 HTTP 服务**
  - **AC-1.2**：WHEN 重置成功 THEN `must_change_password` 置为 true，下次登录被强制跳到 `/change-password`
  - **AC-1.3**：WHEN 重置成功 THEN `token_version` 自增 1，此前签发的所有 root JWT 立即 401
- **US-2**：作为运维，我不用担心误操作把数据搞坏。
  - **AC-2.1**：WHEN 指定的 `DB_PATH` 文件不存在 THEN 拒绝执行并以非 0 退出，**不创建新库**（避免在错误路径上静默 seed 出一个假站点）
  - **AC-2.2**：WHEN 库中不存在 root 账号 THEN 返回明确错误（`no root account found`）并以非 0 退出
  - **AC-2.3**：WHEN DB 是全新空库（由本次命令初始化）THEN 直接使用首启随机密码，不做二次重置
- **US-3**：作为 root，恢复后我仍受正常安全约束。
  - **AC-3.1**：新密码不写入任何日志文件，只出现在 stderr 横幅
  - **AC-3.2**：重置不改变 root 的角色与启用状态（`role` / `disabled` 不变）

## 4. 实现位置

| 文件 | 职责 |
|---|---|
| `internal/config/recover.go` | `ResetRootPasswordForCLI(db)`：查唯一 root → 生成密码 → 单条 UPDATE 同时改 hash / 置强制改密 / bump token_version |
| `cmd/server/main.go` | `-reset-root-password` 标志解析；`runResetRootPassword(cfg)`：DB 存在性闸门 → 取密码 → 打印横幅 → 退出码 |
| `internal/config/recover_test.go` | 轮换成功 / 无 root 报错 / token_version 递增 / 角色与禁用状态不变 |

## 5. 运维用法

```bat
:: 1) 停掉服务（避免与在线进程同时写 SQLite）
LopezPress.bat stop

:: 2) 执行恢复（默认读 .env 里的 DB_PATH）
lopezpress.exe -reset-root-password

:: 3) 复制终端横幅里的新密码，启动服务后用 admin 登录
::    登录后会强制跳转 /change-password 改成自己的密码
LopezPress.bat
```

> 若 DB 不在默认位置：`set DB_PATH=D:\lopezpress\data\app.db` 后再执行。
> 该命令等价于「能读这个 DB 文件的人可以拿到 root」，因此**文件权限就是权限边界**——与直接改库同级，但不会因为手写 bcrypt 出错。
