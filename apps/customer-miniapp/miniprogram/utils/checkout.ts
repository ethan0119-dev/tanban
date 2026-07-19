import type { CartItem } from "../types/domain";
import { cartLineKey } from "./cart";
import { ApiError, idempotencyKey } from "./request";

const CHECKOUT_FLOW_KEY = "tanban_checkout_flow_v1";
export const CHECKOUT_FLOW_TTL_MS = 30 * 60 * 1000;

export interface CheckoutFlow {
  storeCode: string;
  cartFingerprint: string;
  idempotencyKey: string;
  orderNo: string;
  fulfillmentType: "PICKUP" | "DINE_IN";
  remark: string;
  submitted: boolean;
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
    .map((item) => `${cartLineKey(item)}:p=${item.price}:q=${item.quantity}`)
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
  return {
    ...stored,
    orderNo: stored.orderNo || "",
    fulfillmentType: stored.fulfillmentType === "DINE_IN" ? "DINE_IN" : "PICKUP",
    remark: typeof stored.remark === "string" ? stored.remark : "",
    submitted: Boolean(stored.submitted || stored.orderNo),
  };
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
    fulfillmentType: "PICKUP",
    remark: "",
    submitted: false,
    createdAt: Date.now(),
  };
  writeCheckoutFlow(flow);
  return flow;
}

export function rememberCheckoutOrder(flowKey: string, orderNo: string): void {
  const flow = readCheckoutFlow();
  if (!flow || flow.idempotencyKey !== flowKey) return;
  writeCheckoutFlow({ ...flow, orderNo, submitted: true });
}

export function rememberCheckoutDetails(flowKey: string, fulfillmentType: "PICKUP" | "DINE_IN", remark: string): void {
  const flow = readCheckoutFlow();
  if (!flow || flow.idempotencyKey !== flowKey || flow.submitted) return;
  writeCheckoutFlow({ ...flow, fulfillmentType, remark });
}

export function markCheckoutSubmitted(flowKey: string): void {
  const flow = readCheckoutFlow();
  if (!flow || flow.idempotencyKey !== flowKey) return;
  writeCheckoutFlow({ ...flow, submitted: true });
}

export function clearCheckoutFlow(flowKey: string): void {
  const flow = readCheckoutFlow();
  if (!flow || flow.idempotencyKey !== flowKey) return;
  wx.removeStorageSync(CHECKOUT_FLOW_KEY);
}
