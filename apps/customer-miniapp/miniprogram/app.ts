import { env } from "./config/env";
import type { TableOrderingContext } from "./types/domain";
import { orderingEntryKey, parseOrderingEntry, type OrderingEntryOptions } from "./utils/store-route";
import { clearTableOrderingContext, resolveTableOrderingContext, saveTableOrderingContext } from "./utils/table-context";

export interface TanbanAppOption {
  globalData: {
    storeCode: string;
    customerToken: string;
    tableContext: TableOrderingContext | null;
    routeReady: Promise<void>;
    routeError: string;
    routeRevision: number;
    lastEntryKey: string;
    lastEntryAt: number;
  };
  prepareOrderingEntry(options: OrderingEntryOptions, restoreWhenEmpty?: boolean): Promise<void>;
}

App<TanbanAppOption>({
  globalData: {
    storeCode: env.defaultStoreCode,
    customerToken: "",
    tableContext: null,
    routeReady: Promise.resolve(),
    routeError: "",
    routeRevision: 0,
    lastEntryKey: "",
    lastEntryAt: 0,
  },
  onLaunch(options) {
    const token = wx.getStorageSync<string>("tanban_customer_token");
    if (token) this.globalData.customerToken = token;
    this.globalData.routeReady = this.prepareOrderingEntry(options, true);
  },
  onShow(options) {
    const route = parseOrderingEntry(options);
    if (route.kind !== "NONE") this.globalData.routeReady = this.prepareOrderingEntry(options, false);
  },
  prepareOrderingEntry(options, restoreWhenEmpty = true) {
    const route = parseOrderingEntry(options);
    if (route.kind === "NONE") {
      if (restoreWhenEmpty) {
        // A cold start from Recent Mini Programs must never silently keep the
        // previous venue's table. In-process tab/background navigation does not
        // call this branch, so a valid scanned context remains stable there.
        clearTableOrderingContext();
        this.globalData.tableContext = null;
        this.globalData.storeCode = env.defaultStoreCode;
        this.globalData.routeError = "";
      }
      return Promise.resolve();
    }

    const key = orderingEntryKey(route);
    const now = Date.now();
    if (key === this.globalData.lastEntryKey && now - this.globalData.lastEntryAt < 1_500) {
      return this.globalData.routeReady;
    }
    this.globalData.lastEntryKey = key;
    this.globalData.lastEntryAt = now;
    const revision = ++this.globalData.routeRevision;

    if (route.kind === "STORE") {
      clearTableOrderingContext();
      this.globalData.tableContext = null;
      this.globalData.storeCode = route.storeCode;
      this.globalData.routeError = "";
      return Promise.resolve();
    }

    clearTableOrderingContext();
    this.globalData.tableContext = null;
    this.globalData.storeCode = env.defaultStoreCode;
    if (route.kind === "INVALID") {
      this.globalData.routeError = route.message;
      wx.showModal({ title: "无法识别二维码", content: route.message, showCancel: false });
      return Promise.resolve();
    }

    this.globalData.routeError = "";
    return resolveTableOrderingContext(route.publicScene, route.expectedStoreCode)
      .then((context) => {
        if (revision !== this.globalData.routeRevision) return;
        saveTableOrderingContext(context);
        this.globalData.tableContext = context;
        this.globalData.storeCode = context.storeCode;
      })
      .catch((error: unknown) => {
        if (revision !== this.globalData.routeRevision) return;
        clearTableOrderingContext();
        this.globalData.tableContext = null;
        this.globalData.storeCode = env.defaultStoreCode;
        this.globalData.routeError = error instanceof Error ? error.message : "桌码识别失败，请重新扫码";
        wx.showModal({ title: "桌码不可用", content: this.globalData.routeError, showCancel: false });
      });
  },
});
