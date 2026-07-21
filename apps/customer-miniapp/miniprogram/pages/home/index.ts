import type { TanbanAppOption } from "../../app";
import type { DecorationAction, DecorationConfig, DecorationModule, FastFoodOrderingContext, MarketingPlacement, Store, TableOrderingContext } from "../../types/domain";
import {
  applyDecorationChrome,
  decorationStyle,
  defaultDecoration,
  normalizeDecoration,
  rememberSplash,
  runDecorationAction,
  shouldDisplaySplash,
} from "../../utils/decoration";
import { customerGuestKey } from "../../utils/customer";
import { idempotencyKey, request } from "../../utils/request";
import { marketingEventKey, rememberMarketingPopup, shouldDisplayMarketingPopup } from "../../utils/marketing";
import { tableContextForStore } from "../../utils/table-context";
import { fastFoodContextForStore } from "../../utils/fast-food-context";
import { rememberPageAppearance } from "../../utils/page-appearance";
import { customerExperienceCopy, customerSafeErrorMessage } from "../../utils/availability";

let splashTimer: ReturnType<typeof setTimeout> | undefined;

Page({
  data: {
    loading: true,
    store: null as Store | null,
    decoration: defaultDecoration() as DecorationConfig,
    modules: [] as DecorationModule[],
    appearanceStyle: "",
    tableContext: null as TableOrderingContext | null,
    fastFoodContext: null as FastFoodOrderingContext | null,
    splashVisible: false,
    marketingPopup: null as MarketingPlacement | null,
    marketingPopupVisible: false,
    error: "",
  },
  onShow() {
    void this.syncOrderingRoute(!this.data.store);
  },
  async syncOrderingRoute(forceReload: boolean) {
    const app = getApp<TanbanAppOption>();
    await app.globalData.routeReady;
    if (app.globalData.routeError) {
      this.setData({ loading: false, store: null, tableContext: null, fastFoodContext: null, error: app.globalData.routeError });
      return;
    }
    const storeCode = app.globalData.storeCode;
    const tableContext = tableContextForStore(storeCode);
    const fastFoodContext = fastFoodContextForStore(storeCode);
    this.setData({ tableContext, fastFoodContext });
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
      rememberPageAppearance(store);
      const modules = decoration.home.modules.filter((module) => module.enabled);
      this.setData({ store, decoration, modules, appearanceStyle: decorationStyle(decoration), loading: false, marketingPopup: null, marketingPopupVisible: false });
      wx.setNavigationBarTitle({ title: store.name || "摊伴点单" });
      applyDecorationChrome(decoration);
      const marketingShown = await this.loadMarketingPopup(storeCode);
      if (!marketingShown) this.showSplashIfNeeded(storeCode, store.decorationVersion || 0, decoration);
    } catch (error) {
      this.setData({ loading: false, error: customerSafeErrorMessage(error, "门店信息暂时无法加载，请稍后重试。") });
    }
  },
  async loadMarketingPopup(storeCode: string): Promise<boolean> {
    try {
      const channelScope = this.data.tableContext ? "DINE_IN" : "TAKEOUT";
      const placement = await request<MarketingPlacement | null>({ url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/popup?placementCode=HOME_POPUP&channelScope=${channelScope}`, method: "GET" });
      if (!placement || !shouldDisplayMarketingPopup(storeCode, placement)) return false;
      rememberMarketingPopup(storeCode, placement);
      this.setData({ marketingPopup: placement, marketingPopupVisible: true, splashVisible: false });
      void this.recordMarketingEvent("IMPRESSION");
      return true;
    } catch {
      // 营销位故障不影响门店首页和下单主链路。
      return false;
    }
  },
  async recordMarketingEvent(eventType: "IMPRESSION" | "CLICK" | "CLOSE") {
    const placement = this.data.marketingPopup;
    if (!placement) return;
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      await request({
        url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/events`, method: "POST",
        header: { "Idempotency-Key": marketingEventKey(placement.id, eventType) },
        data: { placement_id: placement.id, event_type: eventType, subject_key: customerGuestKey() },
      });
    } catch {
      // 曝光统计采用尽力而为策略，不阻塞顾客交互。
    }
  },
  closeMarketingPopup() {
    const placement = this.data.marketingPopup;
    if (!placement || !this.data.marketingPopupVisible) return;
    rememberMarketingPopup(getApp<TanbanAppOption>().globalData.storeCode, placement);
    this.setData({ marketingPopupVisible: false });
    void this.recordMarketingEvent("CLOSE");
  },
  async onMarketingAction() {
    const placement = this.data.marketingPopup;
    if (!placement || !this.data.marketingPopupVisible) return;
    void this.recordMarketingEvent("CLICK");
    rememberMarketingPopup(getApp<TanbanAppOption>().globalData.storeCode, placement);
    this.setData({ marketingPopupVisible: false });
    if (placement.action_type === "OPEN_MENU") return wx.switchTab({ url: "/pages/menu/index" });
    if (placement.action_type === "OPEN_COUPONS") return wx.navigateTo({ url: "/pages/coupons/index" });
    if (placement.action_type === "OPEN_LOTTERY") return wx.navigateTo({ url: `/pages/lottery/index?id=${placement.action_target_id || ""}` });
    if (placement.action_type === "CLAIM_COUPON" && placement.action_target_id) {
      try {
        const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
        const result = await request<{ warning?: string }>({
          url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/coupons/${placement.action_target_id}/claim`, method: "POST",
          header: { "Idempotency-Key": idempotencyKey("popup_coupon") },
          data: { subject_key: customerGuestKey() },
        });
        void result;
        wx.showModal({ title: "领取结果", content: customerExperienceCopy.couponClaimed, showCancel: false });
      } catch (error) {
        wx.showToast({ title: customerSafeErrorMessage(error, "暂时无法领取，请稍后重试。"), icon: "none" });
      }
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
