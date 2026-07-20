import type { Id } from '../../types';

export type MarketingCampaignStatus = 'DRAFT' | 'ACTIVE' | 'PAUSED' | 'ENDED';
export type CouponType = 'CASH' | 'FULL_REDUCTION';
export type CouponDistributionChannel = 'PUBLIC_CLAIM' | 'MANUAL_ONLY' | 'LOTTERY_ONLY';
export type MarketingPlacementAction = 'NONE' | 'OPEN_MENU' | 'OPEN_COUPONS' | 'CLAIM_COUPON' | 'OPEN_LOTTERY';
export type MarketingPlacementFrequency = 'EVERY_VISIT' | 'DAILY' | 'ONCE_PER_CAMPAIGN';
export type MarketingPlacementCode = 'HOME_POPUP' | 'MENU_POPUP' | 'CHECKOUT_POPUP' | 'ORDER_RESULT_POPUP' | 'PROFILE_POPUP';
export type LotteryPrizeType = 'THANKS' | 'COUPON';
export type CouponValidityMode = 'FIXED_RANGE' | 'RELATIVE_DAYS';

export interface MarketingAppRecord {
  key: string;
  name: string;
  description?: string;
  status?: string;
  available?: boolean;
  route?: string;
}

export interface CouponCampaign {
  id: Id;
  name: string;
  description: string;
  type: CouponType;
  status: MarketingCampaignStatus;
  distributionChannel: CouponDistributionChannel;
  thresholdCents: number;
  discountCents: number;
  totalStock: number;
  issuedCount: number;
  redeemedCount: number;
  perSubjectLimit: number;
  claimStartAt?: string;
  claimEndAt?: string;
  validityMode: CouponValidityMode;
  validFrom?: string;
  validTo?: string;
  validDays: number;
  orderTypes: string[];
  createdAt?: string;
  updatedAt?: string;
}

export interface MarketingPlacement {
  id: Id;
  name: string;
  title: string;
  subtitle: string;
  imageUrl: string;
  slot: MarketingPlacementCode;
  status: MarketingCampaignStatus;
  frequency: MarketingPlacementFrequency;
  actionType: MarketingPlacementAction;
  actionTargetId?: Id;
  priority: number;
  channelScope: string;
  exposureCount: number;
  clickCount: number;
  startsAt?: string;
  endsAt?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface LotteryPrize {
  id?: Id;
  name: string;
  prizeType: LotteryPrizeType;
  couponCampaignId?: Id;
  totalStock: number;
  awardedCount: number;
  weight: number;
  sortOrder: number;
  status?: string;
}

export interface LotteryCampaign {
  id: Id;
  name: string;
  description: string;
  status: MarketingCampaignStatus;
  channelScope: string;
  dailyLimit: number;
  totalLimit: number;
  terms: string;
  drawCount: number;
  winCount: number;
  activeFrom?: string;
  activeTo?: string;
  prizes: LotteryPrize[];
  createdAt?: string;
  updatedAt?: string;
}

export interface CouponCampaignPayload {
  name: string;
  description: string;
  coupon_type: CouponType;
  distribution_mode: CouponDistributionChannel;
  threshold_cents: number;
  discount_cents: number;
  total_stock: number;
  per_subject_limit: number;
  claim_start_at?: string;
  claim_end_at?: string;
  validity_mode: CouponValidityMode;
  valid_from?: string;
  valid_to?: string;
  valid_days: number;
  order_types: string[];
}

export interface MarketingPlacementPayload {
  name: string;
  title: string;
  subtitle: string;
  image_url: string;
  placement_code: MarketingPlacementCode;
  frequency: MarketingPlacementFrequency;
  action_type: MarketingPlacementAction;
  action_target_id?: Id;
  priority: number;
  channel_scope: string;
  active_from?: string;
  active_to?: string;
}

export interface LotteryCampaignPayload {
  name: string;
  description: string;
  channel_scope: string;
  active_from: string;
  active_to: string;
  daily_limit: number;
  total_limit: number;
  terms: string;
  prizes: Array<{
    name: string;
    prize_type: LotteryPrizeType;
    coupon_campaign_id?: Id;
    total_stock: number;
    weight: number;
    sort_order: number;
    status: string;
  }>;
}
