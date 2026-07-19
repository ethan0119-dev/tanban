import type {
  BusinessPrintTemplate,
  Order,
  OrderBusinessType,
  OrderType,
  PrintBusinessType,
  PrintTemplateRecord,
  PrintTemplateSection,
  TableCode,
  TableArea,
} from '../../types';
import { pickupCode } from '../../utils/format';

export const TABLE_CODES_ENDPOINT = '/merchant/table-codes';
export const TABLE_AREAS_ENDPOINT = '/merchant/table-areas';
export const PRINT_TEMPLATES_ENDPOINT = '/merchant/print-templates';

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {};
}

function stringValue(...values: unknown[]): string {
  const value = values.find((item) => item !== undefined && item !== null && String(item).trim());
  return value === undefined ? '' : String(value);
}

function numberValue(...values: unknown[]): number {
  const value = values.find((item) => item !== undefined && item !== null && item !== '');
  const parsed = Number(value ?? 0);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function inferOrderBusinessType(value: unknown): OrderBusinessType {
  const raw = record(value);
  const explicit = stringValue(raw.businessType, raw.business_type, raw.orderType, raw.order_type).toUpperCase();
  if (explicit === 'DELIVERY') return 'DELIVERY';
  if (['DINE_IN', 'TAKEOUT', 'TAKEAWAY', 'TAKE_OUT', 'PICKUP', 'IN_STORE', 'STORE'].includes(explicit)) return 'DINE_IN';
  return stringValue(raw.fulfillmentType, raw.fulfillment_type).toUpperCase() === 'DELIVERY' ? 'DELIVERY' : 'DINE_IN';
}

export function inferOrderType(value: unknown): OrderType {
  const raw = record(value);
  const explicit = stringValue(raw.orderType, raw.order_type).toUpperCase();
  if (explicit === 'DELIVERY') return 'DELIVERY';
  if (explicit === 'TAKEOUT' || explicit === 'TAKEAWAY' || explicit === 'TAKE_OUT' || explicit === 'PICKUP') return 'TAKEOUT';
  const fulfillment = stringValue(raw.fulfillmentType, raw.fulfillment_type).toUpperCase();
  if (fulfillment === 'DELIVERY') return 'DELIVERY';
  if (fulfillment === 'PICKUP' || fulfillment === 'TAKEOUT') return 'TAKEOUT';
  return 'DINE_IN';
}

export function normalizeOrder(value: Order): Order {
  const raw = record(value);
  const table = record(raw.table);
  const rawItems = (raw.items ?? []) as Array<Record<string, unknown>>;
  const orderNo = value.orderNo ?? String(raw.order_no ?? '');
  const existingPickupNo = stringValue(value.pickupNo, raw.pickup_no).trim();
  return {
    ...value,
    id: value.id ?? String(raw.orderId ?? raw.order_id ?? value.orderNo),
    orderNo,
    pickupNo: existingPickupNo || pickupCode(value.id ?? raw.id),
    amount: raw.total_cents !== undefined ? Number(raw.total_cents) / 100 : numberValue(value.amount, raw.totalAmount, raw.total_amount),
    paidAmount: raw.paid_cents !== undefined ? Number(raw.paid_cents) / 100 : numberValue(value.paidAmount, raw.paid_amount, value.amount),
    refundAmount: raw.refunded_cents !== undefined ? Number(raw.refunded_cents) / 100 : numberValue(value.refundAmount, raw.refund_amount),
    customerName: value.customerName ?? String(raw.customer_name ?? ''),
    customerPhone: value.customerPhone ?? String(raw.customer_phone ?? ''),
    businessType: inferOrderBusinessType(value),
    orderType: inferOrderType(value),
    fulfillmentType: (value.fulfillmentType ?? raw.fulfillment_type ?? 'PICKUP') as Order['fulfillmentType'],
    tableCodeId: value.tableCodeId ?? table.id as Order['tableCodeId'] ?? raw.table_id as Order['tableCodeId'] ?? raw.table_code_id as Order['tableCodeId'],
    tableNo: stringValue(value.tableNo, table.tableCode, table.table_code, raw.table_code_snapshot, raw.table_no),
    tableName: stringValue(value.tableName, table.name, raw.table_name_snapshot, raw.table_name),
    tableAreaName: stringValue(value.tableAreaName, table.areaName, table.area_name, raw.table_area_name_snapshot, raw.table_area_name, raw.area_name),
    paidAt: value.paidAt ?? (raw.paid_at ? String(raw.paid_at) : undefined),
    createdAt: value.createdAt ?? String(raw.created_at ?? ''),
    items: rawItems.map((item) => ({
      id: item.id as string | number,
      productName: stringValue(item.productName, item.product_name),
      skuName: stringValue(item.skuName, item.sku_name),
      quantity: numberValue(item.quantity),
      unitPrice: item.unit_price_cents !== undefined ? Number(item.unit_price_cents) / 100 : numberValue(item.unitPrice),
      amount: item.subtotal_cents !== undefined ? Number(item.subtotal_cents) / 100 : numberValue(item.amount),
      remark: item.remark ? String(item.remark) : undefined,
      itemRemark: item.item_remark ? String(item.item_remark) : undefined,
      configuration: item.configuration && typeof item.configuration === 'object'
        ? item.configuration as Order['items'][number]['configuration']
        : undefined,
    })),
  };
}

export function normalizeTableCode(value: TableCode): TableCode {
  const raw = record(value);
  const rawStatus = stringValue(value.status, raw.status, raw.enabled === false ? 'DISABLED' : 'ACTIVE').toUpperCase();
  return {
    ...value,
    id: value.id ?? stringValue(raw.table_code_id, raw.id),
    areaId: value.areaId ?? raw.areaId as TableCode['areaId'] ?? raw.area_id as TableCode['areaId'],
    areaName: stringValue(value.areaName, raw.area_name, '默认区域'),
    tableNo: stringValue(value.tableNo, raw.tableCode, raw.table_code, raw.table_no, raw.code),
    tableName: stringValue(value.tableName, raw.name, raw.table_name, value.tableNo, raw.tableCode, raw.table_no),
    seats: numberValue(value.seats, raw.capacity, raw.seat_count),
    status: rawStatus === 'DISABLED' || rawStatus === 'INACTIVE' ? 'DISABLED' : 'ACTIVE',
    publicId: stringValue(value.publicId, raw.publicId, raw.public_id) || undefined,
    remark: stringValue(value.remark, raw.remark) || undefined,
    sortOrder: numberValue(value.sortOrder, raw.sortOrder, raw.sort_order),
    scene: stringValue(value.scene, raw.qrScene, raw.qr_scene, raw.scene, raw.scene_token, raw.publicId, raw.public_id),
    miniappPath: stringValue(value.miniappPath, raw.miniappPath, raw.miniapp_path, 'pages/menu/index'),
    qrCodeUrl: stringValue(value.qrCodeUrl, raw.qr_code_url) || undefined,
    orderCount: numberValue(value.orderCount, raw.order_count),
    lastScannedAt: stringValue(value.lastScannedAt, raw.last_scanned_at) || undefined,
    createdAt: stringValue(value.createdAt, raw.created_at) || undefined,
    updatedAt: stringValue(value.updatedAt, raw.updated_at) || undefined,
  };
}

export function normalizeTableArea(value: TableArea): TableArea {
  const raw = record(value);
  const rawStatus = stringValue(value.status, raw.status).toUpperCase();
  return {
    id: value.id ?? raw.id as TableArea['id'],
    name: stringValue(value.name, raw.name),
    sortOrder: numberValue(value.sortOrder, raw.sort_order),
    status: rawStatus === 'DISABLED' || rawStatus === 'INACTIVE' ? 'DISABLED' : 'ACTIVE',
  };
}

const DINE_IN_RECEIPT = `【桌码堂食】\n桌台：{{table_area}} {{table_name}}（{{table_code}}）\n订单号：{{order_no}}\n----------------\n{{items}}\n----------------\n合计：{{total_cents}} 分\n备注：{{remark}}`;
const DINE_IN_LABEL = `【桌码堂食】{{table_name}} #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}`;
const TAKEOUT_RECEIPT = `【快餐 / 到店自取】\n订单号：{{order_no}}\n----------------\n{{items}}\n----------------\n合计：{{total_cents}} 分\n备注：{{remark}}`;
const TAKEOUT_LABEL = `【到店自取】#{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}`;
const DELIVERY_RECEIPT = `【外卖单】\n订单号：{{order_no}}\n----------------\n{{items}}\n----------------\n合计：{{total_cents}} 分\n备注：{{remark}}`;
const DELIVERY_LABEL = `【外卖】#{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}`;

export function defaultPrintTemplate(businessType: PrintBusinessType): BusinessPrintTemplate {
  const delivery = businessType === 'DELIVERY';
  const takeout = businessType === 'TAKEOUT';
  return {
    businessType,
    receipt: { name: `${takeout ? '自提' : delivery ? '外卖' : '店内'}小票`, enabled: true, triggerEvent: 'PAYMENT_SUCCESS', copies: 1, templateText: delivery ? DELIVERY_RECEIPT : takeout ? TAKEOUT_RECEIPT : DINE_IN_RECEIPT },
    label: { name: `${takeout ? '自提' : delivery ? '外卖' : '店内'}标签`, enabled: true, triggerEvent: 'PAYMENT_SUCCESS', copies: 1, templateText: delivery ? DELIVERY_LABEL : takeout ? TAKEOUT_LABEL : DINE_IN_LABEL },
  };
}

function normalizePrintTemplateSection(value: PrintTemplateRecord | undefined, fallback: PrintTemplateSection): PrintTemplateSection {
  if (!value) return fallback;
  const raw = record(value);
  const status = stringValue(value.status, raw.status).toUpperCase();
  return {
    id: value.id ?? raw.id as PrintTemplateSection['id'],
    name: stringValue(value.name, raw.name, fallback.name),
    enabled: value.enabled ?? status !== 'DISABLED',
    triggerEvent: stringValue(value.triggerEvent, raw.trigger_event).toUpperCase() === 'ORDER_CREATED' ? 'ORDER_CREATED' : 'PAYMENT_SUCCESS',
    copies: Math.min(5, Math.max(1, numberValue(value.copies, raw.copies, 1))),
    templateText: stringValue(value.content, raw.content, raw.content_text, fallback.templateText),
    updatedAt: stringValue(value.updatedAt, raw.updated_at) || undefined,
  };
}

export function normalizePrintTemplates(values: PrintTemplateRecord[], businessType: PrintBusinessType): BusinessPrintTemplate {
  const fallback = defaultPrintTemplate(businessType);
  const normalized = values.map((item) => {
    const raw = record(item);
    return {
      ...item,
      businessType: stringValue(item.businessType, raw.business_type) as PrintBusinessType,
      templateType: stringValue(item.templateType, raw.template_type).toUpperCase() as PrintTemplateRecord['templateType'],
    };
  }).filter((item) => item.businessType === businessType);
  return {
    businessType,
    receipt: normalizePrintTemplateSection(normalized.find((item) => item.templateType === 'RECEIPT'), fallback.receipt),
    label: normalizePrintTemplateSection(normalized.find((item) => item.templateType === 'LABEL'), fallback.label),
  };
}

export function printTemplatePayload(value: BusinessPrintTemplate, templateType: PrintTemplateRecord['templateType']) {
  const section = templateType === 'RECEIPT' ? value.receipt : value.label;
  return {
    businessType: value.businessType,
    templateType,
    name: section.name,
    content: section.templateText,
    triggerEvent: section.triggerEvent,
    copies: section.copies,
    enabled: section.enabled,
  };
}
