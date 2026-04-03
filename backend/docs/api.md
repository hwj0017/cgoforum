# CGOForum 后端对外接口文档

## 1. 基础信息
- Base URL: `http://localhost:8080`
- API 前缀: `/api`
- 认证方式: `Authorization: Bearer <access_token>`
- 刷新令牌: 登录后服务端写入 Cookie `refresh_token`
- 通用返回结构:

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

错误时：
- HTTP 400 -> `code: 400`
- HTTP 401 -> `code: 401`
- HTTP 403 -> `code: 403`
- HTTP 404 -> `code: 404`
- HTTP 500 -> `code: 500`

分页返回（用于列表接口）在 `data` 内采用：

```json
{
  "list": [],
  "cursor": "next-cursor",
  "has_more": true
}
```

## 2. 健康检查
### GET /health
- 鉴权: 否
- 响应示例:

```json
{
  "status": "ok"
}
```

## 3. 认证模块（Auth）
### 3.1 注册
### POST /api/auth/register
- 鉴权: 否
- 请求体:

```json
{
  "username": "alice",
  "password": "123456",
  "nickname": "Alice"
}
```

- 参数约束:
- `username`: 必填，长度 3~50
- `password`: 必填，长度 6~128
- `nickname`: 必填，长度 1~50

- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1001,
    "username": "alice",
    "nickname": "Alice"
  }
}
```

### 3.2 登录
### POST /api/auth/login
- 鉴权: 否
- 请求体:

```json
{
  "username": "alice",
  "password": "123456"
}
```

- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "<jwt-access-token>"
  }
}
```

- 说明: 同时在 Cookie 中设置 `refresh_token`（HttpOnly）

### 3.3 刷新令牌
### POST /api/auth/refresh
- 鉴权: 否（但必须携带 `refresh_token` Cookie）
- 请求体: 无
- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "<new-jwt-access-token>"
  }
}
```

### 3.4 退出登录
### POST /api/auth/logout
- 鉴权: 是
- 请求体: 无
- 成功响应示例:

```json
{
  "code": 0,
  "message": "success"
}
```

### 3.5 封禁用户（管理员）
### POST /api/auth/ban
- 代码语义: 管理员接口
- 请求体:

```json
{
  "user_id": 1002,
  "reason": "spam",
  "duration": 3600
}
```

- 参数说明:
- `duration`: 秒，`0` 表示永久封禁

### 3.6 解封用户（管理员）
### POST /api/auth/unban
- 代码语义: 管理员接口
- 请求体:

```json
{
  "user_id": 1002
}
```

## 4. 文章模块（Article）
### 4.1 创建文章
### POST /api/articles
- 鉴权: 是
- 请求体:

```json
{
  "title": "Go + Cgo 实战",
  "summary": "摘要",
  "content_md": "# 内容",
  "cover_img": "https://example.com/cover.png",
  "status": 1
}
```

- 参数说明:
- `status`: 0 草稿，1 发布；若传其他值，服务端默认置为 1

- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 2001,
    "status": 1
  }
}
```

### 4.2 文章详情
### GET /api/articles/:id
- 鉴权: 否
- 路径参数:
- `id` 文章 ID

### 4.3 更新文章
### PUT /api/articles/:id
- 鉴权: 是
- 路径参数:
- `id` 文章 ID
- 请求体:

```json
{
  "title": "更新后的标题",
  "summary": "更新后的摘要",
  "content_md": "更新后的内容",
  "cover_img": "https://example.com/new-cover.png",
  "status": 1
}
```

### 4.4 删除文章
### DELETE /api/articles/:id
- 鉴权: 是
- 路径参数:
- `id` 文章 ID

### 4.5 文章列表
### GET /api/articles
- 鉴权: 否
- 查询参数:
- `cursor` 游标（可选）
- `limit` 每页数量（可选，默认 20）

### 4.6 作者文章列表
### GET /api/users/:uid/articles
- 鉴权: 否
- 路径参数:
- `uid` 用户 ID
- 查询参数:
- `cursor` 游标（可选）
- `limit` 每页数量（可选，默认 20）

## 5. 互动模块（Interaction）
> 以下接口均需鉴权（Bearer Token）

### 5.1 点赞
### POST /api/articles/:id/like
- 路径参数:
- `id` 文章 ID
- 请求体: 无
- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "liked": true,
    "count": 10
  }
}
```

### 5.2 取消点赞
### DELETE /api/articles/:id/like
- 路径参数:
- `id` 文章 ID
- 请求体: 无
- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "liked": false,
    "count": 9
  }
}
```

### 5.3 收藏
### POST /api/articles/:id/collect
- 路径参数:
- `id` 文章 ID
- 请求体: 无

- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "collected": true,
    "count": 3
  }
}
```

### 5.4 取消收藏
### DELETE /api/articles/:id/collect
- 路径参数:
- `id` 文章 ID
- 请求体: 无
- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "collected": false,
    "count": 2
  }
}
```

### 5.5 关注作者
### POST /api/follow/:authorId
- 路径参数:
- `authorId` 作者用户 ID

### 5.6 取消关注作者
### DELETE /api/follow/:authorId
- 路径参数:
- `authorId` 作者用户 ID

### 5.7 关注流 Feed
### GET /api/feed/following
- 查询参数:
- `cursor` 游标（可选）
- `limit` 每页数量（可选，默认 20）

## 6. 排行模块（Rank）
### GET /api/rank/hot
- 鉴权: 否
- 查询参数:
- `window` 统计窗口，默认 `24h`（常见可选 `7d`）
- `limit` 条数，默认 20

- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": []
  }
}
```

## 7. 搜索模块（Search）
### GET /api/search
- 鉴权: 否
- 查询参数:
- `q` 搜索词（必填）
- `limit` 条数（可选，默认 20）

- 成功响应示例:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": []
  }
}
```

## 8. 鉴权与测试建议
- 先调用 `POST /api/auth/register`
- 再调用 `POST /api/auth/login` 获取 `access_token`
- 需要鉴权接口统一携带头：`Authorization: Bearer <access_token>`
- `POST /api/auth/refresh` 依赖 Cookie（Postman 默认同域会自动带）
