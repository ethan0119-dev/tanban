import { describe, expect, it } from 'vitest';
import type { Order } from '../../types';
import {
  addItemsBlockedReason,
  canAddItemsToOrder,
  canOperateUnpaidPayAfterOrder,
  canReturnItemsFromOrder,
  dineInOperationBlockedReason,
  isMergeableTableBill,
} from './editability';

function order(overrides: Partial<Order>): Order {
  return {
    id: 1,
    orderNo: 'TB1',
    status: 'PREPARING',
    paymentStatus: 'UNPAID',
    settlementMode: 'PAY_AFTER',
    paidAmount: 0,
    amount: 20,
    orderType: 'DINE_IN',
    createdAt: '',
    items: [],
    ...overrides,
  };
}

describe('dine-in item editability', () => {
  it.each(['PAID', 'ACCEPTED', 'PREPARING', 'READY'] as const)(
    'allows additions to an unpaid pay-after order in %s',
    (status) => expect(canAddItemsToOrder(order({ status }))).toBe(true),
  );

  it('allows item changes on a pay-before order only before payment starts', () => {
    const pending = order({ settlementMode: 'PAY_BEFORE', status: 'PENDING_PAYMENT' });
    expect(canAddItemsToOrder(pending)).toBe(true);
    expect(canReturnItemsFromOrder(pending)).toBe(true);
    expect(canAddItemsToOrder(order({ settlementMode: 'PAY_BEFORE', status: 'PAID', paymentStatus: 'PAID' }))).toBe(false);
  });

  it('never allows takeout additions or changes after collection starts', () => {
    expect(canAddItemsToOrder(order({ orderType: 'TAKEOUT' }))).toBe(false);
    expect(canAddItemsToOrder(order({ paidAmount: 1 }))).toBe(false);
    expect(canAddItemsToOrder(order({ paymentStatus: 'PENDING' }))).toBe(false);
    expect(addItemsBlockedReason(order({ paymentStatus: 'PENDING' }))).toContain('正在支付');
  });

  it('honors the server capability when a payment transaction locks the bill', () => {
    expect(canAddItemsToOrder(order({ canAddItems: false }))).toBe(false);
    expect(dineInOperationBlockedReason('TRANSFER', order({ canAddItems: false }))).toContain('支付流程');
  });

  it('keeps table operations pay-after only and filters merge candidates', () => {
    expect(canOperateUnpaidPayAfterOrder(order({}))).toBe(true);
    expect(canOperateUnpaidPayAfterOrder(order({ settlementMode: 'PAY_BEFORE', status: 'PENDING_PAYMENT' }))).toBe(false);
    expect(isMergeableTableBill({
      id: 2,
      areaId: 1,
      areaName: '大厅',
      name: 'A02',
      tableCode: 'A02',
      capacity: 2,
      state: 'UNSETTLED',
      orderId: 22,
      orderStatus: 'PREPARING',
      paymentStatus: 'UNPAID',
      settlementMode: 'PAY_AFTER',
      paidCents: 0,
    }, 11)).toBe(true);
    expect(isMergeableTableBill({
      id: 3,
      areaId: 1,
      areaName: '大厅',
      name: 'A03',
      tableCode: 'A03',
      capacity: 2,
      state: 'UNSETTLED',
      orderId: 23,
      orderStatus: 'READY',
      paymentStatus: 'UNPAID',
      settlementMode: 'PAY_AFTER',
      paymentLocked: true,
    }, 11)).toBe(false);
  });
});
