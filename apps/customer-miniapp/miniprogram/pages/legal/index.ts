import { loadPageAppearance } from "../../utils/page-appearance";

Page({
  data: { loading: true, storeName: "", activeTab: "privacy", privacyPolicy: "", userAgreement: "", appearanceStyle: "" },
  async onLoad() {
    try {
      const appearance = await loadPageAppearance();
      const store = appearance.store;
      this.setData({ appearanceStyle: appearance.appearanceStyle, storeName: store.name, privacyPolicy: store.legal?.privacyPolicy || "商户暂未配置隐私政策。", userAgreement: store.legal?.userAgreement || "商户暂未配置用户协议。" });
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
