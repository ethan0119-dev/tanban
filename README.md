# 摊伴 TANBAN

面向咖啡摊、夜市摊、餐车和小型门店的多租户餐饮 SaaS。当前仓库包含正式一期代码、产品/技术设计文档，以及上一阶段用于确认产品方向的三端交互原型。

## 正式应用

| 模块 | 目录 | 生产入口 |
| --- | --- | --- |
| Go 模块化单体 API | `apps/api` | `https://tbapi.666qwe.cn` |
| SaaS 平台管理端 | `apps/platform-web` | `https://tbadmin.666qwe.cn` |
| 商户运营后台 | `apps/merchant-web` | `https://mysales.666qwe.cn` |
| 顾客微信小程序 | `apps/customer-miniapp` | 微信开发者工具导入 |

一期使用 MySQL 5.7、进程内缓存、Mock 支付和虚拟打印机。业务代码已保留 Redis、天阙/随行付和真实云打印 Provider 边界；拿到合作方参数与硬件后切换实现，不改订单领域状态机。当前公网部署是明确的联调环境，Mock 确认不代表真实到账。

## 功能范围

- 平台端：登录、经营总览、管理员用户、商户、门店、支付配置状态和审计日志。
- 商户端：经营总览、店内/外卖订单域、区域与桌码、分场景打印模板、商品/SKU/库存、分类/套餐/属性/加料/标签等配置库、顾客标签、会员等级与开卡、不可变余额流水、储值规则与记录、可发布/回滚的小程序装修、部分退款、打印机/打印任务、员工角色和门店设置。外卖订单入口一期仅作信息架构预留，禁止创建配送订单。
- 顾客端：门店码与桌码识别、已发布店铺装修、商品目录、规格/属性/加料选择、服务端权威计价购物车、带桌位快照的幂等堂食下单、Mock 支付和订单状态。
- API：JWT/RBAC、多租户隔离、金额整数分、支付/退款状态机、单订单支付互斥、支付与退款主动补偿、虚拟打印与补打、审计、migration 和健康检查。

## 本地开发

安装 Web 和小程序类型依赖：

```bash
npm install
```

分别启动两个管理端：

```bash
npm run dev:platform
npm run dev:merchant
```

启动 API：

```bash
cd apps/api
cp .env.example .env
# 填写本地 MySQL DSN 与随机 JWT Secret 后导出环境变量
go run ./cmd/server
```

根目录原型：

```bash
npm run dev:prototype
```

## 构建与测试

```bash
npm run build
npm test

cd apps/api
go test ./...
```

## 微信小程序配置

- AppID：在微信开发者工具导入 `apps/customer-miniapp` 后填写，生成的 `project.private.config.json`不提交。
- API、默认演示门店和支付模式：`apps/customer-miniapp/miniprogram/config/env.ts`。
- 正式发布前，把 `https://tbapi.666qwe.cn`加入微信公众平台 request 合法域名。

## 服务器发布提醒

- `scripts/server-deploy.sh` 会在变更 API/数据库前生成并校验 MySQL 备份，发布失败时恢复静态资源、Nginx 和上一 API 镜像；成功发布后默认保留最近 5 套回滚产物。
- 每日数据库 cron、首次 ACME 证书引导和成功发布后的人工回滚需要按 [服务器部署文档](docs/DEPLOYMENT.md) 单独配置/执行，部署脚本不会自动建立异机容灾。
- 当前公网环境开启 Mock 支付确认，只能联调。接入真实 Provider、关闭公开 Mock 确认、配置异机备份并完成恢复演练前，不得用于真实营业收款。

## 文档

- [产品结构、真实系统观摩与完整功能设计](docs/SYSTEM_BLUEPRINT.md)
- [对标商户后台完整菜单清单](docs/REFERENCE_FIREPOS_MERCHANT_ADMIN_MENU.md)
- [商户运营后台 V2：商品、用户会员与店铺装修](docs/MERCHANT_OPERATIONS_V2.md)
- [店内桌码、订单场景与打印模板设计](docs/DINE_IN_TABLE_CODES.md)
- [首页图片热区与结构化打印模板](docs/DECORATION_HOTSPOTS_AND_STRUCTURED_PRINTING.md)
- [一期技术架构、交易链路和扩容边界](docs/TECHNICAL_ARCHITECTURE.md)
- [支付 Provider 与天阙接入](docs/PAYMENT_PROVIDER.md)
- [服务器部署](docs/DEPLOYMENT.md)
- API 说明：`apps/api/README.md`

## 安全

- 不要提交 `.env`、身份证/银行卡资料、RSA 私钥、随行付商户密钥或微信 AppSecret。
- Mock 支付确认接口只在明确启用 Mock Provider 的调试环境开放。
- 正式支付状态只接受服务端验签通知或主动查单结果，客户端返回不能直接改变订单支付状态。
