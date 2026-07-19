import type { CartItem } from "../types/domain";
import { ApiError, idempotencyKey } from "./request";

const CHECKOUT_FLOW_KEY = "tanban_checkout_flow_v1";
export const CHECKOUT_FLOW_TTL_MS = 30 * 60 * 1000;

export interface CheckoutFlow {
  storeCode: string;
  cartFingerprint: string;
  idempotencyKey: string;
  orderNo: string;
  createdAt: number;
}

export function checkoutNeedsFreshOrder(error: unknown): boolean {
  return error instanceof ApiError
    && (error.code === "ORDER_NOT_PAYABLE" || error.code === "PAYMENT_CLOSED");
}

export function checkoutOrderIsClosed(status: string | null | undefined): boolean {
  return String(status ?? '').toUpperCase() === 'CLOSED';
}

function fingerprintCart(cart: CartItem[]): string {
  return cart
    .map((item) => `${item.productId}:${item.skuId || 0}:${item.price}:${item.quantity}`)
    .sort()
    .join("|");
}

function readCheckoutFlow(now = Date.now()): CheckoutFlow | null {
  const stored = wx.getStorageSync<CheckoutFlow>(CHECKOUT_FLOW_KEY);
  if (!stored || typeof stored !== "object") return null;
  if (!stored.storeCode || !stored.cartFingerprint || !stored.idempotencyKey || typeof stored.createdAt !== "number") {
    wx.removeStorageSync(CHECKOUT_FLOW_KEY);
    return null;
  }
  if (now - stored.createdAt >= CHECKOUT_FLOW_TTL_MS || stored.createdAt > now + 60_000) {
    wx.removeStorageSync(CHECKOUT_FLOW_KEY);
    return null;
  }
  return { ...stored, orderNo: stored.orderNo || "" };
}

function writeCheckoutFlow(flow: CheckoutFlow): void {
  wx.setStorageSync(CHECKOUT_FLOW_KEY, flow);
}

/** Reuses one key while the same cart remains in the current checkout flow. */
export function checkoutFlowFor(storeCode: string, cart: CartItem[]): CheckoutFlow {
  const cartFingerprint = fingerprintCart(cart);
  const current = readCheckoutFlow();
  if (current && current.storeCode === storeCode && current.cartFingerprint === cartFingerprint) return current;

  const flow: CheckoutFlow = {
    storeCode,
    cartFingerprint,
    idempotencyKey: idempotencyKey("order"),
    orderNo: "",
    createdAt: Date.now(),
  };
  writeCheckoutFlow(flow);
  return flow;
}

export function rememberCheckoutOrder(flowKey: string, orderNo: string): void {
  const flow = readCheckoutFlow();
  if (!flow || flow.idempotencyKey !== flowKey) return;
  writeCheckoutFlow({ ...flow, orderNo });
}

export function clearCheckoutFlow(flowKey: string): void {
  const flow = readCheckoutFlow();
  if (!flow || flow.idempotencyKey !== flowKey) return;
  wx.removeStorageSync(CHECKOUT_FLOW_KEY);
}
