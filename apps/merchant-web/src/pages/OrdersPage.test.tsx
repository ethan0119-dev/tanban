import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Order } from '../types';
import { OrderWorkCard } from './OrdersPage';

function takeoutOrder(overrides: Partial<Order> = {}): Order {
  return {
    id: 1,
    orderNo: 'T202608020001',
    pickupNo: '0003',
    fastFoodPlateCode: 'A03',
    fastFoodPlateName: '收银台右侧',
    status: 'READY',
    paymentStatus: 'PAID',
    settlementMode: 'PAY_BEFORE',
    amount: 16,
    paidAmount: 16,
    orderType: 'TAKEOUT',
    createdAt: '2026-08-02 10:09:22',
    items: [{ productName: '经典美式', quantity: 1, unitPrice: 16 }],
    ...overrides,
  };
}

describe('order work card', () => {
  afterEach(() => cleanup());

  it('uses the formerly empty takeout badge for the pickup number', () => {
    const onOpen = vi.fn();
    const order = takeoutOrder();
    const view = render(<OrderWorkCard order={order} onOpen={onOpen} />);

    const pickupBadge = screen.getByLabelText('取餐号 0003');
    expect(pickupBadge.classList.contains('order-work-card-icon')).toBe(true);
    expect(pickupBadge.textContent).toContain('取餐号');
    expect(pickupBadge.textContent).toContain('0003');
    expect(screen.getByText('收银台右侧')).toBeTruthy();
    expect(screen.getByText('码牌 A03 · 到店自取')).toBeTruthy();
    expect(view.container.querySelector('.order-work-card-service')?.textContent).not.toContain('0003');
    const actionButton = view.container.querySelector('.order-work-card button');
    expect(actionButton).toBeTruthy();
    fireEvent.click(actionButton!);
    expect(onOpen).toHaveBeenCalledWith(order);
  });

  it('keeps clear fallback copy before a pickup number or plate is assigned', () => {
    render(<OrderWorkCard order={takeoutOrder({ pickupNo: '', fastFoodPlateCode: '', fastFoodPlateName: '' })} onOpen={vi.fn()} />);

    expect(screen.getByLabelText('取餐号 未生成').textContent).toContain('--');
    expect(screen.getByText('到店自取')).toBeTruthy();
    expect(screen.getByText('未指定码牌')).toBeTruthy();
  });
});
