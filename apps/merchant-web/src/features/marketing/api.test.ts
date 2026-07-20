import { describe, expect, it } from 'vitest';
import {
  normalizeCouponCampaign,
  normalizeLotteryCampaign,
  normalizeMarketingApps,
  normalizeMarketingPlacement,
} from './api';

describe('marketing API normalization', () => {
  it('reads the snake-case coupon contract without changing cents', () => {
    expect(normalizeCouponCampaign({
      id: 9,
      name: '满 30 减 5',
      coupon_type: 'FULL_REDUCTION',
      distribution_mode: 'LOTTERY_ONLY',
      threshold_cents: 3000,
      discount_cents: 500,
      total_stock: 80,
      issued_count: 12,
      redeemed_count: 3,
      per_subject_limit: 1,
      claim_start_at: '2026-07-20T10:00:00Z',
      validity_mode: 'RELATIVE_DAYS',
      valid_days: 15,
      order_types: ['DINE_IN', 'TAKEOUT'],
      status: 'ACTIVE',
    })).toMatchObject({
      id: 9,
      type: 'FULL_REDUCTION',
      distributionChannel: 'LOTTERY_ONLY',
      thresholdCents: 3000,
      discountCents: 500,
      perSubjectLimit: 1,
      validDays: 15,
      orderTypes: ['DINE_IN', 'TAKEOUT'],
      status: 'ACTIVE',
    });
  });

  it('normalizes popup placement action and counters', () => {
    expect(normalizeMarketingPlacement({
      id: 3,
      placement_code: 'HOME_POPUP',
      title: '新品上市',
      action_type: 'CLAIM_COUPON',
      action_target_id: 9,
      frequency: 'ONCE_PER_CAMPAIGN',
      channel_scope: 'TAKEOUT',
      exposure_count: 100,
      click_count: 18,
    })).toMatchObject({
      id: 3,
      title: '新品上市',
      actionType: 'CLAIM_COUPON',
      actionTargetId: 9,
      frequency: 'ONCE_PER_CAMPAIGN',
      channelScope: 'TAKEOUT',
      exposureCount: 100,
      clickCount: 18,
    });
  });

  it('keeps lottery prize stock and coupon references', () => {
    expect(normalizeLotteryCampaign({
      id: 4,
      name: '幸运转盘',
      daily_limit: 1,
      total_limit: 1000,
      active_from: '2026-07-20T00:00:00Z',
      channel_scope: 'DINE_IN',
      prizes: [{ id: 41, name: '五元券', prize_type: 'COUPON', coupon_campaign_id: 9, weight: 10, total_stock: 50, awarded_count: 2 }],
    })).toMatchObject({
      id: 4,
      dailyLimit: 1,
      totalLimit: 1000,
      prizes: [{ id: 41, prizeType: 'COUPON', couponCampaignId: 9, totalStock: 50, awardedCount: 2 }],
    });
  });

  it('accepts both flat and embedded marketing app lists', () => {
    expect(normalizeMarketingApps({ apps: [{ key: 'COUPON', name: '优惠券', enabled: true }] })).toEqual([
      expect.objectContaining({ key: 'COUPON', name: '优惠券', available: true }),
    ]);
  });
});
