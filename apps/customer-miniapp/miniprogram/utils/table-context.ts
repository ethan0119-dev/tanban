import type { TableOrderingContext } from "../types/domain";
import { ApiError, request } from "./request";

const TABLE_CONTEXT_KEY = "tanban_table_ordering_context_v1";
export const TABLE_CONTEXT_MAX_AGE_MS = 12 * 60 * 60 * 1000;

interface TableCodeResolution {
  tableCode?: string;
  expiresAt?: string;
  store?: { id?: number; code?: string; name?: string };
  table?: {
    publicId?: string | number;
    name?: string;
    tableCode?: string;
    areaName?: string;
  };
}

export class TableRouteError extends Error {
  constructor(message: string, public readonly code = "INVALID_TABLE_ROUTE") {
    super(message);
    this.name = "TableRouteError";
  }
}

function asText(value: unknown): string {
  return typeof value === "string" || typeof value === "number" ? String(value).trim() : "";
}

function expiryFrom(response: TableCodeResolution, resolvedAt: number): number {
  const remoteExpiry = response.expiresAt ? Date.parse(response.expiresAt) : Number.NaN;
  if (Number.isFinite(remoteExpiry)) return Math.min(remoteExpiry, resolvedAt + TABLE_CONTEXT_MAX_AGE_MS);
  return resolvedAt + TABLE_CONTEXT_MAX_AGE_MS;
}

export function normalizeTableCodeResolution(
  publicScene: string,
  response: TableCodeResolution,
  resolvedAt = Date.now(),
): TableOrderingContext {
  const storeCode = asText(response.store?.code);
  const storeName = asText(response.store?.name);
  const tablePublicId = asText(response.table?.publicId);
  const tableName = asText(response.table?.name) || asText(response.table?.tableCode);
  if (!publicScene || !storeCode || !tablePublicId || !tableName) {
    throw new TableRouteError("桌码信息不完整，请联系商家重新生成桌码");
  }
  const validUntil = expiryFrom(response, resolvedAt);
  if (validUntil <= resolvedAt) throw new TableRouteError("桌码已失效，请重新扫码", "TABLE_CODE_EXPIRED");
  return {
    publicScene,
    storeCode,
    storeName: storeName || storeCode,
    tablePublicId,
    tableName,
    ...(asText(response.table?.tableCode || response.tableCode) ? { tableCode: asText(response.table?.tableCode || response.tableCode) } : {}),
    ...(asText(response.table?.areaName) ? { areaName: asText(response.table?.areaName) } : {}),
    resolvedAt,
    validUntil,
  };
}

export function tableContextIsValid(context: unknown, now = Date.now()): context is TableOrderingContext {
  if (!context || typeof context !== "object") return false;
  const value = context as Partial<TableOrderingContext>;
  return Boolean(
    asText(value.publicScene)
    && asText(value.storeCode)
    && asText(value.tablePublicId)
    && asText(value.tableName)
    && typeof value.resolvedAt === "number"
    && typeof value.validUntil === "number"
    && value.resolvedAt <= now + 60_000
    && value.validUntil > now,
  );
}

export function readTableOrderingContext(now = Date.now()): TableOrderingContext | null {
  const stored = wx.getStorageSync<TableOrderingContext>(TABLE_CONTEXT_KEY);
  if (!tableContextIsValid(stored, now)) {
    if (stored) wx.removeStorageSync(TABLE_CONTEXT_KEY);
    return null;
  }
  return stored;
}

export function saveTableOrderingContext(context: TableOrderingContext): void {
  if (!tableContextIsValid(context)) throw new TableRouteError("无法保存无效的桌码信息");
  wx.setStorageSync(TABLE_CONTEXT_KEY, context);
}

export function clearTableOrderingContext(): void {
  wx.removeStorageSync(TABLE_CONTEXT_KEY);
}

export function tableContextForStore(storeCode: string, now = Date.now()): TableOrderingContext | null {
  const context = readTableOrderingContext(now);
  if (!context) return null;
  return context.storeCode === storeCode ? context : null;
}

export function sameTableContext(left: TableOrderingContext | null, right: TableOrderingContext | null): boolean {
  if (!left || !right) return left === right;
  return left.storeCode === right.storeCode
    && left.publicScene === right.publicScene
    && left.tablePublicId === right.tablePublicId;
}

export async function resolveTableOrderingContext(
  publicScene: string,
  expectedStoreCode?: string,
): Promise<TableOrderingContext> {
  try {
    const response = await request<TableCodeResolution>({
      url: `/public/table-codes/${encodeURIComponent(publicScene)}`,
      method: "GET",
    });
    const context = normalizeTableCodeResolution(publicScene, response);
    if (expectedStoreCode && context.storeCode !== expectedStoreCode) {
      throw new TableRouteError("桌码不属于当前门店，请重新扫码", "TABLE_STORE_MISMATCH");
    }
    return context;
  } catch (error) {
    if (error instanceof TableRouteError) throw error;
    if (error instanceof ApiError && [400, 404, 410, 422].includes(error.statusCode)) {
      throw new TableRouteError("桌码无效、已停用或已过期，请重新扫码", error.code || "TABLE_CODE_INVALID");
    }
    throw error;
  }
}

export async function revalidateTableOrderingContext(context: TableOrderingContext): Promise<TableOrderingContext> {
  const current = readTableOrderingContext();
  if (!current || !sameTableContext(current, context)) {
    throw new TableRouteError("桌码上下文已变化，请重新扫码后下单", "TABLE_CONTEXT_CHANGED");
  }
  const verified = await resolveTableOrderingContext(context.publicScene, context.storeCode);
  if (verified.tablePublicId !== context.tablePublicId) {
    throw new TableRouteError("桌码对应桌台已变化，请重新扫码", "TABLE_CHANGED");
  }
  saveTableOrderingContext(verified);
  return verified;
}

export function tableOrderFields(context: TableOrderingContext | null): Record<string, string> {
  return context ? { order_scene: "DINE_IN", table_public_id: context.tablePublicId } : {};
}
