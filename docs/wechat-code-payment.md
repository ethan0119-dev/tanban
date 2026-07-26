# 微信付款码支付接入

收银台使用微信支付普通服务商模式的 APIv2 付款码支付（MICROPAY）。顾客出示微信付款码，收银端通过平板摄像头、扫码枪或手动输入读取 18 位付款码，服务端以特约商户身份完成扣款。

## 上线前配置

平台 API 服务需要配置：

- `TB_PAYMENT_PROVIDER=wechat_partner`
- `TB_WECHAT_PAY_SP_MCH_ID`：服务商商户号
- `TB_WECHAT_PAY_SP_APP_ID`：服务商 AppID
- `TB_WECHAT_PAY_API_V2_KEY`：32 位 APIv2 密钥
- `TB_WECHAT_PAY_API_CERT`：API 证书 PEM、文件路径或 `file:/path/to/apiclient_cert.pem`
- `TB_WECHAT_PAY_PRIVATE_KEY`：API 证书私钥 PEM、文件路径或 `file:/path/to/apiclient_key.pem`
- `TB_WECHAT_PAY_SERVER_IP`：调用付款码支付接口的 API 服务器公网 IPv4

商户租户还需具备：

- `payment_provider=wechat_partner`
- 已绑定 `payment_merchant_no`
- 进件状态为 `ACTIVE`
- 产品授权状态为 `AUTHORIZED`

缺少任一条件时，收银台仍可使用现金和系统外支付，但“微信付款码”会显示为待配置且不可点击。

## 支付状态流

1. 本地先创建 `WECHAT_MICROPAY` 支付尝试，以唯一 `provider_request_no` 作为微信商户订单号。
2. 付款码只存在于本次请求内存中，不写入数据库、支付响应或审计日志。
3. 微信明确返回成功时，支付事务和订单在同一订单锁内落账。
4. 微信返回 `USERPAYING`、`SYSTEMERROR`、`BANKERROR` 或网络超时时，支付进入 `PENDING`，收银端与后台补偿任务只使用商户订单号查单，不重复提交付款码。
5. 操作员取消时先查单；支付发起满 15 秒且仍无明确结果才调用撤销接口。撤销需要 API 双向证书。
6. 延迟成功、重复支付等异常路径会进入现有 `PAYMENT_EXCEPTION` 处理，而不会静默覆盖账务。

## 终端要求

- 平板摄像头扫码需要 HTTPS 和相机权限。
- iPad、Android Pad 无相机权限时可使用蓝牙/USB 扫码枪；输入框支持扫码枪回车提交。
- 付款码属于敏感且短时有效的支付凭证，禁止截图、复制到工单或写入日志。

官方接口：

- 付款码支付：<https://pay.wechatpay.cn/doc/v2/partner/4011941293>
- 查询订单：<https://pay.wechatpay.cn/doc/v2/partner/4011941754>
- 撤销订单：<https://pay.wechatpay.cn/doc/v2/partner/4012218602>
