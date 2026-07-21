import { describe, expect, it } from 'vitest';
import type { Order } from '../../types';
import { ordersForBusinessType } from './model';

const order = (id: string, orderType: Order['orderType']): Order => ({
  id,
  orderNo: id,
  orderType,
  // Dine-in and takeaway are separate scenes inside the same in-store domain.
  businessType: orderType === 'DELIVERY' ? 'DELIVERY' : 'DINE_IN',
  status: 'PAID',
  amount: 10,
  createdAt: '2026-07-21T00:00:00Z',
  items: [],
});

describe('ordersForBusinessType', () => {
  it('never shows fast-food cards in the dine-in route', () => {
    expect(ordersForBusinessType([order('dine', 'DINE_IN'), order('takeout', 'TAKEOUT')], 'DINE_IN').map((item) => item.id)).toEqual(['dine']);
  });

  it('keeps dine-in cards out of the fast-food route', () => {
    expect(ordersForBusinessType([order('dine', 'DINE_IN'), order('takeout', 'TAKEOUT')], 'TAKEOUT').map((item) => item.id)).toEqual(['takeout']);
  });
});
