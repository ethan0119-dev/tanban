# 摊伴商户运营后台

React + TypeScript + Vite + Ant Design Layout 实现的商户工作台。

## 本地运行

```bash
npm install
cp .env.example .env.local
npm run dev
```

默认 API 地址为 `https://tbapi.666qwe.cn/api/v1`，可通过 `VITE_API_BASE_URL` 覆盖。

## 构建与测试

```bash
npm run build
npm test
```

生产构建输出在 `dist/`。前端路由使用 history 模式，Nginx 需要将未知路径回退到 `index.html`。

## 鉴权与接口约定

- 登录：`POST /auth/login`
- 当前用户：`GET /auth/me`
- Token 保存于浏览器 localStorage，并通过 `Authorization: Bearer <token>` 发送。
- 收到 HTTP 401 后自动清理 Token 并返回登录页。
- 响应兼容 `{ data, meta, error }`，列表兼容 `items/list/records/rows` 常见结构。
- 金额在界面层以元展示，与 API 通信时按接口契约转换为 `*_cents` 整数分。

退款、订单状态和补打操作均以服务端成功响应为准；请求失败时不会在页面伪造成功状态。
