# 顾客小程序订阅消息

当前版本只处理顾客侧微信小程序订阅消息，不依赖服务号，也不包含商户接单通知。

## 已配置模板

模板只绑定公版小程序 `wx087d633542ae8d0b`。独立小程序必须在自己的微信后台选用模板，并向
`miniapp_notification_templates` 写入该 AppID 对应的模板配置，不能复用公版模板 ID。

| 场景 | 模板标题 | 模板 ID | 字段 |
| --- | --- | --- | --- |
| `PICKUP_READY` | 取餐提醒 | `sPKz9ZotFXeTAQz08giDX9dcarm1uBGp9BqdtE-uQH8` | `character_string4` 取餐号、`thing2` 门店、`character_string1` 订单号、`phrase19` 状态、`thing11` 提示 |
| `RECHARGE_SUCCESS` | 会员充值成功通知 | `4Ft2cM2A8zyFFzn04v4TbLGDaggJxRVz_fQHuKtBCS4` | `amount4` 充值、`amount14` 赠送、`amount5` 余额、`date6` 时间、`thing8` 门店 |
| `BALANCE_CONSUMED` | 储值余额使用提醒 | `gMUJbiXDqPKC0LHG3yGpSrALVOw9VFDNh0YUU_4tMOU` | `thing3` 门店、`amount4` 消费、`amount1` 余额、`time6` 时间、`thing2` 提示 |

## 授权流程

- 顾客首次成功创建订单后，小程序集中申请三个模板。
- 后续自提订单申请一次取餐提醒；使用余额的订单同时申请一次余额消费提醒。
- 顾客确认充值后、创建支付单前，申请一次充值成功提醒。
- 每次微信授权结果都用 `requestId` 幂等写入服务端；客户端网络失败时保存在本地并在下次请求时补交。
- 拒绝或关闭授权不影响下单、支付、充值，业务页面也不会反复展示自定义强制弹窗。

## 发送边界

- 自提订单从商户端进入 `READY` 时写入取餐通知。
- 储值支付到账且本金、赠送金全部入账后写入充值成功通知。
- 订单余额实际扣减后写入余额使用通知；组合支付只显示实际扣除的余额金额。
- 一条 `accept` 记录最多被一个通知占用。业务类型与业务号有唯一约束，状态重放不会重复通知。
- 主业务事务只写 `miniapp_notification_outbox`。后台 worker 获取小程序稳定版 `access_token` 后发送，
  微信网络错误会退避重试，模板错误或用户无可用订阅次数会终止；发送结果不回滚业务。

## 上线检查

1. 确认生产环境 `TB_WECHAT_MINIAPP_APP_ID=wx087d633542ae8d0b`，并配置对应 AppSecret。
2. 自动迁移开启时部署会执行 `039_miniapp_subscription_notifications.up.sql`；否则先手工执行。
3. 在微信后台确认三个模板仍处于启用状态，字段名与上表完全一致。
4. 使用体验版真机分别验证：首次下单集中授权、自提订单变为请取餐、储值充值到账、余额全额支付、余额组合支付。
5. 通过 `miniapp_notification_outbox` 检查 `DONE`、`SKIPPED`、`DEAD` 状态与 `last_error`。
