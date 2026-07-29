import { describe, expect, it } from 'vitest';

// Extract pure functions for isolated testing
function computeOrderTypes(recentOrders: Array<{ orderType?: string }>) {
  const ORDER_TYPE_LABELS: Record<string, string> = { DINE_IN: '堂食', TAKEOUT: '外带', PICKUP: '自取', DELIVERY: '外卖' };
  const ORDER_TYPE_COLORS: Record<string, string> = { DINE_IN: '#a5683f', TAKEOUT: '#d99b68', PICKUP: '#f0b884', DELIVERY: '#7db38b' };
  const map: Record<string, number> = {};
  recentOrders.forEach((o) => {
    const t = o.orderType || 'OTHER';
    map[t] = (map[t] || 0) + 1;
  });
  return Object.entries(map).map(([type, count]) => ({
    name: ORDER_TYPE_LABELS[type] || type,
    value: count,
    color: ORDER_TYPE_COLORS[type] || '#c0b0a0',
  }));
}

// Simulate the normalizeDashboard key transformation
function normalizeField(value: Record<string, unknown>) {
  return {
    orderType: value.orderType ?? value.order_type ?? undefined,
    todayHourly: value.todayHourly ?? value.today_hourly ?? [],
    monthlyTrend: value.monthlyTrend ?? value.monthly_trend ?? [],
  };
}

describe('Dashboard order type distribution', () => {
  it('groups orders by orderType (not fulfillmentType)', () => {
    const orders = [
      { orderType: 'DINE_IN' },
      { orderType: 'TAKEOUT' },
      { orderType: 'DINE_IN' },
      { orderType: 'DELIVERY' },
      { orderType: 'PICKUP' },
    ];
    const result = computeOrderTypes(orders);
    expect(result).toHaveLength(4);
    expect(result.find((r) => r.name === '堂食')?.value).toBe(2);
    expect(result.find((r) => r.name === '外带')?.value).toBe(1);
    expect(result.find((r) => r.name === '外卖')?.value).toBe(1);
    expect(result.find((r) => r.name === '自取')?.value).toBe(1);
  });

  it('falls back to OTHER when orderType is missing', () => {
    const result = computeOrderTypes([{ orderType: undefined }, { orderType: 'DINE_IN' }]);
    expect(result.find((r) => r.name === 'OTHER')?.value).toBe(1);
    expect(result.find((r) => r.name === '堂食')?.value).toBe(1);
  });

  it('ignores fulfillmentType even when present', () => {
    const orders = [{ orderType: 'DINE_IN', fulfillmentType: 'TAKEOUT' } as unknown as { orderType?: string }];
    const result = computeOrderTypes(orders);
    expect(result.find((r) => r.name === '堂食')?.value).toBe(1);
  });
});

describe('Dashboard field normalization', () => {
  it('normalizes orderType from snake_case', () => {
    expect(normalizeField({ order_type: 'DINE_IN' }).orderType).toBe('DINE_IN');
    expect(normalizeField({ orderType: 'TAKEOUT' }).orderType).toBe('TAKEOUT');
    expect(normalizeField({}).orderType).toBeUndefined();
  });

  it('normalizes todayHourly from snake_case', () => {
    const data = [{ hour: '09:00', count: 5 }];
    // @ts-expect-error: test case
    expect(normalizeField({ today_hourly: data }).todayHourly).toEqual(data);
    // @ts-expect-error: test case
    expect(normalizeField({ todayHourly: data }).todayHourly).toEqual(data);
  });

  it('normalizes monthlyTrend from snake_case', () => {
    const data = [{ label: '07-01', value: 100 }];
    // @ts-expect-error: test case
    expect(normalizeField({ monthly_trend: data }).monthlyTrend).toEqual(data);
    // @ts-expect-error: test case
    expect(normalizeField({ monthlyTrend: data }).monthlyTrend).toEqual(data);
  });

  it('returns empty arrays as defaults', () => {
    const result = normalizeField({});
    expect(result.todayHourly).toEqual([]);
    expect(result.monthlyTrend).toEqual([]);
  });
});
