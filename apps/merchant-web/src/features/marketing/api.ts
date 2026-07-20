import { api } from '../../api/client';
import type {
  CouponCampaign,
  CouponCampaignPayload,
  CouponDistributionChannel,
  CouponType,
  LotteryCampaign,
  LotteryCampaignPayload,
  LotteryPrize,
  MarketingAppRecord,
  MarketingCampaignStatus,
  MarketingPlacement,
  MarketingPlacementAction,
  MarketingPlacementCode,
  MarketingPlacementFrequency,
  MarketingPlacementPayload,
} from './types';

type UnknownRecord = Record<string, unknown>;

function record(value: unknown): UnknownRecord {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as UnknownRecord : {};
}

function text(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

function number(value: unknown, fallback = 0): number {
  const result = Number(value);
  return Number.isFinite(result) ? result : fallback;
}

function boolean(value: unknown, fallback = false): boolean {
  if (typeof value === 'boolean') return value;
  if (value === 1 || value === '1' || value === 'true') return true;
  if (value === 0 || value === '0' || value === 'false') return false;
  return fallback;
}

function list(value: unknown): unknown[] {
  if (Array.isArray(value)) return value;
  const source = record(value);
  const nested = source.items ?? source.list ?? source.records ?? source.rows ?? source.apps ?? source.campaigns ?? source.placements;
  return Array.isArray(nested) ? nested : [];
}

function value(source: UnknownRecord, camel: string, snake: string): unknown {
  return source[camel] ?? source[snake];
}

function campaignStatus(raw: unknown): MarketingCampaignStatus {
  const status = text(raw).toUpperCase();
  return status === 'ACTIVE' || status === 'PAUSED' || status === 'ENDED' ? status : 'DRAFT';
}

export function normalizeMarketingApps(payload: unknown): MarketingAppRecord[] {
  return list(payload).map((raw) => {
    const source = record(raw);
    return {
      key: text(source.key ?? source.code ?? source.app_key ?? source.appKey),
      name: text(source.name ?? source.title),
      description: text(source.description),
      status: text(source.status),
      available: boolean(source.available ?? source.enabled, false),
      route: text(source.route ?? source.path),
    };
  }).filter((item) => item.key || item.name);
}

export function normalizeCouponCampaign(raw: unknown): CouponCampaign {
  const source = record(raw);
  const type = text(source.type ?? source.coupon_type).toUpperCase() === 'FULL_REDUCTION' ? 'FULL_REDUCTION' : 'CASH';
  const distribution = text(source.distributionChannel ?? source.distribution_mode).toUpperCase();
  return {
    id: (source.id ?? '') as string | number,
    name: text(source.name),
    description: text(source.description),
    type: type as CouponType,
    status: campaignStatus(source.status),
    distributionChannel: (distribution === 'MANUAL_ONLY' || distribution === 'LOTTERY_ONLY' ? distribution : 'PUBLIC_CLAIM') as CouponDistributionChannel,
    thresholdCents: number(value(source, 'thresholdCents', 'threshold_cents')),
    discountCents: number(value(source, 'discountCents', 'discount_cents')),
    totalStock: number(value(source, 'totalStock', 'total_stock')),
    issuedCount: number(value(source, 'issuedCount', 'issued_count')),
    redeemedCount: number(source.redeemedCount ?? source.redeemed_count ?? source.use_count),
    perSubjectLimit: number(value(source, 'perSubjectLimit', 'per_subject_limit'), 1),
    claimStartAt: text(value(source, 'claimStartAt', 'claim_start_at')) || undefined,
    claimEndAt: text(value(source, 'claimEndAt', 'claim_end_at')) || undefined,
    validityMode: text(value(source, 'validityMode', 'validity_mode')).toUpperCase() === 'FIXED_RANGE' ? 'FIXED_RANGE' : 'RELATIVE_DAYS',
    validFrom: text(value(source, 'validFrom', 'valid_from')) || undefined,
    validTo: text(value(source, 'validTo', 'valid_to')) || undefined,
    validDays: number(value(source, 'validDays', 'valid_days'), 30),
    orderTypes: Array.isArray(value(source, 'orderTypes', 'order_types')) ? value(source, 'orderTypes', 'order_types') as string[] : [],
    createdAt: text(value(source, 'createdAt', 'created_at')) || undefined,
    updatedAt: text(value(source, 'updatedAt', 'updated_at')) || undefined,
  };
}

export function normalizeMarketingPlacement(raw: unknown): MarketingPlacement {
  const source = record(raw);
  const frequency = text(source.frequency).toUpperCase();
  const actionType = text(value(source, 'actionType', 'action_type')).toUpperCase();
  return {
    id: (source.id ?? '') as string | number,
    name: text(source.name),
    title: text(source.title),
    subtitle: text(source.subtitle),
    imageUrl: text(value(source, 'imageUrl', 'image_url')),
    slot: (['MENU_POPUP', 'CHECKOUT_POPUP', 'ORDER_RESULT_POPUP', 'PROFILE_POPUP'].includes(text(value(source, 'slot', 'placement_code')).toUpperCase())
      ? text(value(source, 'slot', 'placement_code')).toUpperCase()
      : 'HOME_POPUP') as MarketingPlacementCode,
    status: campaignStatus(source.status),
    frequency: (frequency === 'EVERY_VISIT' || frequency === 'DAILY' ? frequency : 'ONCE_PER_CAMPAIGN') as MarketingPlacementFrequency,
    actionType: (['OPEN_MENU', 'OPEN_COUPONS', 'CLAIM_COUPON', 'OPEN_LOTTERY'].includes(actionType) ? actionType : 'NONE') as MarketingPlacementAction,
    actionTargetId: value(source, 'actionTargetId', 'action_target_id') as string | number | undefined,
    priority: number(source.priority),
    channelScope: text(value(source, 'channelScope', 'channel_scope'), 'ALL'),
    exposureCount: number(source.exposureCount ?? source.exposure_count ?? source.impression_count),
    clickCount: number(value(source, 'clickCount', 'click_count')),
    startsAt: text(source.startsAt ?? source.starts_at ?? source.active_from) || undefined,
    endsAt: text(source.endsAt ?? source.ends_at ?? source.active_to) || undefined,
    createdAt: text(value(source, 'createdAt', 'created_at')) || undefined,
    updatedAt: text(value(source, 'updatedAt', 'updated_at')) || undefined,
  };
}

function normalizeLotteryPrize(raw: unknown, index: number): LotteryPrize {
  const source = record(raw);
  return {
    id: source.id as string | number | undefined,
    name: text(source.name, `奖项 ${index + 1}`),
    prizeType: text(value(source, 'prizeType', 'prize_type')).toUpperCase() === 'COUPON' ? 'COUPON' : 'THANKS',
    couponCampaignId: value(source, 'couponCampaignId', 'coupon_campaign_id') as string | number | undefined,
    totalStock: number(value(source, 'totalStock', 'total_stock')),
    awardedCount: number(value(source, 'awardedCount', 'awarded_count')),
    weight: number(source.weight),
    sortOrder: number(value(source, 'sortOrder', 'sort_order'), index),
    status: text(source.status),
  };
}

export function normalizeLotteryCampaign(raw: unknown): LotteryCampaign {
  const source = record(raw);
  const prizes = Array.isArray(source.prizes) ? source.prizes : [];
  return {
    id: (source.id ?? '') as string | number,
    name: text(source.name),
    description: text(source.description),
    status: campaignStatus(source.status),
    channelScope: text(value(source, 'channelScope', 'channel_scope'), 'ALL'),
    dailyLimit: number(value(source, 'dailyLimit', 'daily_limit'), 1),
    totalLimit: number(value(source, 'totalLimit', 'total_limit')),
    terms: text(source.terms),
    drawCount: number(value(source, 'drawCount', 'draw_count')),
    winCount: number(value(source, 'winCount', 'win_count')),
    activeFrom: text(value(source, 'activeFrom', 'active_from')) || undefined,
    activeTo: text(value(source, 'activeTo', 'active_to')) || undefined,
    prizes: prizes.map(normalizeLotteryPrize),
    createdAt: text(value(source, 'createdAt', 'created_at')) || undefined,
    updatedAt: text(value(source, 'updatedAt', 'updated_at')) || undefined,
  };
}

export const marketingApi = {
  async listApps(): Promise<MarketingAppRecord[]> {
    return normalizeMarketingApps(await api.get<unknown>('/merchant/marketing/apps'));
  },
  async listCoupons(): Promise<CouponCampaign[]> {
    const response = await api.get<unknown>('/merchant/marketing/coupons', { page_size: 100 });
    return list(response).map(normalizeCouponCampaign);
  },
  createCoupon: (payload: CouponCampaignPayload) => api.post('/merchant/marketing/coupons', payload),
  updateCoupon: (id: string | number, payload: CouponCampaignPayload) => api.put(`/merchant/marketing/coupons/${id}`, payload),
  activateCoupon: (id: string | number) => api.post(`/merchant/marketing/coupons/${id}/activate`),
  pauseCoupon: (id: string | number) => api.post(`/merchant/marketing/coupons/${id}/pause`),
  async listPlacements(): Promise<MarketingPlacement[]> {
    const response = await api.get<unknown>('/merchant/marketing/placements', { page_size: 100 });
    return list(response).map(normalizeMarketingPlacement);
  },
  createPlacement: (payload: MarketingPlacementPayload) => api.post('/merchant/marketing/placements', payload),
  updatePlacement: (id: string | number, payload: MarketingPlacementPayload) => api.put(`/merchant/marketing/placements/${id}`, payload),
  activatePlacement: (id: string | number) => api.post(`/merchant/marketing/placements/${id}/activate`),
  pausePlacement: (id: string | number) => api.post(`/merchant/marketing/placements/${id}/pause`),
  async listLotteries(): Promise<LotteryCampaign[]> {
    const response = await api.get<unknown>('/merchant/marketing/lotteries', { page_size: 100 });
    return list(response).map(normalizeLotteryCampaign);
  },
  createLottery: (payload: LotteryCampaignPayload) => api.post('/merchant/marketing/lotteries', payload),
  updateLottery: (id: string | number, payload: LotteryCampaignPayload) => api.put(`/merchant/marketing/lotteries/${id}`, payload),
  activateLottery: (id: string | number) => api.post(`/merchant/marketing/lotteries/${id}/activate`),
  pauseLottery: (id: string | number) => api.post(`/merchant/marketing/lotteries/${id}/pause`),
};
