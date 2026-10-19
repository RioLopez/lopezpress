# 决策 0003：token 存储位置写死在 localStorage

## 背景

审计报告中指出前端把 JWT 存在 `localStorage`，并留下一个待决问题：
"若 XSS 风险高（如后续接入图床、第三方脚本），应改 cookie（HttpOnly + SameSite=Strict + 短期 + CSRF 防护）。"

本决策对该问题做最终判断，写死为 **继续用 localStorage**。

## 决策

- **token 存储介质**：`localStorage['echo.token']`（不变）
- **token 有效期**：默认 `JWT_EXPIRY_HOURS=1`（1 小时）
- **不引入第三方脚本**：前端 `web/package.json` 中不引入任何第三方 SDK
  （不含图床脚本、统计脚本、广告脚本等）
- **HTML 输出净化**：所有 Markdown 渲染结果在插入 DOM 前必须经过
  `DOMPurify` 白名单（当前 `marked` 输出直接 `dangerouslySetInnerHTML`
  的情况需逐一排查并替换）
- **CSP 策略**：`default-src 'self'; script-src 'self'; style-src 'self';
  img-src 'self' https:; connect-src 'self'; frame-ancestors 'none'`
  （CSP 以 HTTP 响应头形式由反向代理 / 静态服务器下发；本项目不直接渲染 HTML，
  故 CSP 主要对静态托管方而言）

## 理由（为什么不改 cookie）

1. **本项目无第三方脚本面。** 不引入图床 / 统计 / 广告 SDK，XSS 的攻击面
   只来自 Markdown 渲染（被 DOMPurify 闭环）。第三方脚本是 cookie 方案
   的主要威胁源，但这里没有。
2. **cookie 方案在低配 VPS 上收益有限。** `HttpOnly + SameSite=Strict`
   需要后端在登录 / 刷新时显式 `Set-Cookie`，并在 `/api/auth/logout` 时
   `Expires` 清除。当前 token 是无状态 JWT，cookie 方案意味着要引入
   服务端 session 表，违背"低资源、纯 Go、零依赖"的架构目标。
3. **短有效期 + DOMPurify + 无第三方脚本** 已把"XSS 偷到有效 token"的
   窗口压到 1 小时以内。攻击者拿到 token 后只能在该窗口内冒用，风险
   可接受。
4. **审计结论**：cookie 方案在"接入第三方脚本"前提下才有意义；当前项目
   不引入第三方脚本（写死在 §1 决策里），故 cookie 方案的收益不成立。

## 实现落地

| 改动点 | 文件 | 内容 |
|---|---|---|
| `JWT_EXPIRY_HOURS` 默认改 1 | `internal/config/config.go` | `envIntOr("JWT_EXPIRY_HOURS", 1)` |
| `.env.example` 注释 | `.env.example` | 加注释说明"本项目 1 小时，按需可调" |
| 图床 DOMPurify 排查 | `web/src/pages/ReadPage.tsx` | `marked` 输出经 `DOMPurify.sanitize` |
| 引入 DOMPurify | `web/package.json` | 新增依赖（本项目第一次引入第三方库，仅限 XSS 净化用途） |

## 风险与残余

- **残余风险**：未来若接入图床 / 第三方脚本，本决策立即失效，必须
  重新评估 cookie 方案。建议在 `web/package.json` 加一条注释：
  "// 引入任何新第三方脚本前，必须重新审视 docs/decisions/0003"。
- **浏览器兼容性**：localStorage 在 Safari 隐私模式下容量受限（~5 MB），
  但本项目 token 仅 ~500 字节，无问题。
- **清退 token**：`logout` 立即 `removeItem`，token 在 1 小时内自然过期，
  双保险。

## 与 cookie 方案的对比（审计方复核用）

| 维度 | localStorage（本决策） | cookie（HttpOnly + SameSite=Strict） |
|---|---|---|
| XSS 偷 token 风险 | 高（但被 DOMPurify + 1h 有效期压低） | 无（HttpOnly 不可读） |
| CSRF 风险 | 无（无 cookie） | 有（需 CSRF token） |
| 服务端复杂度 | 无 session 表 | 需 Set-Cookie / Expires / 可选 session 表 |
| 第三方脚本兼容性 | 好（无 cookie 依赖） | 差（需 SameSite=Strict 与三方脚本冲突） |
| 本项目现状适配度 | **高**（零第三方脚本） | 中（需引入 session 机制） |

## 何时重新评估

- 引入任何第三方脚本（图床、统计、广告）
- 接入 OAuth / SSO（跨域 cookie 复杂化）
- 上线敏感 / 金融 / 医疗场景（合规要求 HttpOnly）
