# CGOForum Frontend

基于 React + TypeScript + Vite 的论坛前端，已对接后端真实接口。

## 功能覆盖

- 用户注册、登录、退出
- 文章列表与文章详情
- 文章搜索
- 热榜展示
- 创建 / 更新 / 删除文章
- 点赞 / 收藏切换
- 关注作者、关注流

## 本地运行

```bash
npm install
npm run dev
```

默认访问地址：`http://127.0.0.1:5173`

## 构建

```bash
npm run build
```

## 环境变量

可通过 `VITE_API_BASE` 配置后端地址。

示例：

```bash
VITE_API_BASE=http://127.0.0.1:8080 npm run dev
```

未配置时默认使用：`http://127.0.0.1:8080`

## 接口约定

后端返回结构：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

前端会自动处理 `Authorization: Bearer <access_token>`，登录成功后会将 `access_token` 存在本地。
