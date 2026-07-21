export type MerchantFeatureKey =
  | 'DELIVERY'
  | 'OFFICIAL_MINIAPP_CODE'
  | 'ONLINE_STORED_VALUE'
  | 'BALANCE_PAYMENT'
  | 'COUPON_REDEMPTION'
  | 'LOTTERY_REWARD_REDEMPTION'
  | 'MEMBERSHIP_AUTOMATION'
  | 'PAID_MEMBERSHIP'
  | 'CATALOG_PACKAGE_SALE'
  | 'CATALOG_TEMPLATE_APPLICATION'
  | 'MODIFIER_INVENTORY'
  | 'PRODUCT_AUTO_RESTOCK'
  | 'CLOUD_PRINTING'
  | 'PAY_AFTER_MEAL'
  | 'AUTO_ACCEPT_ORDER'
  | 'ORDER_VOICE_NOTICE'
  | 'ORDER_REMINDER'
  | 'AUTO_REFUND'
  | 'PICKUP_VERIFICATION'
  | 'CUSTOMER_REVIEW'
  | 'OFFICIAL_ACCOUNT_NOTICE';

export type MerchantFeatureAvailability = {
  title: string;
  description: string;
  /** A release blocker must not be presented as an enabled production capability. */
  releaseBlocker: boolean;
};

export const merchantFeatureCopy: Record<MerchantFeatureKey, MerchantFeatureAvailability> = {
  DELIVERY: {
    title: '外卖服务暂未开通',
    description: '当前门店可使用堂食和到店自取；外卖开通后，配送订单会在这里单独展示。',
    releaseBlocker: false,
  },
  OFFICIAL_MINIAPP_CODE: {
    title: '正式点单码正在准备中',
    description: '请联系平台管理员生成微信官方小程序码，生成后即可下载并用于桌牌、海报等经营物料。',
    releaseBlocker: true,
  },
  ONLINE_STORED_VALUE: {
    title: '在线储值暂未开通',
    description: '当前可维护储值规则和线下收款记录；在线充值开通前，顾客端不会发起扣款。',
    releaseBlocker: true,
  },
  BALANCE_PAYMENT: {
    title: '余额付款暂未开通',
    description: '账户仍会分别记录本金与赠送金额；余额付款开通前，这些资金不会自动抵扣订单。',
    releaseBlocker: true,
  },
  COUPON_REDEMPTION: {
    title: '优惠券抵扣暂未开通',
    description: '当前可维护券面、库存和领取规则；抵扣开通前，优惠券不会改变顾客订单金额。',
    releaseBlocker: true,
  },
  LOTTERY_REWARD_REDEMPTION: {
    title: '抽奖权益使用暂未开通',
    description: '活动和奖项可以提前维护；顾客中奖后，暂不能将优惠券奖品用于订单抵扣。',
    releaseBlocker: true,
  },
  MEMBERSHIP_AUTOMATION: {
    title: '会员自动成长暂未开通',
    description: '当前支持会员卡样式、等级和人工开卡；首单开卡、消费成长和自动升级暂不执行。',
    releaseBlocker: true,
  },
  PAID_MEMBERSHIP: {
    title: '付费会员暂未开通',
    description: '当前可登记线下已完成的等级订单；系统不会代顾客付款，也不会自动变更会员等级。',
    releaseBlocker: true,
  },
  CATALOG_PACKAGE_SALE: {
    title: '套餐销售暂未开通',
    description: '可以先维护套餐名称、售价和组成说明；开通前不会在顾客点单页出售。',
    releaseBlocker: true,
  },
  CATALOG_TEMPLATE_APPLICATION: {
    title: '配置模板自动套用暂未开通',
    description: '当前资料可维护并绑定到商品；模板不会自动改变商品选项、价格或打印方式。',
    releaseBlocker: true,
  },
  MODIFIER_INVENTORY: {
    title: '加料独立库存暂未开通',
    description: '加料可设置加价和选择规则；库存仍随商品人工维护，不会单独扣减。',
    releaseBlocker: true,
  },
  PRODUCT_AUTO_RESTOCK: {
    title: '每日自动补库存暂未开通',
    description: '请继续在商品列表手工沽清或补足库存，系统不会在营业日切换时自动恢复库存。',
    releaseBlocker: true,
  },
  CLOUD_PRINTING: {
    title: '云打印设备服务正在准备中',
    description: '打印模板和任务记录可以先行维护；正式设备开通后，模板无需重新配置。',
    releaseBlocker: true,
  },
  PAY_AFTER_MEAL: {
    title: '先用餐后结账暂未开通',
    description: '当前堂食采用先付款后制作；餐后结账开通前，请不要使用该模式承接订单。',
    releaseBlocker: true,
  },
  AUTO_ACCEPT_ORDER: {
    title: '自动接单暂未开通',
    description: '当前请由店员确认并开始制作，避免门店忙碌或异常时自动进入制作。',
    releaseBlocker: true,
  },
  ORDER_VOICE_NOTICE: {
    title: '订单语音提醒暂未开通',
    description: '请通过订单列表和站内通知关注新订单；可靠语音提醒开通后可在这里设置。',
    releaseBlocker: true,
  },
  ORDER_REMINDER: {
    title: '顾客催单暂未开通',
    description: '顾客端暂不展示催单入口；开放后可在这里设置两次催单之间的最短间隔。',
    releaseBlocker: true,
  },
  AUTO_REFUND: {
    title: '超时自动退款暂未开通',
    description: '当前退款需由有权限的人员核对订单和实收金额后发起。',
    releaseBlocker: true,
  },
  PICKUP_VERIFICATION: {
    title: '自提核销暂未开通',
    description: '当前请按取餐号人工交付；核销开通后，店员可验证顾客取餐凭证。',
    releaseBlocker: true,
  },
  CUSTOMER_REVIEW: {
    title: '顾客评价暂未开通',
    description: '顾客端暂不展示评价入口，开放时间请关注平台通知。',
    releaseBlocker: false,
  },
  OFFICIAL_ACCOUNT_NOTICE: {
    title: '微信服务号通知尚未启用',
    description: '可先保存希望接收的通知类型；完成服务号和接收人绑定后才会发送微信消息。',
    releaseBlocker: true,
  },
};

export const merchantReleaseBlockers = (Object.keys(merchantFeatureCopy) as MerchantFeatureKey[])
  .filter((key) => merchantFeatureCopy[key].releaseBlocker);
