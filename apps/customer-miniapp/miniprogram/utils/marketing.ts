import type { MarketingPlacement } from "../types/domain";

const PREFIX = "tanban_marketing_popup_v1";

function localDay(now = new Date()): string {
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${now.getFullYear()}-${month}-${day}`;
}

function storageKey(storeCode: string, placement: MarketingPlacement): string {
  return `${PREFIX}:${storeCode}:${placement.id}`;
}

export function shouldDisplayMarketingPopup(storeCode: string, placement: MarketingPlacement, now = new Date()): boolean {
  if (!hasMarketingPopupVisual(placement)) return false;
  if (placement.frequency === "EVERY_VISIT") return true;
  const remembered = wx.getStorageSync<string>(storageKey(storeCode, placement));
  if (placement.frequency === "DAILY") return remembered !== localDay(now);
  return remembered !== "SEEN";
}

export function hasMarketingPopupVisual(placement: MarketingPlacement): boolean {
  return Boolean(placement.image_url?.trim() || placement.title?.trim() || placement.subtitle?.trim());
}

export function rememberMarketingPopup(storeCode: string, placement: MarketingPlacement, now = new Date()): void {
  wx.setStorageSync(storageKey(storeCode, placement), placement.frequency === "DAILY" ? localDay(now) : "SEEN");
}

export function marketingEventKey(placementID: number, eventType: string): string {
  return `marketing_${placementID}_${eventType.toLowerCase()}_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
}
