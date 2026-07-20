import type { TanbanAppOption } from "../../app";
import type { Store } from "../../types/domain";
import { request } from "../../utils/request";

Page({
  data: { loading: true, storeName: "", activeTab: "privacy", privacyPolicy: "", userAgreement: "" },
  async onLoad() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      const store = await request<Store>({ url: `/public/stores/${encodeURIComponent(storeCode)}`, method: "GET" });
      this.setData({ storeName: store.name, privacyPolicy: store.legal?.privacyPolicy || "商户暂未配置隐私政策。", userAgreement: store.legal?.userAgreement || "商户暂未配置用户协议。" });
    } catch (error) {
      wx.showToast({ title: error instanceof Error ? error.message : "加载失败", icon: "none" });
    } finally {
      this.setData({ loading: false });
    }
  },
  selectTab(event: WechatMiniprogram.BaseEvent) {
    this.setData({ activeTab: String(event.currentTarget.dataset.tab || "privacy") });
  },
});
