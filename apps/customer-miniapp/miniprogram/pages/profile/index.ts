import type { TanbanAppOption } from "../../app";
import { tableContextForStore } from "../../utils/table-context";

Page({
  data: { version: "v0.1.0", storeCode: "", channelScope: "TAKEOUT" },
  onShow() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setData({ storeCode, channelScope: tableContextForStore(storeCode) ? "DINE_IN" : "TAKEOUT" });
  },
  goOrders() { wx.switchTab({ url: "/pages/orders/index" }); },
  goCoupons() { wx.navigateTo({ url: "/pages/coupons/index" }); },
  goLottery() { wx.navigateTo({ url: "/pages/lottery/index" }); },
  contact() { wx.showModal({ title: "联系商家", content: "请在门店首页查看商家联系方式。", showCancel: false }); },
});
