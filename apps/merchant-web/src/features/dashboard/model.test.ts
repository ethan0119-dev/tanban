import { describe, expect, it } from 'vitest';
import { normalizeDashboard, orderTypeChartData } from './model';

describe('dashboard data model', () => {
  it('normalizes backend aggregates without deriving analytics from recent orders', () => {
    const dashboard = normalizeDashboard({
      today_revenue_cents: 12345,
      today_orders: 7,
      today_order_types: [
        { type: 'DINE_IN', value: 4 },
        { order_type: 'TAKEOUT', count: 3 },
      ],
      today_hourly: [
        { hour: '09:00', count: 2 },
        { hour: '10:00', value: 5 },
      ],
      recentOrders: [
        { orderType: 'DELIVERY', createdAt: '2026-07-28 23:00:00' },
      ],
    });

    expect(dashboard.todayRevenue).toBe(123.45);
    expect(dashboard.todayOrderTypes).toEqual([
      { type: 'DINE_IN', value: 4 },
      { type: 'TAKEOUT', value: 3 },
    ]);
    expect(dashboard.todayHourly).toEqual([
      { hour: '09:00', count: 2 },
      { hour: '10:00', count: 5 },
    ]);
  });

  it('maps supported order types to stable labels and preserves unknown types', () => {
    expect(orderTypeChartData([
      { type: 'DINE_IN', value: 5 },
      { type: 'TAKEOUT', value: 3 },
      { type: 'DELIVERY', value: 2 },
      { type: 'CURBSIDE', value: 1 },
    ])).toEqual([
      { name: '堂食', value: 5, color: '#a5683f' },
      { name: '外带', value: 3, color: '#d99b68' },
      { name: '外卖', value: 2, color: '#7db38b' },
      { name: 'CURBSIDE', value: 1, color: '#c0b0a0' },
    ]);
  });
});

