# Tanban API

一期 API 是 Go 模块化单体，默认监听 `:18090`，运行于 MySQL 5.7。所有金额均使用整数分，所有 JSON 响应统一为：

```json
{"data": {}, "meta": {}, "error": null}
```

## 本地启动

```bash
cp .env.example .env
set -a; source .env; set +a
go run ./cmd/server
```

启动时默认执行 `migrations/*.up.sql`。当同时提供 `TB_BOOTSTRAP_ADMIN_USERNAME` 和 `TB_BOOTSTRAP_ADMIN_PASSWORD` 且数据库尚无平台管理员时，创建首个平台管理员。密码只从环境变量进入 bcrypt 哈希，不写入代码和迁移。

设置 `TB_SEED_DEMO=true` 并提供 `TB_DEMO_MERCHANT_USERNAME`、`TB_DEMO_MERCHANT_PASSWORD` 后，幂等创建“码农咖啡”、`manong-coffee` 门店、咖啡商品和商户老板账号。

## 核心接口

- `GET /healthz`、`GET /readyz`
- `POST /api/v1/auth/login`、`GET /api/v1/auth/me`
- `/api/v1/platform/*`：平台 dashboard、管理员、商户、门店、审计与系统/支付设置
- `/api/v1/merchant/*`：dashboard、员工、门店设置、分类、商品/SKU/库存、订单、交易、退款、打印机和打印任务
- `GET /api/v1/public/stores/{storeCode}/catalog`
- `POST /api/v1/public/stores/{storeCode}/orders`（必须提供 `Idempotency-Key`）
- `POST /api/v1/public/orders/{orderNo}/payments`
- `POST /api/v1/public/payments/{paymentId}/mock-confirm`（仅 mock 且显式允许时存在）
- `POST /api/v1/merchant/refunds`（必须提供 `Idempotency-Key`）

核心接口示例与数据结构见 [openapi.yaml](./openapi/openapi.yaml)。当前文件尚未覆盖全部平台端和商户端管理路由，联调时以服务端路由与实现为准，后续再持续补全契约。

## 一致性与安全边界

- JWT 中包含 user/tenant/role；每次请求重新查询账号状态，所有商户 SQL 都带 `tenant_id`。
- 顾客订单价格从数据库 SKU 计算，不信任客户端金额；建单事务原子扣库存，关闭未支付订单时恢复库存。
- 同一订单发起支付由 MySQL 命名锁串行化并有数据库唯一约束；支付成功依据 provider 回调/主动查询，更新具有幂等条件；Mock 只用于调试且确认能力默认关闭。
- 退款请求必须带幂等键，先在事务内预占可退额度，多次退款累计不能超过实付金额；渠道结果不明确时保持 `PENDING` 并由查询 Worker 补偿，不能把网络超时误判成退款失败。
- 打印事务只创建 `PENDING` 任务，后台 worker 调用打印供应商，支付回调不等待硬件。
- CORS 只允许 `TB_CORS_ALLOWED_ORIGINS` 白名单；登录按 IP+用户名限制五分钟内最多五次失败。
- 天阙 adapter 已保留配置和接口边界，但在取得 orgId/RSA 密钥、测试域名后才实现签名与生产调用，当前主动返回 `provider not configured`。
- 公共订单详情不返回手机号、openid 等个人信息。一期订单号包含随机成分但仍属于访问凭据；正式公网运营前应增加独立订单访问 token（服务端只存哈希），或引入顾客会话鉴权。

## 缓存和打印扩展

一期默认使用进程内 TTL Cache。`cache.Redis` 是明确的适配器占位，不连接 Redis。打印默认使用虚拟打印机，并保留芯烨等云打印 adapter 边界。
