import type { TanbanAppOption } from "../../app";
import type { Store } from "../../types/domain";
import { request } from "../../utils/request";
import { routedStoreCode } from "../../utils/store-route";

Page({
  data: {
    loading: true,
    store: null as Store | null,
    error: "",
  },
  onLoad(options: Record<string, string>) {
    const app = getApp<TanbanAppOption>();
    const routedCode = routedStoreCode(options);
    if (routedCode) app.globalData.storeCode = routedCode;
    const storeCode = app.globalData.storeCode;
    this.loadStore(storeCode);
  },
  onPullDownRefresh() {
    this.loadStore(getApp<TanbanAppOption>().globalData.storeCode).finally(() => wx.stopPullDownRefresh());
  },
  async loadStore(storeCode: string) {
    this.setData({ loading: true, error: "" });
    try {
      const store = await request<Store>({ url: `/public/stores/${encodeURIComponent(storeCode)}`, method: "GET" });
      this.setData({ store, loading: false });
      wx.setNavigationBarTitle({ title: store.name || "摊伴点单" });
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : "门店加载失败" });
    }
  },
  goMenu() { wx.switchTab({ url: "/pages/menu/index" }); },
  goOrders() { wx.switchTab({ url: "/pages/orders/index" }); },
});
