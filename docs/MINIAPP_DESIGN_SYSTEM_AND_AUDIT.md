# 顾客小程序视觉规范、装修模板与 2026-07-21 审计

## 1. 结论

餐饮小程序没有一套微信官方的“餐饮 SaaS 装修协议”，但有成熟的结构基线可用：微信原生体验可参考 WeUI，复杂零售流程可参考腾讯 TDesign Mini Program 及其零售模板。摊伴采用这些基线中的信息层级、设计令牌和完整交易结构，同时保留自己的餐饮场景、桌码、快餐码牌和商户装修能力。

主要参考：

- [WeUI WXSS](https://github.com/Tencent/weui-wxss)：微信设计团队提供的原生视觉基础库。
- [TDesign](https://github.com/Tencent/tdesign)：腾讯企业级设计体系，包含颜色、字体、间距、圆角、阴影等设计令牌。
- [TDesign Mini Program](https://github.com/Tencent/tdesign-miniprogram)：面向微信小程序的组件实现。
- [TDesign 零售模板](https://github.com/Tencent/tdesign-miniprogram-starter-retail)：覆盖首页、商品、购物车、订单和个人中心的完整零售结构，可作为点单小程序的信息架构参考。

## 2. 本轮截图证据

审计按“首页 → 点单 → 订单 → 我的 → 充值 → 商户装修”的完整顾客旅程进行，截图保存在 `.audit/miniapp-theme-2026-07-21/`：

1. [小程序首页](../.audit/miniapp-theme-2026-07-21/01-home.png)
2. [点单页](../.audit/miniapp-theme-2026-07-21/02-menu.png)
3. [订单页](../.audit/miniapp-theme-2026-07-21/03-orders.png)
4. [个人中心](../.audit/miniapp-theme-2026-07-21/04-profile.png)
5. [充值页](../.audit/miniapp-theme-2026-07-21/05-recharge.png)
6. [商户装修工作台](../.audit/miniapp-theme-2026-07-21/06-merchant-decoration-page.png)
7. [原全店风格配置](../.audit/miniapp-theme-2026-07-21/07-merchant-global-theme.png)
8. [原装修模板列表](../.audit/miniapp-theme-2026-07-21/08-merchant-decoration-templates.png)

审计发现：

- 首页大图能形成品牌感，但会掩盖订单、会员、充值和优惠券页面仍使用固定绿色的问题。
- 点单与订单页的部分辅助文字过小，场景 Tab 密集，主要操作的最小触控面积不足。
- 原全店风格只有离散颜色和圆角，没有字号、卡片质感、按钮造型，商户很难自行配出稳定方案。
- 原模板数量少，远端模板缺少说明时还会误用其他模板描述。
- 码农咖啡已发布 `warm-bakery` V2，但旧版本快照缺少 `navigation.templateKey`，公共 API 严格校验后错误降级到了默认绿色主题。这是“首页看起来暖调、其他页面仍绿色”的直接原因。

## 3. 统一设计令牌

所有顾客页面必须由同一份已发布装修配置生成以下令牌：

| 维度 | 可选值 | 作用 |
| --- | --- | --- |
| 字号 | 紧凑 / 标准 / 大字 | 统一 caption、body、subtitle、title、display 五级字号 |
| 卡片 | 平面 / 描边 / 浮层 | 统一边框与阴影，不允许页面自行写固定绿色阴影 |
| 按钮 | 直角 / 圆角 / 胶囊 | 统一主按钮、加购、领券和结算按钮 |
| 圆角 | 小 / 中 / 大 | 统一图片、卡片和容器圆角 |
| 颜色 | 主色、强调色、背景、表面、正文、辅助字、导航三色 | 由系统自动计算主色和强调色上的高对比文字色 |

默认标准字号为：caption 22rpx、body 26rpx、subtitle 30rpx、title 36rpx、display 44rpx；紧凑与大字各自形成完整比例，不做单个页面的随意放大缩小。

## 4. 成套模板

商户优先套用整套模板，模板同时影响配色、字号、卡片、按钮、首页模块顺序、点单布局和底部导航；套用后仍可在“高级自定义颜色”中输入 `#RRGGBB`。

当前内置十套：夜色咖啡、暖调烘焙、鲜果茶铺、清爽快餐、日式原木、海盐蓝调、奶油甜品、夜市霓虹、轻奢黑金、品牌通用。另保留 API 下发的咖啡暖调、夜市深色、简洁明亮作为服务端模板扩展样例。

模板卡必须展示适用业态、结构特点和核心视觉能力，不能只展示颜色名称。

## 5. 发布与向后兼容

- 商户装修草稿不会影响顾客端；只有“保存并发布”生成的不可变版本才对外可见。
- 公共 API 读取历史发布版本时，只在内存中补齐后续新增的受控字段，再执行安全校验；不能因为历史版本缺少新字段而整体降级为默认主题。
- 小程序所有业务页都读取同一个 `appearanceStyle`，禁止在充值、优惠券、订单等页面写死品牌绿色。
- 商户自定义颜色必须通过 `#RRGGBB` 校验；不接受任意 CSS、脚本、HTML 或自定义跳转路径。

## 6. 微信服务器域名

顾客小程序当前只请求 `https://tbapi.666qwe.cn`。微信公众平台需要在“开发管理 → 开发设置 → 服务器域名”配置 request、downloadFile 和预留的 uploadFile 合法域名。“业务域名”仅用于 `<web-view>`，不能代替服务器域名。

仓库中的公开和私有项目配置均必须保持 `setting.urlCheck: true`。关闭它只会让开发者工具绕过白名单，不能解决真实用户的 `url is not in domain list`。
