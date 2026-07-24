import { describe, expect, it } from 'vitest';
import { paymentProviderName } from './payment';

describe('paymentProviderName', () => {
  it('renders every supported adapter with customer-facing copy', () => {
    expect(paymentProviderName('mock')).toBe('模拟支付');
    expect(paymentProviderName('tianque')).toBe('会生活 · 随行付');
    expect(paymentProviderName('wechat_partner')).toBe('微信支付');
  });

  it('keeps an already formatted method and supplies a neutral fallback', () => {
    expect(paymentProviderName('现金')).toBe('现金');
    expect(paymentProviderName('')).toBe('在线支付');
  });
});
