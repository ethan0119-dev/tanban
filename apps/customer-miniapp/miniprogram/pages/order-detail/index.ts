import type { Order } from "../../types/domain";
import { request } from "../../utils/request";

Page({
  data: { order: null as Order | null, loading: true, orderNo: "" },
  onLoad(options: Record<string, string>) { this.setData({ orderNo: options.orderNo || "" }); this.loadOrder(); },
  onShow() { if (this.data.orderNo) this.loadOrder(); },
  async loadOrder() {
    if (!this.data.orderNo) return;
    try { this.setData({ order: await request<Order>({ url: `/public/orders/${encodeURIComponent(this.data.orderNo)}`, method: "GET" }), loading: false }); }
    catch (error) { this.setData({ loading: false }); wx.showToast({ title: error instanceof Error ? error.message : "加载失败", icon: "none" }); }
  },
  backToMenu() { wx.switchTab({ url: "/pages/menu/index" }); },
});
