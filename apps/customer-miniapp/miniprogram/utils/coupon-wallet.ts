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
