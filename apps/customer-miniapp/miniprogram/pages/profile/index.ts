import type { TanbanAppOption } from "../../app";
import type { Store } from "../../types/domain";
import { localCouponCount } from "../../utils/coupon-wallet";
import { tableContextForStore } from "../../utils/table-context";
import { loadPageAppearance } from "../../utils/page-appearance";
import { showUnavailableFeature } from "../../utils/availability";

Page({
  data: { version: "v0.2.2", storeCode: "", store: null as Store | null, channelScope: "TAKEOUT", couponCount: 0, appearanceStyle: "" },
  async onShow() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setData({ storeCode, couponCount: localCouponCount(storeCode), channelScope: tableContextForStore(storeCode) ? "DINE_IN" : "TAKEOUT" });
    try {
      const appearance = await loadPageAppearance();
      this.setData({ store: appearance.store, appearanceStyle: appearance.appearanceStyle });
    } catch {
      // 联系方式仍以服务端下单页最终校验为准，个人中心加载失败不阻断其他入口。
    }
  },
  goOrders() { wx.switchTab({ url: "/pages/orders/index" }); },
  goCoupons() { wx.navigateTo({ url: "/pages/my-coupons/index" }); },
  goCouponCenter() { wx.navigateTo({ url: "/pages/coupons/index" }); },
  goRecharge() { wx.navigateTo({ url: "/pages/recharge/index" }); },
  goLottery() { wx.navigateTo({ url: "/pages/lottery/index" }); },
  goLegal() { wx.navigateTo({ url: "/pages/legal/index" }); },
  unavailable(event: WechatMiniprogram.BaseEvent) {
    const feature = String(event.currentTarget.dataset.feature || "该功能");
    showUnavailableFeature("PROFILE_SERVICE", feature);
  },
  contact() {
    const service = this.data.store?.customerService;
    const options: Array<{ label: string; action: () => void }> = [];
    if (service?.phone) options.push({ label: `拨打 ${service.phone}`, action: () => wx.makePhoneCall({ phoneNumber: service.phone! }) });
    if (service?.qrUrl) options.push({ label: "查看客服二维码", action: () => wx.previewImage({ urls: [service.qrUrl!], current: service.qrUrl }) });
    if (service?.wechat) options.push({ label: `客服微信：${service.wechat}`, action: () => wx.setClipboardData({ data: service.wechat!, success: () => wx.showToast({ title: "微信号已复制" }) }) });
    if (!options.length) {
      wx.showModal({ title: "联系商家", content: "暂时无法联系门店，请稍后再试。", showCancel: false });
      return;
    }
    wx.showActionSheet({ itemList: options.map((item) => item.label), success: (result) => options[result.tapIndex]?.action() });
  },
});
