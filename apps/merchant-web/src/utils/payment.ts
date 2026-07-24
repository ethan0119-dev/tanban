const paymentProviderNames: Record<string, string> = {
  mock: '模拟支付',
  tianque: '会生活 · 随行付',
  wechat_partner: '微信支付',
};

export function paymentProviderName(value?: unknown, fallback = '在线支付'): string {
  const provider = String(value ?? '').trim();
  if (!provider) return fallback;
  return paymentProviderNames[provider.toLowerCase()] || provider;
}
