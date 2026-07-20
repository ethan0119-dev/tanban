import type { CartItem, FastFoodOrderingContext, TableOrderingContext } from "../types/domain";
import { cartLineKey } from "./cart";
import { ApiError, idempotencyKey } from "./request";

const CHECKOUT_FLOW_KEY = "tanban_checkout_flow_v1";
export const CHECKOUT_FLOW_TTL_MS = 30 * 60 * 1000;

export interface CheckoutFlow {
  storeCode: string;
  cartFingerprint: string;
  orderingContextKey: string;
  idempotencyKey: string;
  orderNo: string;
  fulfillmentType: "PICKUP" | "DINE_IN";
  remark: string;
  submitted: boolean;
  orderScene?: "DINE_IN";
  tablePublicId?: string;
  tablePublicScene?: string;
  tableName?: string;
  fastFoodPlatePublicId?: string;
  fastFoodPlateName?: string;
  createdAt: number;
}

export function checkoutNeedsFreshOrder(error: unknown): boolean {
  return error instanceof ApiError
    && (error.code === "ORDER_NOT_PAYABLE" || error.code === "PAYMENT_CLOSED");
}

export function checkoutOrderIsClosed(status: string | null | undefined): boolean {
  return String(status ?? '').toUpperCase() === 'CLOSED';
}

/** Closing the store blocks new orders, but never strands an order that was
 * already created while the store was accepting orders. Its payment endpoint
 * remains the source of truth for whether that order is still payable. */
export function checkoutBlockedByStoreStatus(businessStatus: string | null | undefined, orderNo: string | null | undefined): boolean {
  return String(businessStatus ?? '').toUpperCase() !== 'OPEN' && !String(orderNo ?? '').trim();
}

function fingerprintCart(cart: CartItem[]): string {
  return cart
    .map((item) => `${cartLineKey(item)}:p=${item.price}:q=${item.quantity}`)
    .sort()
    .join("|");
}

function orderingContextKey(context: TableOrderingContext | null | undefined, fastFoodContext?: FastFoodOrderingContext | null): string {
  if (context) return `TABLE:${context.publicScene}:${context.tablePublicId}`;
  if (fastFoodContext) return `FAST_FOOD:${fastFoodContext.publicId}`;
  return "STORE";
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
    orderingContextKey: stored.orderingContextKey || "STORE",
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
export function checkoutFlowFor(
  storeCode: string,
  cart: CartItem[],
  tableContext?: TableOrderingContext | null,
  fastFoodContext?: FastFoodOrderingContext | null,
): CheckoutFlow {
  const cartFingerprint = fingerprintCart(cart);
  const contextKey = orderingContextKey(tableContext, fastFoodContext);
  const current = readCheckoutFlow();
  if (
    current
    && current.storeCode === storeCode
    && current.cartFingerprint === cartFingerprint
    && current.orderingContextKey === contextKey
    && (Boolean(tableContext) || current.fulfillmentType !== "DINE_IN")
  ) return current;

  const flow: CheckoutFlow = {
    storeCode,
    cartFingerprint,
    orderingContextKey: contextKey,
    idempotencyKey: idempotencyKey("order"),
    orderNo: "",
    fulfillmentType: tableContext ? "DINE_IN" : "PICKUP",
    remark: "",
    submitted: false,
    ...(tableContext ? {
      orderScene: "DINE_IN" as const,
      tablePublicId: tableContext.tablePublicId,
      tablePublicScene: tableContext.publicScene,
      tableName: tableContext.tableName,
    } : {}),
    ...(fastFoodContext ? { fastFoodPlatePublicId: fastFoodContext.publicId, fastFoodPlateName: fastFoodContext.plateName } : {}),
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
  writeCheckoutFlow({ ...flow, fulfillmentType: flow.orderScene === "DINE_IN" ? "DINE_IN" : fulfillmentType, remark });
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
