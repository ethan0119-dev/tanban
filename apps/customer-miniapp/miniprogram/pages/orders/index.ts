import type { TanbanAppOption } from "../../app";
import type { Order } from "../../types/domain";
import { localOrderNumbers } from "../../utils/orders";
import { request } from "../../utils/request";

Page({
  data: { orders: [] as Order[], loading: true },
  onShow() { this.loadOrders(); },
  onPullDownRefresh() { this.loadOrders().finally(() => wx.stopPullDownRefresh()); },
  async loadOrders() {
    try {
      const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
      const numbers = localOrderNumbers(storeCode);
      const results = await Promise.allSettled(numbers.map((orderNo) => request<Order>({ url: `/public/orders/${encodeURIComponent(orderNo)}`, method: "GET" })));
      const orders = results.flatMap((result) => result.status === "fulfilled" ? [result.value] : []);
      this.setData({ orders, loading: false });
    }
    catch { this.setData({ orders: [], loading: false }); }
  },
  openOrder(event: WechatMiniprogram.BaseEvent) { wx.navigateTo({ url: `/pages/order-detail/index?orderNo=${encodeURIComponent(String(event.currentTarget.dataset.no))}` }); },
});
