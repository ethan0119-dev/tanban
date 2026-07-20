import type { FastFoodOrderingContext } from "../types/domain";
import { ApiError, request } from "./request";

const FAST_FOOD_CONTEXT_KEY = "tanban_fast_food_context_v1";
const MAX_AGE_MS = 12 * 60 * 60 * 1000;

interface FastFoodPlateResolution {
  publicId?: string;
  storeCode?: string;
  storeName?: string;
  plateCode?: string;
  plateName?: string;
  status?: string;
}

export class FastFoodRouteError extends Error {
  constructor(message: string, public readonly code = "INVALID_FAST_FOOD_ROUTE") {
    super(message);
    this.name = "FastFoodRouteError";
  }
}

function text(value: unknown): string {
  return typeof value === "string" || typeof value === "number" ? String(value).trim() : "";
}

export function normalizeFastFoodResolution(publicId: string, response: FastFoodPlateResolution, resolvedAt = Date.now()): FastFoodOrderingContext {
  const storeCode = text(response.storeCode);
  const plateCode = text(response.plateCode);
  const plateName = text(response.plateName) || plateCode;
  if (!publicId || text(response.publicId) !== publicId || !storeCode || !plateCode || response.status !== "ACTIVE") {
    throw new FastFoodRouteError("快餐码牌不可用，请联系商家重新生成");
  }
  return { publicId, storeCode, storeName: text(response.storeName) || storeCode, plateCode, plateName, resolvedAt, validUntil: resolvedAt + MAX_AGE_MS };
}

export function fastFoodContextIsValid(value: unknown, now = Date.now()): value is FastFoodOrderingContext {
  if (!value || typeof value !== "object") return false;
  const context = value as Partial<FastFoodOrderingContext>;
  return Boolean(text(context.publicId) && text(context.storeCode) && text(context.plateCode) && typeof context.resolvedAt === "number" && typeof context.validUntil === "number" && context.validUntil > now);
}

export function readFastFoodContext(now = Date.now()): FastFoodOrderingContext | null {
  const stored = wx.getStorageSync<FastFoodOrderingContext>(FAST_FOOD_CONTEXT_KEY);
  if (!fastFoodContextIsValid(stored, now)) {
    if (stored) wx.removeStorageSync(FAST_FOOD_CONTEXT_KEY);
    return null;
  }
  return stored;
}

export function saveFastFoodContext(context: FastFoodOrderingContext): void {
  if (!fastFoodContextIsValid(context)) throw new FastFoodRouteError("无法保存无效的快餐码牌");
  wx.setStorageSync(FAST_FOOD_CONTEXT_KEY, context);
}

export function clearFastFoodContext(): void { wx.removeStorageSync(FAST_FOOD_CONTEXT_KEY); }

export function fastFoodContextForStore(storeCode: string): FastFoodOrderingContext | null {
  const context = readFastFoodContext();
  return context?.storeCode === storeCode ? context : null;
}

export function sameFastFoodContext(left: FastFoodOrderingContext | null, right: FastFoodOrderingContext | null): boolean {
  if (!left || !right) return left === right;
  return left.storeCode === right.storeCode && left.publicId === right.publicId;
}

export async function resolveFastFoodContext(publicId: string, expectedStoreCode?: string): Promise<FastFoodOrderingContext> {
  try {
    const response = await request<FastFoodPlateResolution>({ url: `/public/fast-food-plates/${encodeURIComponent(publicId)}`, method: "GET" });
    const context = normalizeFastFoodResolution(publicId, response);
    if (expectedStoreCode && context.storeCode !== expectedStoreCode) throw new FastFoodRouteError("快餐码牌不属于当前门店", "FAST_FOOD_STORE_MISMATCH");
    return context;
  } catch (error) {
    if (error instanceof FastFoodRouteError) throw error;
    if (error instanceof ApiError && [400, 404, 410, 422].includes(error.statusCode)) throw new FastFoodRouteError("快餐码牌无效或已停用，请重新扫码", error.code);
    throw error;
  }
}

export async function revalidateFastFoodContext(context: FastFoodOrderingContext): Promise<FastFoodOrderingContext> {
  const current = readFastFoodContext();
  if (!current || !sameFastFoodContext(current, context)) throw new FastFoodRouteError("快餐码牌上下文已变化，请重新扫码");
  const verified = await resolveFastFoodContext(context.publicId, context.storeCode);
  saveFastFoodContext(verified);
  return verified;
}
