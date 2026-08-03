import { describe, expect, it } from 'vitest';
import {
  defaultPrintTemplate,
  inferOrderBusinessType,
  inferOrderType,
  normalizeOrder,
  normalizePrintTemplates,
  normalizeTableCode,
  printTemplatePayload,
} from './model';

describe('storefront domain normalization', () => {
  it('separates delivery orders while keeping legacy pickup orders in the dine-in domain', () => {
    expect(inferOrderBusinessType({ business_type: 'DELIVERY' })).toBe('DELIVERY');
    expect(inferOrderBusinessType({ order_type: 'TAKEOUT' })).toBe('DINE_IN');
    expect(inferOrderBusinessType({ fulfillment_type: 'DELIVERY' })).toBe('DELIVERY');
    expect(inferOrderBusinessType({ fulfillment_type: 'PICKUP' })).toBe('DINE_IN');
    expect(inferOrderBusinessType({ fulfillment_type: 'DINE_IN' })).toBe('DINE_IN');
    expect(inferOrderType({ order_type: 'TAKEOUT' })).toBe('TAKEOUT');
  });

  it('normalizes table context from an order response', () => {
    const order = normalizeOrder({
      id: 1,
      orderNo: '',
      status: 'PAID',
      amount: 0,
      createdAt: '',
      items: [],
      order_no: 'TB20260720001',
      total_cents: 1290,
      business_type: 'DINE_IN',
      table: { id: 9, tableCode: 'B02', name: 'B02 桌', areaName: '露台' },
    } as never);
    expect(order.amount).toBe(12.9);
    expect(order.businessType).toBe('DINE_IN');
    expect(order.tableCodeId).toBe(9);
    expect(order.tableName).toBe('B02 桌');
    expect(order.tableAreaName).toBe('露台');
  });

  it('keeps paid and remaining amounts separate for a split settlement', () => {
    const order = normalizeOrder({
      id: 2,
      orderNo: 'TB-SPLIT',
      status: 'PREPARING',
      amount: 0,
      createdAt: '',
      items: [],
      total_cents: 3600,
      paid_cents: 1200,
      remaining_cents: 2400,
    } as never);
    expect(order.amount).toBe(36);
    expect(order.paidAmount).toBe(12);
    expect(order.remainingAmount).toBe(24);
  });

  it('converts the public order response from cents without losing product names', () => {
    const order = normalizeOrder({
      id: 78,
      orderNo: 'TB20260803103517D2D85D63',
      pickupCode: '0001',
      businessDate: '2026-08-03',
      status: 'PENDING_PAYMENT',
      paymentStatus: 'UNPAID',
      settlementMode: 'PAY_BEFORE',
      fulfillmentType: 'PICKUP',
      orderType: 'TAKEOUT',
      amount: 6000,
      paidAmount: 0,
      remainingAmount: 6000,
      refundedAmount: 0,
      memberDiscount: 0,
      createdAt: '2026-08-03 10:35:17',
      items: [
        { id: 112, name: '经典美式', skuName: '大杯', price: 1500, originalPrice: 1500, quantity: 1, amount: 1500 },
        { id: 113, name: '经典拿铁', skuName: '标准杯', price: 1600, originalPrice: 1600, quantity: 2, amount: 3200 },
        { id: 114, name: '生椰拿铁', skuName: '标准杯', price: 1800, originalPrice: 1800, quantity: 1, amount: 1800 },
      ],
    } as never);

    expect(order.amount).toBe(60);
    expect(order.remainingAmount).toBe(60);
    expect(order.items).toEqual(expect.arrayContaining([
      expect.objectContaining({ productName: '经典美式', skuName: '大杯', unitPrice: 15, amount: 15 }),
      expect.objectContaining({ productName: '经典拿铁', skuName: '标准杯', unitPrice: 16, amount: 32 }),
      expect.objectContaining({ productName: '生椰拿铁', skuName: '标准杯', unitPrice: 18, amount: 18 }),
    ]));
    expect(normalizeOrder(order).amount).toBe(60);
  });

  it('prefers explicit cent fields in the hardened public order contract', () => {
    const order = normalizeOrder({
      id: 79,
      orderNo: 'TB-PUBLIC-CENTS',
      status: 'PENDING_PAYMENT',
      amount: 999999,
      amountCents: 2580,
      paidAmount: 999999,
      paidAmountCents: 300,
      remainingAmount: 999999,
      remainingAmountCents: 2280,
      items: [
        { id: 115, name: '拿铁', price: 999999, priceCents: 1290, quantity: 2, amount: 999999, amountCents: 2580 },
      ],
    } as never);

    expect(order).toMatchObject({ amount: 25.8, paidAmount: 3, remainingAmount: 22.8 });
    expect(order.items[0]).toMatchObject({ productName: '拿铁', unitPrice: 12.9, amount: 25.8 });
  });

  it('normalizes a snake-case table-code response and defaults to a stable miniapp path', () => {
    const table = normalizeTableCode({
      id: 3,
      areaId: 8,
      areaName: '一楼',
      tableCode: 'A03',
      name: '靠窗桌',
      capacity: 4,
      qrScene: 'tb_opaque_token',
      status: 'ACTIVE',
    } as never);
    expect(table).toMatchObject({
      areaName: '一楼',
      tableNo: 'A03',
      tableName: '靠窗桌',
      seats: 4,
      scene: 'tb_opaque_token',
      miniappPath: 'pages/menu/index',
      status: 'ACTIVE',
    });
  });

  it('normalizes the stable pickup code and fast-food plate snapshot', () => {
    const order = normalizeOrder({
      id: 81,
      orderNo: 'TB81',
      status: 'PAID',
      amount: 18,
      createdAt: '2026-07-20T10:00:00Z',
      items: [],
      order_type: 'TAKEOUT',
      business_date: '2026-07-20',
      pickup_code: '0038',
      fast_food_plate: { public_id: 'plate-public', plate_code: 'K08', plate_name: '取餐架 K08' },
    } as never);
    expect(order.pickupNo).toBe('0038');
    expect(order.fastFoodPlateName).toBe('取餐架 K08');
    expect(order.fastFoodPlateCode).toBe('K08');
  });

  it('does not invent an ID-based pickup code for a migrated order', () => {
    const order = normalizeOrder({
      id: 81,
      orderNo: 'TB81',
      status: 'PAID',
      amount: 18,
      createdAt: '2026-07-20T10:00:00Z',
      items: [],
      order_type: 'TAKEOUT',
      business_date: '2026-07-20',
      pickup_code: '',
    } as never);
    expect(order.pickupNo).toBe('');
  });

  it('keeps dine-in and delivery print templates independent', () => {
    expect(defaultPrintTemplate('DINE_IN').sections.MERCHANT.templateText).toContain('{{table_name}}');
    expect(defaultPrintTemplate('TAKEOUT').sections.MERCHANT.templateText).toContain('{{order_no}}');
    expect(defaultPrintTemplate('DELIVERY').sections.MERCHANT.templateText).toContain('{{total_cents}}');
    expect(defaultPrintTemplate('DINE_IN').sections.CUSTOMER.enabled).toBe(false);
    expect(defaultPrintTemplate('DINE_IN').sections.KITCHEN.layout.showPrices).toBe(false);
    expect(defaultPrintTemplate('DINE_IN').sections.ITEM.layout).toMatchObject({
      preset: 'LARGE',
      showItemSequence: true,
      showOrderType: true,
      showOrderNo: false,
      labelWidthMM: 40,
      labelHeightMM: 30,
    });
    expect(defaultPrintTemplate('DINE_IN').sections.MERCHANT.layout).toMatchObject({
      copyTitle: '商',
      showItemHeader: true,
      showOptionGroupNames: false,
      emphasizePaid: true,
      showEndMarker: true,
      feedLines: 3,
    });

    const normalized = normalizePrintTemplates([{
      id: 1,
      businessType: 'DINE_IN',
      templateType: 'RECEIPT',
      copyRole: 'MERCHANT',
      name: '堂食小票',
      content: '桌台 {{table_name}}',
      triggerEvent: 'PAYMENT_SUCCESS',
      copies: 2,
      paperWidth: 80,
      layout: { schemaVersion: 1, headerStyle: 'SIMPLE', showQrCode: true },
      status: 'ACTIVE',
    }, {
      id: 2,
      businessType: 'DINE_IN',
      templateType: 'LABEL',
      copyRole: 'ITEM',
      name: '堂食标签',
      content: '标签',
      triggerEvent: 'ORDER_CREATED',
      copies: 1,
      status: 'DISABLED',
    }], 'DINE_IN');
    expect(normalized.sections.MERCHANT.name).toBe('堂食小票');
    expect(normalized.sections.MERCHANT.copies).toBe(2);
    expect(normalized.sections.MERCHANT.paperWidth).toBe(80);
    expect(normalized.sections.MERCHANT.layout.showQrCode).toBe(true);
    expect(normalized.sections.MERCHANT.layout.showItemHeader).toBe(true);
    expect(normalized.sections.MERCHANT.layout.showOptionGroupNames).toBe(false);
    expect(normalized.sections.MERCHANT.layout.emphasizePaid).toBe(true);
    expect(normalized.sections.MERCHANT.templateText).toContain('{{table_name}}');
    expect(normalized.sections.ITEM.enabled).toBe(false);
    expect(normalized.sections.MERCHANT.layout.preset).toBe('CUSTOM');
    expect(printTemplatePayload(normalized, 'MERCHANT')).toMatchObject({ businessType: 'DINE_IN', templateType: 'RECEIPT', copyRole: 'MERCHANT', triggerEvent: 'PAYMENT_SUCCESS', copies: 2, paperWidth: 80, enabled: true });
  });
});
