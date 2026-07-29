const ORDER_HISTORY_KEY = "tanban_order_history_v1";

type OrderHistory = Record<string, string[]>;

function readHistory(): OrderHistory {
  const stored = wx.getStorageSync<OrderHistory>(ORDER_HISTORY_KEY);
  return stored && typeof stored === "object" ? stored : {};
}

export function rememberOrder(storeCode: string, orderNo: string): void {
  const history = readHistory();
  history[storeCode] = [orderNo, ...(history[storeCode] || []).filter((item) => item !== orderNo)].slice(0, 50);
  wx.setStorageSync(ORDER_HISTORY_KEY, history);
}

export function localOrderNumbers(storeCode: string): string[] {
  return readHistory()[storeCode] || [];
}

function orderTime(value: string): number {
  const source = String(value || "").trim();
  if (!source) return 0;
  const normalized = source.includes("T") ? source : `${source.replace(" ", "T")}+08:00`;
  const timestamp = Date.parse(normalized);
  return Number.isFinite(timestamp) ? timestamp : 0;
}

export function sortCustomerOrders<T extends { current: boolean; createdAt: string; id: number }>(orders: T[]): T[] {
  return [...orders].sort((left, right) => {
    if (left.current !== right.current) return left.current ? -1 : 1;
    return orderTime(right.createdAt) - orderTime(left.createdAt) || right.id - left.id;
  });
}
