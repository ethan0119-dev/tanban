import type { DashboardData, Order } from '../../types';

const ORDER_TYPE_LABELS: Record<string, string> = {
  DINE_IN: '堂食',
  TAKEOUT: '外带',
  DELIVERY: '外卖',
};

const ORDER_TYPE_COLORS: Record<string, string> = {
  DINE_IN: '#a5683f',
  TAKEOUT: '#d99b68',
  DELIVERY: '#7db38b',
};

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {};
}

function normalizeOrderTypes(value: unknown): NonNullable<DashboardData['todayOrderTypes']> {
  if (!Array.isArray(value)) return [];
  return value.map((item) => {
    const point = record(item);
    return {
      type: String(point.type ?? point.order_type ?? '').toUpperCase(),
      value: Number(point.value ?? point.count ?? 0),
    };
  }).filter((item) => item.type && Number.isFinite(item.value) && item.value > 0);
}

function normalizeHourly(value: unknown): NonNullable<DashboardData['todayHourly']> {
  if (!Array.isArray(value)) return [];
  return value.map((item) => {
    const point = record(item);
    return {
      hour: String(point.hour ?? ''),
      count: Number(point.count ?? point.value ?? 0),
    };
  }).filter((item) => item.hour && Number.isFinite(item.count) && item.count >= 0);
}

export function normalizeDashboard(raw: unknown): DashboardData {
  const value = record(raw);
  return {
    todayRevenue: value.today_revenue_cents !== undefined
      ? Number(value.today_revenue_cents) / 100
      : Number(value.todayRevenue ?? value.today_revenue ?? value.revenue ?? 0),
    todayOrders: Number(value.todayOrders ?? value.today_orders ?? value.orderCount ?? 0),
    pendingOrders: Number(value.pendingOrders ?? value.pending_orders ?? value.pendingCount ?? value.active_orders ?? 0),
    averageOrderValue: Number(value.averageOrderValue ?? value.average_order_value ?? value.avgOrderAmount ?? 0),
    yesterdayRevenue: Number(value.yesterdayRevenue ?? value.yesterday_revenue ?? 0),
    refundAmount: value.today_refunded_cents !== undefined ? Number(value.today_refunded_cents) / 100 : Number(value.refundAmount ?? value.refund_amount ?? 0),
    revenueTrend: (value.revenueTrend ?? value.revenue_trend ?? []) as DashboardData['revenueTrend'],
    monthlyTrend: (value.monthlyTrend ?? value.monthly_trend ?? []) as DashboardData['monthlyTrend'],
    todayOrderTypes: normalizeOrderTypes(value.todayOrderTypes ?? value.today_order_types),
    todayHourly: normalizeHourly(value.todayHourly ?? value.today_hourly),
    popularProducts: (value.popularProducts ?? value.popular_products ?? []) as DashboardData['popularProducts'],
    recentOrders: (value.recentOrders ?? value.recent_orders ?? []) as Order[],
  };
}

export function orderTypeChartData(points: DashboardData['todayOrderTypes']) {
  return (points ?? []).map((point) => ({
    name: ORDER_TYPE_LABELS[point.type] || point.type,
    value: point.value,
    color: ORDER_TYPE_COLORS[point.type] || '#c0b0a0',
  }));
}

