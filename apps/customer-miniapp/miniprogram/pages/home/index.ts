import type { TanbanAppOption } from "../../app";
import type { DecorationAction, DecorationConfig, DecorationModule, Store, TableOrderingContext } from "../../types/domain";
import {
  applyDecorationChrome,
  decorationStyle,
  defaultDecoration,
  normalizeDecoration,
  rememberSplash,
  runDecorationAction,
  shouldDisplaySplash,
} from "../../utils/decoration";
import { request } from "../../utils/request";
import { tableContextForStore } from "../../utils/table-context";

let splashTimer: ReturnType<typeof setTimeout> | undefined;

Page({
  data: {
    loading: true,
    store: null as Store | null,
    decoration: defaultDecoration() as DecorationConfig,
    modules: [] as DecorationModule[],
    appearanceStyle: "",
    tableContext: null as TableOrderingContext | null,
    splashVisible: false,
    error: "",
  },
  onShow() {
    void this.syncOrderingRoute(!this.data.store);
  },
  async syncOrderingRoute(forceReload: boolean) {
    const app = getApp<TanbanAppOption>();
    await app.globalData.routeReady;
    if (app.globalData.routeError) {
      this.setData({ loading: false, store: null, tableContext: null, error: app.globalData.routeError });
      return;
    }
    const storeCode = app.globalData.storeCode;
    const tableContext = tableContextForStore(storeCode);
    this.setData({ tableContext });
    if (forceReload || !this.data.store || this.data.store.code !== storeCode) await this.loadStore(storeCode);
  },
  onUnload() {
    if (splashTimer) clearTimeout(splashTimer);
  },
  onPullDownRefresh() {
    this.syncOrderingRoute(true).finally(() => wx.stopPullDownRefresh());
  },
  async loadStore(storeCode: string) {
    this.setData({ loading: true, error: "" });
    try {
      const store = await request<Store>({ url: `/public/stores/${encodeURIComponent(storeCode)}`, method: "GET" });
      const decoration = normalizeDecoration(store.decoration, store);
      const modules = decoration.home.modules.filter((module) => module.enabled);
      this.setData({ store, decoration, modules, appearanceStyle: decorationStyle(decoration), loading: false });
      wx.setNavigationBarTitle({ title: store.name || "摊伴点单" });
      applyDecorationChrome(decoration);
      this.showSplashIfNeeded(storeCode, store.decorationVersion || 0, decoration);
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : "门店加载失败" });
    }
  },
  showSplashIfNeeded(storeCode: string, version: number, decoration: DecorationConfig) {
    if (!shouldDisplaySplash(decoration, storeCode, version)) return;
    this.setData({ splashVisible: true });
    if (splashTimer) clearTimeout(splashTimer);
    if (decoration.splash.autoCloseSeconds > 0) {
      splashTimer = setTimeout(() => this.closeSplash(), decoration.splash.autoCloseSeconds * 1000);
    }
  },
  closeSplash() {
    if (!this.data.splashVisible) return;
    if (splashTimer) clearTimeout(splashTimer);
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    rememberSplash(storeCode, this.data.store?.decorationVersion || 0);
    this.setData({ splashVisible: false });
  },
  onDecorationAction(event: WechatMiniprogram.BaseEvent) {
    const type = String(event.currentTarget.dataset.actionType || "NONE") as DecorationAction["type"];
    const phone = String(event.currentTarget.dataset.actionPhone || "");
    runDecorationAction({ type, ...(phone ? { phone } : {}) });
  },
  onSplashAction() {
    const action = this.data.decoration.splash.action;
    this.closeSplash();
    runDecorationAction(action);
  },
  goMenu() { wx.switchTab({ url: "/pages/menu/index" }); },
  goOrders() { wx.switchTab({ url: "/pages/orders/index" }); },
  noop() {},
});
