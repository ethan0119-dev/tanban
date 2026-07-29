const CUSTOMER_KEY = "tanban_customer_guest_key_v1";

/**
 * Install-scoped anonymous CRM key. It is a correlation identifier only, not
 * an authentication credential. A future wx.login flow can merge it into the
 * verified OpenID customer record.
 */
export function customerGuestKey(): string {
  const existing = wx.getStorageSync<string>(CUSTOMER_KEY);
  if (typeof existing === "string" && existing.length >= 12 && existing.length <= 128) return existing;
  const generated = `guest_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 14)}`;
  wx.setStorageSync(CUSTOMER_KEY, generated);
  return generated;
}
