export type CustomerFeatureKey =
  | "DELIVERY"
  | "STORED_VALUE"
  | "BALANCE_PAYMENT"
  | "COUPON_REDEMPTION"
  | "LOTTERY_REWARD"
  | "MEMBERSHIP"
  | "PROFILE_SERVICE";

export type CustomerFeatureAvailability = {
  title: string;
  content: string;
  releaseBlocker: boolean;
};

export const customerFeatureCopy: Record<CustomerFeatureKey, CustomerFeatureAvailability> = {
  DELIVERY: {
    title: "外卖服务暂未开通",
    content: "本店当前支持堂食和到店自取，外卖开通后即可在这里下单。",
    releaseBlocker: false,
  },
  STORED_VALUE: {
    title: "储值服务暂未开通",
    content: "开通后可在这里选择储值优惠并查看余额，具体开放时间请关注门店通知。",
    releaseBlocker: true,
  },
  BALANCE_PAYMENT: {
    title: "余额付款暂未开通",
    content: "当前订单暂不能使用账户余额付款，请选择门店支持的其他付款方式。",
    releaseBlocker: true,
  },
  COUPON_REDEMPTION: {
    title: "优惠券使用说明",
    content: "结算时会自动选择当前订单可用且优惠最大的券；使用门槛、有效期和适用场景以券面说明为准。",
    releaseBlocker: false,
  },
  LOTTERY_REWARD: {
    title: "抽奖权益暂不可用于付款",
    content: "中奖记录会为您保留，权益使用方式和开放时间请关注活动说明。",
    releaseBlocker: true,
  },
  MEMBERSHIP: {
    title: "会员服务暂未开通",
    content: "本店开通会员服务后，可在这里查看会员等级和专属权益。",
    releaseBlocker: true,
  },
  PROFILE_SERVICE: {
    title: "该服务暂未开通",
    content: "本店开通后即可使用，具体开放时间请关注门店通知。",
    releaseBlocker: false,
  },
};

export const customerReleaseBlockers = (Object.keys(customerFeatureCopy) as CustomerFeatureKey[])
  .filter((key) => customerFeatureCopy[key].releaseBlocker);

export const customerExperienceCopy = {
  networkError: "网络连接失败，请检查网络后重试。",
  serviceError: "服务暂时繁忙，请稍后重试。",
  invalidQrCode: "二维码无效或已过期，请重新扫描门店提供的二维码。",
  wrongStoreQrCode: "该二维码不适用于当前门店，请重新扫码。",
  couponClaimed: "领取成功。满足门槛时，结算页会自动使用当前订单优惠最大的券。",
  couponPreparing: "优惠券服务正在准备中，暂不可领取。",
  lotteryTerms: "本活动免费参与，奖品数量有限，权益使用范围和有效期以活动说明为准。",
} as const;

const apiErrorCopy: Record<string, string> = {
  STORE_CLOSED: "门店休息中，暂时不能下单。",
  ITEM_UNAVAILABLE: "部分商品已下架或售罄，请重新选择。",
  INVALID_CONFIGURATION: "商品选项已更新，请重新选择。",
  INVALID_ITEM: "部分商品信息已更新，请重新选择。",
  ORDER_NOT_PAYABLE: "订单已关闭，请重新提交订单。",
  PAYMENT_PENDING: "付款结果正在确认，请勿重复付款。",
  PAYMENT_FAILED: "付款未完成，请重试或联系门店。",
  COUPON_NOT_AVAILABLE: "这张优惠券已失效、已使用或暂不可用，请重新选择。",
  COUPON_THRESHOLD_NOT_MET: "当前商品金额还未达到优惠券使用门槛。",
  COUPON_ORDER_TYPE_MISMATCH: "这张优惠券不适用于当前点餐方式。",
};

/** Convert API/SDK failures to customer-safe copy without exposing raw backend messages. */
export function customerSafeErrorMessage(error: unknown, fallback: string = customerExperienceCopy.serviceError): string {
  const value = error as { message?: unknown; statusCode?: unknown; code?: unknown } | null;
  const statusCode = Number(value?.statusCode);
  const code = typeof value?.code === "string" ? value.code : "";
  if (code && apiErrorCopy[code]) return apiErrorCopy[code];
  if (statusCode === 0) return customerExperienceCopy.networkError;
  if (statusCode === 401 || statusCode === 403) return "当前操作需要重新进入小程序后再试。";
  if (statusCode === 404) return "相关内容已下架或不存在。";
  if (statusCode >= 500) return customerExperienceCopy.serviceError;

  // Locally-created validation messages are allowed when they contain no
  // protocol, database or stack-trace vocabulary. API errors carry statusCode.
  const message = typeof value?.message === "string" ? value.message.trim() : "";
  const technical = /(database|sql|exception|stack|request:fail|url is not|https?:\/\/|\/api\/|\bmock\b|\bscene\b|\bv\d+\b)/i;
  if (!Number.isFinite(statusCode) && message && message.length <= 80 && !technical.test(message)) return message;
  return fallback;
}

export function showUnavailableFeature(key: CustomerFeatureKey, featureName?: string): void {
  const copy = customerFeatureCopy[key];
  wx.showModal({
    title: featureName ? `${featureName}暂未开通` : copy.title,
    content: copy.content,
    showCancel: false,
  });
}
