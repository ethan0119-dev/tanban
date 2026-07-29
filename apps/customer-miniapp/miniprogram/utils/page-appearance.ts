import type { TanbanAppOption } from "../app";
import type { DecorationConfig, Store } from "../types/domain";
import { applyDecorationChrome, decorationStyle, normalizeDecoration } from "./decoration";
import { request } from "./request";

export interface PageAppearance {
  store: Store;
  decoration: DecorationConfig;
  appearanceStyle: string;
}

export function rememberPageAppearance(store: Store): PageAppearance {
  const app = getApp<TanbanAppOption>();
  const decoration = normalizeDecoration(store.decoration, store);
  const appearanceStyle = decorationStyle(decoration);
  app.globalData.appearanceStoreCode = store.code;
  app.globalData.appearanceStore = store;
  app.globalData.appearanceDecoration = decoration;
  app.globalData.appearanceStyle = appearanceStyle;
  applyDecorationChrome(decoration);
  return { store, decoration, appearanceStyle };
}

export async function loadPageAppearance(): Promise<PageAppearance> {
  const app = getApp<TanbanAppOption>();
  await app.globalData.routeReady;
  const storeCode = app.globalData.storeCode;
  if (app.globalData.appearanceStoreCode === storeCode && app.globalData.appearanceStore && app.globalData.appearanceDecoration) {
    applyDecorationChrome(app.globalData.appearanceDecoration);
    return {
      store: app.globalData.appearanceStore,
      decoration: app.globalData.appearanceDecoration,
      appearanceStyle: app.globalData.appearanceStyle,
    };
  }
  const store = await request<Store>({ url: `/public/stores/${encodeURIComponent(storeCode)}`, method: "GET" });
  return rememberPageAppearance(store);
}
