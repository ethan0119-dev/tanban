# 顾客微信小程序

使用微信开发者工具导入本目录。正式 AppID 在微信开发者工具的项目配置中填写，开发工具会生成不入库的 `project.private.config.json`。

环境配置位于 `miniprogram/config/env.ts`：

- `apiBaseUrl`：API 地址，生产环境默认为 `https://tbapi.666qwe.cn/api/v1`。
- `defaultStoreCode`：开发工具未通过商户小程序码启动时使用的演示门店编码。
- `paymentMode`：初版使用 `mock`；拿到天阙生产参数后改为 `tianque`。

正式发布前还需在微信公众平台配置 request 合法域名，并将 `tbapi.666qwe.cn` 配置为 HTTPS 域名。
