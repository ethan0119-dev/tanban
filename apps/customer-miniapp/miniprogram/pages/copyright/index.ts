import type { TanbanAppOption } from "../../app";
import type { Store } from "../../types/domain";
import { request } from "../../utils/request";
import { rememberPageAppearance } from "../../utils/page-appearance";

Page({
  data: {
    store: null as Store | null,
    appearanceStyle: "",
    loading: true,
    webUrl: "",
  },
  async onLoad(options: Record<string, string | undefined>) {
    wx.setNavigationBarTitle({ title: "版权说明" });
    const storeCode = String(options.storeCode || getApp<TanbanAppOption>().globalData.storeCode || "");
    try {
      const store = await request<Store>({ url: `/public/stores/${encodeURIComponent(storeCode)}`, method: "GET" });
      const branding = store.platformBranding || {};
      const baseURL = String(branding.marketingPageUrl || "").trim();
      const query = [
        ["brand", branding.platformName || "摊伴餐饮系统"],
        ["title", branding.marketingTitle || ""],
        ["subtitle", branding.marketingSubtitle || ""],
        ["wechat", branding.contactWechat || ""],
        ["qr", branding.contactQrUrl || ""],
      ].filter(([, value]) => value).map(([key, value]) => `${key}=${encodeURIComponent(value)}`).join("&");
      this.setData({ store, webUrl: baseURL ? `${baseURL}${baseURL.includes("?") ? "&" : "?"}${query}` : "", appearanceStyle: rememberPageAppearance(store).appearanceStyle, loading: false });
    } catch {
      this.setData({ loading: false });
    }
  },
  previewQR() {
    const url = this.data.store?.platformBranding?.contactQrUrl || "";
    if (!url) return wx.showToast({ title: "平台暂未配置联系二维码", icon: "none" });
    wx.previewImage({ current: url, urls: [url] });
  },
  copyWechat() {
    const wechat = this.data.store?.platformBranding?.contactWechat || "";
    if (!wechat) return;
    wx.setClipboardData({ data: wechat });
  },
});
