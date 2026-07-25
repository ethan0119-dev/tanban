import type { TanbanAppOption } from "../../app";
import type { Store } from "../../types/domain";
import { localCouponCount } from "../../utils/coupon-wallet";
import { tableContextForStore } from "../../utils/table-context";
import { loadPageAppearance } from "../../utils/page-appearance";
import { showUnavailableFeature } from "../../utils/availability";
import { request } from "../../utils/request";

interface PublicMembership {
  available: boolean;
  card?: { name: string; color: string; imageUrl?: string; agreementUrl?: string; showBalance: boolean };
  member?: { memberId: number; memberNo: string; levelId: number; levelName: string; principalCents: number; bonusCents: number; balanceCents: number };
  levels: Array<{ id: number; name: string; acquireType: string; rechargeCents: number; validDays: number; discountPercent: number }>;
}

Page({
  data: { version: "v0.3.1", storeCode: "", store: null as Store | null, membership: null as PublicMembership | null, channelScope: "TAKEOUT", couponCount: 0, appearanceStyle: "" },
  async onShow() {
    try {
      const appearance = await loadPageAppearance();
      const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
      this.setData({ storeCode, couponCount: localCouponCount(storeCode), channelScope: tableContextForStore(storeCode) ? "DINE_IN" : "TAKEOUT" });
      this.setData({ store: appearance.store, appearanceStyle: appearance.appearanceStyle });
      const membership = await request<PublicMembership>({
        url: `/public/stores/${encodeURIComponent(storeCode)}/membership`,
        method: "GET",
      });
      this.setData({ membership });
    } catch {
      // 联系方式仍以服务端下单页最终校验为准，个人中心加载失败不阻断其他入口。
    }
  },
  goOrders() { wx.switchTab({ url: "/pages/orders/index" }); },
  goCoupons() { wx.navigateTo({ url: "/pages/my-coupons/index" }); },
  goCouponCenter() { wx.navigateTo({ url: "/pages/coupons/index" }); },
  goRecharge() { wx.navigateTo({ url: "/pages/recharge/index" }); },
  goLottery() { wx.navigateTo({ url: "/pages/lottery/index" }); },
  showMembership() {
    const membership = this.data.membership;
    if (!membership?.available) {
      wx.showModal({ title: "会员服务", content: "当前门店暂未开启会员卡。", showCancel: false });
      return;
    }
    wx.navigateTo({ url: "/pages/membership/index" });
  },
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
