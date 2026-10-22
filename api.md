> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# API 参考

Base URL：`http://localhost:8080`（可用 `PORT` 环境变量覆盖）

所有需要鉴权的接口都要带请求头：`Authorization: Bearer <token>`。

---

## 健康检查

### `GET /health`
返回服务状态。

```json
{ "status": "ok" }
```

---

## 认证

### `POST /api/auth/register`
注册新用户。

请求体：
```json
{ "username": "bob", "email": "bob@example.com", "password": "secret" }
```

- `201 Created`：`{ "id": "...", "username": "bob", "email": "bob@example.com", "role": "user" }`
- `400 Bad Request`：字段缺失
- `409 Conflict`：用户名或邮箱已存在

### `POST /api/auth/login`
登录，`email` 字段可填用户名或邮箱。

请求体：
```json
{ "email": "admin", "password": "admin123" }
```

- `200 OK`：
```json
{
  "token": "eyJ...",
  "user": { "id": "...", "username": "admin", "email": "admin@example.com", "role": "admin", "created_at": "..." },
  "expires_at": "..."
}
```
- `401 Unauthorized`：凭据错误

### `GET /api/auth/me`  (需鉴权)
返回当前登录用户信息。

### `POST /api/auth/logout`  (需鉴权)
客户端登出信号（JWT 无状态，服务端不真正吊销）。

```json
{ "message": "logged out" }
```

---

## 文章

### `GET /api/posts`  (公开)
文章列表，按创建时间倒序。
- 查询参数：`?status=published` 可选
- `200 OK`：文章数组

### `GET /api/posts/:id`  (公开)
单篇文章。
- `200 OK`：文章对象
- `404 Not Found`：不存在

### `POST /api/posts`  (需鉴权)
创建文章（作者为当前登录用户）。

请求体：
```json
{ "title": "新文章", "body": "正文", "status": "published" }
```
- `title` 必填；`status` 默认 `published`
- `201 Created`：新文章对象
- `400 Bad Request`：缺少 title
- `401 Unauthorized`：未带 token

### `PUT /api/posts/:id`  (需鉴权)
编辑文章，仅作者本人或 admin 可操作。

请求体（字段可部分提供）：
```json
{ "title": "改标题", "body": "改正文", "status": "draft" }
```
- `200 OK`：更新后的文章
- `403 Forbidden`：非作者且非 admin
- `404 Not Found`：文章不存在

### `DELETE /api/posts/:id`  (需鉴权)
删除文章，仅作者本人或 admin 可操作。
- `204 No Content`：删除成功
- `403 Forbidden`：非作者且非 admin
- `404 Not Found`：文章不存在

---

## 错误格式

统一 JSON：
```json
{ "error": "人类可读的错误信息" }
```

常见状态码：`400` 参数错误、`401` 未鉴权、`403` 无权限、`404` 资源不存在、`409` 冲突、`500` 服务端错误。
