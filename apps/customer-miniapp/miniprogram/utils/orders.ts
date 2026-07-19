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
