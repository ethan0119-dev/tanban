import { beforeEach, describe, expect, it, vi } from 'vitest';
import { bestEligibleCoupon, eligibleCoupons, rememberClaimedCoupon } from '../miniprogram/utils/coupon-wallet';
import type { MarketingCoupon } from '../miniprogram/types/domain';

describe('coupon checkout selection', () => {
  let storage: Map<string, unknown>;

  beforeEach(() => {
    storage = new Map();
    vi.stubGlobal('wx', {
      getStorageSync: (key: string) => storage.get(key),
      setStorageSync: (key: string, value: unknown) => storage.set(key, value),
    });
  });

  it('lists all eligible coupons and defaults to the largest discount', () => {
    const base: MarketingCoupon = {
      id: 1, name: '满 30 减 5', coupon_type: 'FULL_REDUCTION', threshold_cents: 3000,
      discount_cents: 500, distribution_mode: 'PUBLIC_CLAIM', total_stock: 100,
      issued_count: 1, per_subject_limit: 1, valid_days: 30, order_types: ['TAKEOUT'], status: 'ACTIVE',
    };
    rememberClaimedCoupon('coffee', base);
    rememberClaimedCoupon('coffee', { ...base, id: 2, name: '满 50 减 10', threshold_cents: 5000, discount_cents: 1000 });
    rememberClaimedCoupon('coffee', { ...base, id: 3, name: '堂食券', discount_cents: 1200, order_types: ['DINE_IN'] });

    expect(eligibleCoupons('coffee', 6000, 'TAKEOUT').map((item) => item.id)).toEqual([2, 1]);
    expect(bestEligibleCoupon('coffee', 6000, 'TAKEOUT')?.id).toBe(2);
  });

  it('excludes coupons whose threshold is not met', () => {
    const coupon: MarketingCoupon = {
      id: 1, name: '满 50 减 10', coupon_type: 'FULL_REDUCTION', threshold_cents: 5000,
      discount_cents: 1000, distribution_mode: 'PUBLIC_CLAIM', total_stock: 100,
      issued_count: 1, per_subject_limit: 1, valid_days: 30, order_types: ['TAKEOUT'], status: 'ACTIVE',
    };
    rememberClaimedCoupon('coffee', coupon);
    expect(eligibleCoupons('coffee', 4999, 'TAKEOUT')).toEqual([]);
  });
});
