# 支付 Provider 设计与天阙接入说明

> 当前交付状态（2026-07-19）：Mock 支付、关单、部分退款、幂等和主动补偿链路可运行；真实会生活/天阙适配器尚未获得合作方凭证和最终接口协议，仍会显式返回 `ErrNotConfigured`。本环境只能用于联调，不能用于真实营业收款。

## 1. 资金与系统边界

摊伴只保存业务订单、支付单和退款单，不保存商户可提现余额，也不把顾客资金结算到摊伴账户。每个租户绑定自己的随行付/天阙商户号 `mno`，支付机构直接向商户结算卡结算。

```text
tenant_id + store_id
        -> payment_account(provider=TianQue, merchant_no=mno)
        -> 天阙/随行付收单
        -> 商户结算银行卡
```

平台级合作方参数与商户级参数必须分开：

| 层级 | 参数 | 保存位置 |
| --- | --- | --- |
| 合作方 | `orgId`、我方 RSA 私钥、天阙公钥、生产域名 | 生产环境密钥配置，不进数据库明文、不进 Git |
| 商户 | `mno`、审核状态、费率、结算类型、微信子商户配置状态 | 一期使用 `tenants.payment_merchant_no/payment_sub_appid`；接入多账户时拆至 `payment_accounts`，敏感字段加密或脱敏展示 |
| 门店 | 默认支付账户、打印触发方式 | `stores/settings` |
| 顾客 | OpenID、支付尝试 | 顾客会话与支付记录，日志脱敏 |

## 2. Provider 契约

领域层只使用统一契约：

```go
type PaymentProvider interface {
    Name() string
    Create(ctx context.Context, input CreatePaymentRequest) (CreatePaymentResult, error)
    Query(ctx context.Context, providerTradeNo string) (QueryPaymentResult, error)
    Close(ctx context.Context, providerTradeNo string) error
    Refund(ctx context.Context, input RefundRequest) (RefundResult, error)
    QueryRefund(ctx context.Context, refundNo string) (QueryRefundResult, error)
}
```

一期注册可执行的 `mock`，并保留返回 `ErrNotConfigured` 的 `tianque` 适配器边界。API 已运行支付/退款对账 Worker：丢失回调时调用 `Query`，退款结果不确定时调用 `QueryRefund`。拿到合作方生产参数后，在适配器内补齐签名、验签、通知解析、关单和查询；订单领域代码不变。

## 3. 小程序支付

天阙公开 `TQP003`接口路径为 `/order/jsapiScan`。微信小程序支付至少需要：

```json
{
  "mno": "商户15位编号",
  "ordNo": "摊伴支付单号",
  "amt": "13.00",
  "payType": "WECHAT",
  "payWay": "03",
  "subject": "码农咖啡-经典美式",
  "trmIp": "顾客请求IP",
  "subAppid": "统一小程序AppID",
  "userId": "顾客OpenID",
  "notifyUrl": "https://tbapi.666qwe.cn/api/v1/payments/tianque/callback"
}
```

同步返回只表示预下单成功。小程序调用 `wx.requestPayment`，最终支付事实由验签后的异步通知或主动查询确认。

## 4. 支付状态确认

处理优先级：

1. `TQP005`成功异步通知：验签、核对 `mno/ordNo/amt`，事务内幂等落库。
2. 小程序支付控件返回后：立即请求 API 查单，API 必要时调用 `TQP004`。
3. 后台补偿：扫描超过阈值仍为 `PROCESSING` 的支付单，主动查询。
4. 日终对账：比较业务订单、支付交易和退款数据。

同一订单的发起支付操作使用 MySQL 连接级命名锁串行化，并由 `unique(tenant_id, order_id)`兜底，避免双击或并发请求形成两笔活跃支付。调用渠道前先持久化 `CREATING` 意图，并把当次 `mno/subAppid/OpenID` 快照在支付记录中；补偿重试只使用该快照，不能在商户绑定变化后误投到新账户。支付机构侧必须分别以业务订单号和退款号实现创建支付、退款幂等，否则无法覆盖“渠道成功、我方写库前进程退出”的极端窗口。

天阙成功通知可能重复，正确响应前最多重试 10 次；失败交易不依赖成功通知。因此回调必须幂等，待支付订单不能只等待回调。

任何一个客户端字段都不能直接把订单标记为已支付。服务端只有在渠道确认成功并且商户号、订单号、金额一致时，才能在同一事务中执行：

```text
payment   -> SUCCESS
order     -> PAID
print_jobs -> PAYMENT_SUCCESS（按门店触发策略）
```

当前版本在同一数据库事务内生成打印任务，由后台 Worker 重试，不阻塞支付请求；通用通知 Outbox 是后续能力，尚未实现。

## 5. 部分退款

天阙 `TQP006 /order/refund`支持 180 天内多次部分退款。每个退款请求还必须携带 `Idempotency-Key`，同一个键重复提交返回原退款记录；每次退款使用独立 `refund_no`，数据库事务对原支付记录加行锁并检查：

```text
本次退款金额 > 0
成功退款总额 + 退款中总额 + 本次退款金额 <= 原支付金额
```

同步状态可能为 `REFUNDING`，最终通过 `TQP007`查询或 `TQP036`通知确认。退款成功后再更新订单聚合退款状态，并生成退款打印/提醒事件。

## 6. Mock Provider

开发环境的 Mock Provider 必须遵守与真实 Provider 相同的状态约束：

- 创建支付只返回 `PENDING`。
- 单独的确认接口模拟支付机构成功通知。
- 重复确认结果一致。
- 支持部分退款、退款中和重复回调测试。
- 支持通过测试开关模拟金额不符、签名失败、超时和重复通知。

Mock 接口只能在 `PAYMENT_PROVIDER=mock` 且非正式天阙门店时使用；切换生产天阙后必须关闭公开确认能力。

## 7. 待商务获得的资料

- 合作方主体要求和协议。
- 生产 `orgId`、请求域名和天阙生产公钥确认。
- 商户进件权限或托管进件页面能力。
- 统一小程序 AppID 下多子商户配置流程。
- 费率、结算、分润和退款手续费规则。
- 生产联调商户、回调白名单、对账文件权限。

## 8. 官方资料

- [天阙开放平台](https://paas.tianquetech.com/)
- [支付 API 列表](https://payapi-test.suixingpay.com/app/doc/api/payment/pay.html)
- [TQP003 聚合支付](https://payapi-test.suixingpay.com/app/doc/api/payment/tqp003.html)
- [TQP004 支付查询](https://payapi-test.suixingpay.com/app/doc/api/payment/tqp004.html)
- [TQP005 支付通知](https://payapi-test.suixingpay.com/app/doc/api/payment/tqp005.html)
- [TQP006 申请退款](https://payapi-test.suixingpay.com/app/doc/api/payment/tqp006.html)
- [RSA 签名](https://payapi-test.suixingpay.com/app/doc/api/perface/signature.html)
- [测试/生产环境](https://payapi-test.suixingpay.com/app/doc/api/perface/publicKey.html)
