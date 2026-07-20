import { beforeEach, describe, expect, it, vi } from 'vitest';
import { addCartItem, changeCartItemQuantity, clearCart, readCart, writeCart } from '../miniprogram/utils/cart';
import {
  CHECKOUT_FLOW_TTL_MS,
  checkoutBlockedByStoreStatus,
  checkoutFlowFor,
  checkoutNeedsFreshOrder,
  checkoutOrderIsClosed,
  clearCheckoutFlow,
  markCheckoutSubmitted,
  rememberCheckoutDetails,
  rememberCheckoutOrder,
} from '../miniprogram/utils/checkout';
import { ApiError } from '../miniprogram/utils/request';
import { customerGuestKey } from '../miniprogram/utils/customer';
import type { FastFoodOrderingContext, TableOrderingContext } from '../miniprogram/types/domain';
import { rememberMarketingPopup, shouldDisplayMarketingPopup } from '../miniprogram/utils/marketing';
import type { MarketingPlacement } from '../miniprogram/types/domain';

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

  it('keeps option and modifier variants as separate cart lines', () => {
    addCartItem('coffee-a', {
      productId: 8,
      skuId: 81,
      name: '拿铁',
      price: 18,
      quantity: 1,
      optionValueIds: [101],
      modifiers: [{ groupId: 3, modifierItemId: 31, quantity: 1 }],
    });
    addCartItem('coffee-a', {
      productId: 8,
      skuId: 81,
      name: '拿铁',
      price: 18,
      quantity: 1,
      optionValueIds: [102],
      modifiers: [{ groupId: 3, modifierItemId: 31, quantity: 1 }],
    });

    expect(readCart('coffee-a')).toHaveLength(2);
  });

  it('creates a fresh checkout idempotency key when configuration changes', () => {
    const base = { productId: 8, skuId: 81, name: '拿铁', price: 18, quantity: 1 };
    const first = checkoutFlowFor('coffee-a', [{ ...base, optionValueIds: [101] }]);
    const second = checkoutFlowFor('coffee-a', [{ ...base, optionValueIds: [102] }]);
    const third = checkoutFlowFor('coffee-a', [{
      ...base,
      optionValueIds: [102],
      modifiers: [{ groupId: 3, modifierItemId: 31, quantity: 1 }],
    }]);

    expect(second.idempotencyKey).not.toBe(first.idempotencyKey);
    expect(third.idempotencyKey).not.toBe(second.idempotencyKey);
  });

  it('scopes checkout idempotency and fulfillment to the verified table', () => {
    const cart = [{ productId: 1, name: '美式', price: 12, quantity: 1 }];
    const tableContext: TableOrderingContext = {
      publicScene: '0123456789abcdef0123456789ab',
      storeCode: 'coffee-a',
      storeName: '码农咖啡',
      tablePublicId: 'table-public-b02',
      tableName: 'B02桌',
      resolvedAt: Date.now(),
      validUntil: Date.now() + 60_000,
    };
    const ordinary = checkoutFlowFor('coffee-a', cart);
    const dineIn = checkoutFlowFor('coffee-a', cart, tableContext);
    const backToOrdinary = checkoutFlowFor('coffee-a', cart);

    expect(dineIn.idempotencyKey).not.toBe(ordinary.idempotencyKey);
    expect(dineIn).toMatchObject({
      fulfillmentType: 'DINE_IN',
      orderScene: 'DINE_IN',
      tablePublicId: 'table-public-b02',
    });
    expect(backToOrdinary.idempotencyKey).not.toBe(dineIn.idempotencyKey);
    expect(backToOrdinary.fulfillmentType).toBe('PICKUP');
  });

  it('persists one install-scoped anonymous customer key', () => {
    const first = customerGuestKey();
    const second = customerGuestKey();

    expect(first).toMatch(/^guest_/);
    expect(second).toBe(first);
  });

  it('scopes checkout idempotency to a verified fast-food pickup plate', () => {
    const cart = [{ productId: 1, name: '美式', price: 12, quantity: 1 }];
    const context: FastFoodOrderingContext = {
      publicId: '0123456789abcdef0123456789ab',
      storeCode: 'coffee-a',
      storeName: '码农咖啡',
      plateCode: 'K08',
      plateName: '取餐架 K08',
      resolvedAt: Date.now(),
      validUntil: Date.now() + 60_000,
    };
    const ordinary = checkoutFlowFor('coffee-a', cart);
    const fastFood = checkoutFlowFor('coffee-a', cart, null, context);

    expect(fastFood.idempotencyKey).not.toBe(ordinary.idempotencyKey);
    expect(fastFood).toMatchObject({ fulfillmentType: 'PICKUP', fastFoodPlatePublicId: context.publicId, fastFoodPlateName: context.plateName });
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

  it('persists checkout details and freezes them after the first submission', () => {
    const cart = [{ productId: 1, name: '美式', price: 12, quantity: 1 }];
    const tableContext: TableOrderingContext = {
      publicScene: '0123456789abcdef0123456789ab',
      storeCode: 'coffee-a',
      storeName: '码农咖啡',
      tablePublicId: 'table-public-b02',
      tableName: 'B02桌',
      resolvedAt: Date.now(),
      validUntil: Date.now() + 60_000,
    };
    const flow = checkoutFlowFor('coffee-a', cart, tableContext);
    rememberCheckoutDetails(flow.idempotencyKey, 'DINE_IN', '送到 B02');

    expect(checkoutFlowFor('coffee-a', cart, tableContext)).toMatchObject({
      fulfillmentType: 'DINE_IN',
      remark: '送到 B02',
      submitted: false,
    });

    markCheckoutSubmitted(flow.idempotencyKey);
    rememberCheckoutDetails(flow.idempotencyKey, 'PICKUP', '不应覆盖');
    expect(checkoutFlowFor('coffee-a', cart, tableContext)).toMatchObject({
      fulfillmentType: 'DINE_IN',
      remark: '送到 B02',
      submitted: true,
    });
  });

  it('recognizes terminal order and payment outcomes that require a fresh order', () => {
    expect(checkoutOrderIsClosed('closed')).toBe(true);
    expect(checkoutOrderIsClosed('PAID')).toBe(false);
    expect(checkoutNeedsFreshOrder(new ApiError('不可支付', 409, 'ORDER_NOT_PAYABLE'))).toBe(true);
    expect(checkoutNeedsFreshOrder(new ApiError('网络错误', 0, 'NETWORK_ERROR'))).toBe(false);
  });

  it('blocks new checkout while closed but allows an already-created order to continue payment', () => {
    expect(checkoutBlockedByStoreStatus('CLOSED', '')).toBe(true);
    expect(checkoutBlockedByStoreStatus('CLOSED', 'TB202607200001')).toBe(false);
    expect(checkoutBlockedByStoreStatus('OPEN', '')).toBe(false);
  });

  it('uses the device local calendar day for daily marketing popup frequency', () => {
    const placement: MarketingPlacement = { id: 9, name: '新人礼', placement_code: 'HOME_POPUP', image_url: '/coupon.jpg', action_type: 'OPEN_COUPONS', frequency: 'DAILY', priority: 10 };
    const firstDay = new Date(2026, 6, 20, 23, 55);
    const nextDay = new Date(2026, 6, 21, 0, 5);
    expect(shouldDisplayMarketingPopup('coffee-a', placement, firstDay)).toBe(true);
    rememberMarketingPopup('coffee-a', placement, firstDay);
    expect(shouldDisplayMarketingPopup('coffee-a', placement, firstDay)).toBe(false);
    expect(shouldDisplayMarketingPopup('coffee-a', placement, nextDay)).toBe(true);
  });
});
