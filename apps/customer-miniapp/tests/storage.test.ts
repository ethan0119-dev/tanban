import { beforeEach, describe, expect, it, vi } from 'vitest';
import { addCartItem, changeCartItemQuantity, clearCart, readCart, writeCart } from '../miniprogram/utils/cart';
import {
  CHECKOUT_FLOW_TTL_MS,
  checkoutFlowFor,
  checkoutNeedsFreshOrder,
  checkoutOrderIsClosed,
  clearCheckoutFlow,
  rememberCheckoutOrder,
} from '../miniprogram/utils/checkout';
import { ApiError } from '../miniprogram/utils/request';

describe('miniapp scoped storage', () => {
  let storage: Map<string, unknown>;

  beforeEach(() => {
    storage = new Map();
    vi.stubGlobal('wx', {
      getStorageSync: (key: string) => storage.get(key),
      setStorageSync: (key: string, value: unknown) => storage.set(key, value),
      removeStorageSync: (key: string) => storage.delete(key),
    });
  });

  it('increments and decrements the exact product and SKU line', () => {
    addCartItem('coffee-a', { productId: 8, skuId: 81, name: '拿铁', skuName: '中杯', price: 16, quantity: 1 });
    addCartItem('coffee-a', { productId: 8, skuId: 82, name: '拿铁', skuName: '大杯', price: 19, quantity: 2 });

    changeCartItemQuantity('coffee-a', 8, 81, 2);
    changeCartItemQuantity('coffee-a', 8, 82, -1);

    expect(readCart('coffee-a')).toEqual([
      { productId: 8, skuId: 81, name: '拿铁', skuName: '中杯', price: 16, quantity: 3 },
      { productId: 8, skuId: 82, name: '拿铁', skuName: '大杯', price: 19, quantity: 1 },
    ]);
  });

  it('isolates carts by store and removes the unsafe legacy cart', () => {
    storage.set('tanban_cart', [{ productId: 999, name: '旧商品', price: 1, quantity: 1 }]);
    writeCart('coffee-a', [{ productId: 1, name: '美式', price: 12, quantity: 1 }]);
    addCartItem('coffee-b', { productId: 2, name: '拿铁', price: 16, quantity: 2 });

    expect(readCart('coffee-a')).toEqual([{ productId: 1, name: '美式', price: 12, quantity: 1 }]);
    expect(readCart('coffee-b')).toEqual([{ productId: 2, name: '拿铁', price: 16, quantity: 2 }]);
    expect(storage.has('tanban_cart')).toBe(false);

    clearCart('coffee-a');
    expect(readCart('coffee-a')).toEqual([]);
    expect(readCart('coffee-b')).toHaveLength(1);
  });

  it('expires an abandoned checkout flow and generates a fresh key', () => {
    const now = 1_700_000_000_000;
    const nowSpy = vi.spyOn(Date, 'now').mockReturnValue(now);
    const cart = [{ productId: 1, name: '美式', price: 12, quantity: 1 }];
    const first = checkoutFlowFor('coffee-a', cart);
    rememberCheckoutOrder(first.idempotencyKey, 'ORDER-1');
    expect(checkoutFlowFor('coffee-a', cart).orderNo).toBe('ORDER-1');

    nowSpy.mockReturnValue(now + CHECKOUT_FLOW_TTL_MS + 1);
    const next = checkoutFlowFor('coffee-a', cart);
    expect(next.idempotencyKey).not.toBe(first.idempotencyKey);
    expect(next.orderNo).toBe('');

    clearCheckoutFlow(next.idempotencyKey);
    expect(storage.has('tanban_checkout_flow_v1')).toBe(false);
    nowSpy.mockRestore();
  });

  it('recognizes terminal order and payment outcomes that require a fresh order', () => {
    expect(checkoutOrderIsClosed('closed')).toBe(true);
    expect(checkoutOrderIsClosed('PAID')).toBe(false);
    expect(checkoutNeedsFreshOrder(new ApiError('不可支付', 409, 'ORDER_NOT_PAYABLE'))).toBe(true);
    expect(checkoutNeedsFreshOrder(new ApiError('网络错误', 0, 'NETWORK_ERROR'))).toBe(false);
  });
});
