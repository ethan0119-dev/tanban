# 摊伴 SaaS 系统管理端

平台运营人员使用的 Web 管理端，基于 React、TypeScript、Vite 与 Ant Design，采用原生 Layout/Sider/Menu 组合成响应式 admin shell。

## 本地运行

```bash
npm install
cp .env.example .env.local
npm run dev
```

默认 API 地址为 `https://tbapi.666qwe.cn/api/v1`，可通过 `VITE_API_BASE_URL` 覆盖。

## 构建与测试

```bash
npm test
npm run build
```

构建产物位于 `dist/`，部署时需要将 SPA 路由回退到 `index.html`。

## 接口约定

- 登录：`POST /auth/login`，请求 `{ username, password, portal: "platform" }`
- 当前用户：`GET /auth/me`
- 经营总览：`GET /platform/dashboard`
- 管理员：`/platform/users`
- 商户：`/platform/tenants`
- 平台按“一个商户对应一家门店”管理租户生命周期；门店经营资料由商户后台维护。
- 支付配置：`/platform/settings/payment`
- 系统设置：`/platform/settings/system`
- 审计日志：`/platform/audit-logs`

响应兼容 `{ data, meta, error }`，列表数据兼容 `items/list/records/rows`。登录令牌兼容 `accessToken`、`token` 和 `access_token`，统一以 Bearer Token 发送；收到 401 后会清除本地令牌并退出登录。

支付密钥不会由此管理端读取或展示，只显示服务端返回的“是否已配置”状态。生产环境应使用服务器环境变量或密钥管理服务注入。
