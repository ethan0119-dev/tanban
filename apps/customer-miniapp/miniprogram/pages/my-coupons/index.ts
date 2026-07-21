import type { TanbanAppOption } from "../../app";
import type { LocalCouponAsset } from "../../utils/coupon-wallet";
import { localCouponAssets } from "../../utils/coupon-wallet";
import { loadPageAppearance } from "../../utils/page-appearance";
import { formatBeijingDate } from "../../utils/datetime";

interface CouponAssetView extends LocalCouponAsset {
  amountText: string;
  thresholdText: string;
  claimedDate: string;
}

function viewOf(item: LocalCouponAsset): CouponAssetView {
  return {
    ...item,
    amountText: (item.discount_cents / 100).toFixed(item.discount_cents % 100 ? 2 : 0),
    thresholdText: item.coupon_type === "CASH" || !item.threshold_cents ? "无门槛" : `满 ¥${(item.threshold_cents / 100).toFixed(2)} 可用`,
    claimedDate: formatBeijingDate(item.claimedAt),
  };
}

Page({
  data: { coupons: [] as CouponAssetView[], appearanceStyle: "" },
  async onShow() {
    const appearance = await loadPageAppearance();
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setData({ coupons: localCouponAssets(storeCode).map(viewOf), appearanceStyle: appearance.appearanceStyle });
  },
  goCouponCenter() { wx.navigateTo({ url: "/pages/coupons/index" }); },
});
