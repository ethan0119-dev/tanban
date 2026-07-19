import type { CartItem } from "../types/domain";

const LEGACY_CART_KEY = "tanban_cart";
const CART_KEY_PREFIX = "tanban_cart_v2:";

function cartKey(storeCode: string): string {
  const normalized = storeCode.trim();
  if (!normalized) throw new Error("缺少门店编码，无法读写购物车");
  return `${CART_KEY_PREFIX}${encodeURIComponent(normalized)}`;
}

function clearLegacyCart(): void {
  // 旧版购物车没有门店归属，无法安全判断应迁移到哪家门店。
  // 首次读取任一新购物车时直接清理，避免跨商户下单。
  if (wx.getStorageSync(LEGACY_CART_KEY)) wx.removeStorageSync(LEGACY_CART_KEY);
}

export function cartLineKey(item: Pick<CartItem, "productId" | "skuId" | "optionValueIds" | "modifiers" | "itemRemark">): string {
  const optionIds = [...(item.optionValueIds || [])].sort((a, b) => a - b).join(",");
  const modifiers = [...(item.modifiers || [])]
    .sort((a, b) => a.groupId - b.groupId || a.modifierItemId - b.modifierItemId)
    .map((entry) => `${entry.groupId}.${entry.modifierItemId}.${entry.quantity}`)
    .join(",");
  return `${item.productId}:${item.skuId ?? 0}:o=${optionIds}:m=${modifiers}:r=${item.itemRemark || ""}`;
}

export function readCart(storeCode: string): CartItem[] {
  clearLegacyCart();
  const stored = wx.getStorageSync<CartItem[]>(cartKey(storeCode));
  return Array.isArray(stored) ? stored : [];
}

export function writeCart(storeCode: string, items: CartItem[]): void {
  clearLegacyCart();
  wx.setStorageSync(cartKey(storeCode), items.filter((item) => item.quantity > 0));
}

export function addCartItem(storeCode: string, item: CartItem): CartItem[] {
  const items = readCart(storeCode);
  const key = cartLineKey(item);
  const current = items.find((entry) => cartLineKey(entry) === key);
  if (current) current.quantity += item.quantity;
  else items.push(item);
  writeCart(storeCode, items);
  return items;
}

export function changeCartLineQuantity(storeCode: string, lineKey: string, delta: number): CartItem[] {
  const items = readCart(storeCode);
  const current = items.find((entry) => cartLineKey(entry) === lineKey);
  if (current) current.quantity += delta;
  writeCart(storeCode, items);
  return readCart(storeCode);
}

export function changeCartItemQuantity(
  storeCode: string,
  productId: number,
  skuId: number | undefined,
  delta: number,
): CartItem[] {
  const items = readCart(storeCode);
  const current = items.find((entry) => entry.productId === productId && entry.skuId === skuId);
  if (current) current.quantity += delta;
  writeCart(storeCode, items);
  return readCart(storeCode);
}

export function clearCart(storeCode: string): void {
  wx.removeStorageSync(cartKey(storeCode));
}
