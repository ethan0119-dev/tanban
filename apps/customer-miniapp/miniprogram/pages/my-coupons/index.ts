import type { TanbanAppOption } from "../../app";
import type { LocalCouponAsset } from "../../utils/coupon-wallet";
import { localCouponAssets } from "../../utils/coupon-wallet";
import { loadPageAppearance } from "../../utils/page-appearance";

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
    claimedDate: item.claimedAt.slice(0, 10),
  };
}

Page({
  data: { coupons: [] as CouponAssetView[], activeTab: "USABLE", appearanceStyle: "" },
  async onShow() {
    const appearance = await loadPageAppearance();
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setData({ coupons: localCouponAssets(storeCode).map(viewOf), appearanceStyle: appearance.appearanceStyle });
  },
  chooseTab(event: WechatMiniprogram.BaseEvent) {
    this.setData({ activeTab: String(event.currentTarget.dataset.tab || "USABLE") });
  },
  goCouponCenter() { wx.navigateTo({ url: "/pages/coupons/index" }); },
  goMenu() { wx.switchTab({ url: "/pages/menu/index" }); },
});
