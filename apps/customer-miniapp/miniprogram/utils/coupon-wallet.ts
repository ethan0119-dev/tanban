import type { MarketingCoupon } from "../types/domain";
import { beijingNowDateTime } from "./datetime";

const WALLET_KEY = "tanban_coupon_wallet_v1";

export interface LocalCouponAsset extends MarketingCoupon {
  claimedAt: string;
  assetStatus: "PROVISIONAL";
}

type CouponWallet = Record<string, LocalCouponAsset[]>;

function readWallet(): CouponWallet {
  const value = wx.getStorageSync<CouponWallet>(WALLET_KEY);
  return value && typeof value === "object" ? value : {};
}

export function rememberClaimedCoupon(storeCode: string, coupon: MarketingCoupon): void {
  const wallet = readWallet();
  const current = wallet[storeCode] || [];
  wallet[storeCode] = [
    { ...coupon, claimedAt: beijingNowDateTime(), assetStatus: "PROVISIONAL" as const },
    ...current.filter((item) => item.id !== coupon.id),
  ].slice(0, 100);
  wx.setStorageSync(WALLET_KEY, wallet);
}

export function localCouponAssets(storeCode: string): LocalCouponAsset[] {
  return readWallet()[storeCode] || [];
}

export function localCouponCount(storeCode: string): number {
  return localCouponAssets(storeCode).length;
}

export function forgetClaimedCoupon(storeCode: string, campaignID: number): void {
  const wallet = readWallet();
  wallet[storeCode] = (wallet[storeCode] || []).filter((item) => item.id !== campaignID);
  wx.setStorageSync(WALLET_KEY, wallet);
}

function couponValidAt(coupon: LocalCouponAsset, now: number): boolean {
  const fixedFrom = coupon.valid_from ? Date.parse(coupon.valid_from.replace(" ", "T") + "+08:00") : Number.NaN;
  const fixedTo = coupon.valid_to ? Date.parse(coupon.valid_to.replace(" ", "T") + "+08:00") : Number.NaN;
  if (!Number.isNaN(fixedFrom) && now < fixedFrom) return false;
  if (!Number.isNaN(fixedTo) && now >= fixedTo) return false;
  if (Number.isNaN(fixedTo) && coupon.valid_days) {
    const claimedAt = Date.parse(coupon.claimedAt.replace(" ", "T") + "+08:00");
    if (!Number.isNaN(claimedAt) && now >= claimedAt + coupon.valid_days * 86400000) return false;
  }
  return true;
}

export function bestEligibleCoupon(
  storeCode: string,
  subtotalCents: number,
  orderType: "DINE_IN" | "TAKEOUT",
  now = Date.now(),
): LocalCouponAsset | null {
  return localCouponAssets(storeCode)
    .filter((coupon) =>
      coupon.status === "ACTIVE" &&
      coupon.discount_cents > 0 &&
      subtotalCents >= coupon.threshold_cents &&
      (!coupon.order_types?.length || coupon.order_types.includes(orderType)) &&
      couponValidAt(coupon, now))
    .sort((a, b) => b.discount_cents - a.discount_cents || a.threshold_cents - b.threshold_cents || a.id - b.id)[0] || null;
}
