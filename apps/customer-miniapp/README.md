# 顾客微信小程序

使用微信开发者工具导入本目录。正式 AppID 在微信开发者工具的项目配置中填写，开发工具会生成不入库的 `project.private.config.json`。AppSecret 只保存在 API 服务器的 root-only 环境文件中，不能进入本目录。

环境配置位于 `miniprogram/config/env.ts`：

- `apiBaseUrl`：API 地址，生产环境默认为 `https://tbapi.666qwe.cn/api/v1`。
- `defaultStoreCode`：开发工具未通过商户小程序码启动时使用的演示门店编码。
- `paymentMode`：初版使用 `mock`；拿到天阙生产参数后改为 `tianque`。

正式发布前还需在微信公众平台配置 request 合法域名，并将 `tbapi.666qwe.cn` 配置为 HTTPS 域名。

## 门店码与桌码

- 普通门店码继续使用 `pages/home/index?storeCode=<storeCode>`，只进入门店自取模式。
- 堂食桌码使用 `pages/home/index?scene=tc%3D<publicScene>`。`publicScene` 为后端生成的 20–32 位不可猜公开标识。
- 小程序通过 `GET /public/table-codes/{publicScene}` 解析门店和桌台；创建堂食订单时额外发送 `order_scene=DINE_IN` 与 `table_public_id`。
- 冷启动没有二维码参数时会清除旧桌台，避免顾客离店后从“最近使用”误下单；堂食提交前还会再次解析桌码。
