import { describe, expect, it } from 'vitest';
import type { MerchantOperationSettings, MerchantSettings } from '../../types';
import {
  memberLevelOrderWritePayload,
  merchantSettingsWritePayload,
  operationSettingsWritePayload,
} from './payloads';

describe('settings write payloads', () => {
  it('does not let an unmounted field erase a loaded operation setting', () => {
    const settings = {
      storeId: 1,
      settlementMode: 'PAY_BEFORE',
      orderingMode: 'MULTI_PERSON',
      distanceCheckEnabled: false,
      distanceLimitM: 5000,
    } as MerchantOperationSettings;

    expect(operationSettingsWritePayload(settings, { distanceLimitM: undefined }).distanceLimitM).toBe(5000);
  });

  it('removes read-only license fields from merchant settings updates', () => {
    const settings = {
      storeName: '测试门店',
      businessLicenseUrl: 'https://example.test/business.png',
      foodBusinessLicenseUrl: 'https://example.test/food.png',
      allowLatePayment: false,
    } satisfies MerchantSettings;

    const payload = merchantSettingsWritePayload(settings, { allowLatePayment: true });
    expect(payload.allowLatePayment).toBe(true);
    expect(payload).not.toHaveProperty('businessLicenseUrl');
    expect(payload).not.toHaveProperty('foodBusinessLicenseUrl');
  });

  it('converts level-order yuan without retaining the form-only amount field', () => {
    const payload = memberLevelOrderWritePayload({
      customer_id: 2,
      level_id: 3,
      amount: 12.34,
      payment_method: 'MANUAL',
      status: 'COMPLETED',
    });
    expect(payload.amount_cents).toBe(1234);
    expect(payload).not.toHaveProperty('amount');
  });
});
