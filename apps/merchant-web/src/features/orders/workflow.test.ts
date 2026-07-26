import { describe, expect, it } from 'vitest';
import { canRunOrderWorkflowAction, nextOrderWorkflowAction, orderWorkflowActionText } from './workflow';

describe('order workflow', () => {
  it('keeps cashier and order management on one production path', () => {
    expect(nextOrderWorkflowAction.PAID).toEqual({ status: 'PREPARING', text: '开始制作' });
    expect(nextOrderWorkflowAction.ACCEPTED).toEqual({ status: 'PREPARING', text: '开始制作' });
    expect(nextOrderWorkflowAction.PREPARING).toEqual({ status: 'READY', text: '完成制作 / 请取餐' });
    expect(nextOrderWorkflowAction.READY).toEqual({ status: 'COMPLETED', text: '完成订单' });
  });

  it('requires pay-after orders to settle before completion', () => {
    expect(canRunOrderWorkflowAction('READY', 'PAY_AFTER', 'UNPAID')).toBe(false);
    expect(canRunOrderWorkflowAction('READY', 'PAY_AFTER', 'PAID')).toBe(true);
    expect(canRunOrderWorkflowAction('PREPARING', 'PAY_AFTER', 'UNPAID')).toBe(true);
  });

  it('uses clear-table copy only for dine-in completion', () => {
    expect(orderWorkflowActionText('READY', 'DINE_IN')).toBe('完成订单/清台');
    expect(orderWorkflowActionText('READY', 'TAKEOUT')).toBe('完成订单');
    expect(orderWorkflowActionText('PREPARING', 'DINE_IN')).toBe('完成制作 / 请取餐');
  });
});
