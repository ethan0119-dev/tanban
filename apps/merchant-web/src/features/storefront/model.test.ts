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

  it('keeps dine-in and delivery print templates independent', () => {
    expect(defaultPrintTemplate('DINE_IN').receipt.templateText).toContain('{{table_name}}');
    expect(defaultPrintTemplate('TAKEOUT').receipt.templateText).toContain('{{order_no}}');
    expect(defaultPrintTemplate('DELIVERY').receipt.templateText).toContain('{{total_cents}}');

    const normalized = normalizePrintTemplates([{
      id: 1,
      businessType: 'DINE_IN',
      templateType: 'RECEIPT',
      name: '堂食小票',
      content: '桌台 {{table_name}}',
      triggerEvent: 'PAYMENT_SUCCESS',
      copies: 2,
      status: 'ACTIVE',
    }, {
      id: 2,
      businessType: 'DINE_IN',
      templateType: 'LABEL',
      name: '堂食标签',
      content: '标签',
      triggerEvent: 'ORDER_CREATED',
      copies: 1,
      status: 'DISABLED',
    }], 'DINE_IN');
    expect(normalized.receipt.name).toBe('堂食小票');
    expect(normalized.receipt.copies).toBe(2);
    expect(normalized.receipt.templateText).toContain('{{table_name}}');
    expect(normalized.label.enabled).toBe(false);
    expect(printTemplatePayload(normalized, 'RECEIPT')).toMatchObject({ businessType: 'DINE_IN', templateType: 'RECEIPT', triggerEvent: 'PAYMENT_SUCCESS', copies: 2, enabled: true });
  });
});
