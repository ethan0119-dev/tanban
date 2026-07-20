import type { TanbanAppOption } from "../../app";
import { localCouponCount } from "../../utils/coupon-wallet";
import { tableContextForStore } from "../../utils/table-context";

Page({
  data: { version: "v0.2.0", storeCode: "", channelScope: "TAKEOUT", couponCount: 0 },
  onShow() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setData({ storeCode, couponCount: localCouponCount(storeCode), channelScope: tableContextForStore(storeCode) ? "DINE_IN" : "TAKEOUT" });
  },
  goOrders() { wx.switchTab({ url: "/pages/orders/index" }); },
  goCoupons() { wx.navigateTo({ url: "/pages/my-coupons/index" }); },
  goCouponCenter() { wx.navigateTo({ url: "/pages/coupons/index" }); },
  goRecharge() { wx.navigateTo({ url: "/pages/recharge/index" }); },
  goLottery() { wx.navigateTo({ url: "/pages/lottery/index" }); },
  unavailable(event: WechatMiniprogram.BaseEvent) {
    const feature = String(event.currentTarget.dataset.feature || "该功能");
    wx.showModal({ title: `${feature}暂未开放`, content: "当前优先跑通点单、支付确认和履约闭环，该能力已预留入口。", showCancel: false });
  },
  contact() { wx.showModal({ title: "联系商家", content: "请在门店首页查看商家联系方式。", showCancel: false }); },
});
