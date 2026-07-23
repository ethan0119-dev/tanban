import type {
  BusinessPrintTemplate,
  Order,
  OrderBusinessType,
  OrderType,
  PrintBusinessType,
  PrintCopyRole,
  PrintTemplateLayout,
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
  const fastFoodPlate = record(raw.fastFoodPlate ?? raw.fast_food_plate);
  const rawItems = (raw.items ?? []) as Array<Record<string, unknown>>;
  const orderNo = value.orderNo ?? String(raw.order_no ?? '');
  const existingPickupNo = stringValue(value.pickupNo, raw.pickupCode, raw.pickup_code, raw.pickup_no).trim();
  const hasPersistedBusinessDate = Boolean(stringValue(raw.businessDate, raw.business_date));
  return {
    ...value,
    id: value.id ?? String(raw.orderId ?? raw.order_id ?? value.orderNo),
    orderNo,
    pickupNo: existingPickupNo || (!hasPersistedBusinessDate ? pickupCode(value.id ?? raw.id) : ''),
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
    fastFoodPlatePublicId: stringValue(value.fastFoodPlatePublicId, fastFoodPlate.publicId, fastFoodPlate.public_id, raw.fast_food_plate_public_id_snapshot, raw.fast_food_plate_public_id) || undefined,
    fastFoodPlateCode: stringValue(value.fastFoodPlateCode, fastFoodPlate.plateCode, fastFoodPlate.plate_code, raw.fast_food_plate_code_snapshot, raw.fast_food_plate_code) || undefined,
    fastFoodPlateName: stringValue(value.fastFoodPlateName, fastFoodPlate.plateName, fastFoodPlate.plate_name, raw.fast_food_plate_name_snapshot, raw.fast_food_plate_name) || undefined,
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
const DINE_IN_LABEL = `【桌码堂食】{{table_name}} #{{pickup_no}} 数量：{{item_sequence}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}`;
const TAKEOUT_RECEIPT = `【快餐 / 到店自取】\n订单号：{{order_no}}\n----------------\n{{items}}\n----------------\n合计：{{total_cents}} 分\n备注：{{remark}}`;
const TAKEOUT_LABEL = `【到店自取】#{{pickup_no}} 数量：{{item_sequence}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}`;
const DELIVERY_RECEIPT = `【外卖单】\n订单号：{{order_no}}\n----------------\n{{items}}\n----------------\n合计：{{total_cents}} 分\n备注：{{remark}}`;
const DELIVERY_LABEL = `【外卖】#{{pickup_no}} 数量：{{item_sequence}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}`;

export const PRINT_COPY_ROLES: PrintCopyRole[] = ['MERCHANT', 'CUSTOMER', 'KITCHEN', 'ITEM'];

const COPY_ROLE_NAMES: Record<PrintCopyRole, string> = {
  MERCHANT: '商家联',
  CUSTOMER: '顾客联',
  KITCHEN: '厨房联',
  ITEM: '商品标签',
};

function defaultLayout(copyRole: PrintCopyRole): PrintTemplateLayout {
  const kitchen = copyRole === 'KITCHEN';
  const item = copyRole === 'ITEM';
  const customer = copyRole === 'CUSTOMER';
  const copyTitle = { MERCHANT: '商', CUSTOMER: '客', KITCHEN: '厨', ITEM: '签' }[copyRole];
  return {
    schemaVersion: 1,
    preset: item || kitchen ? 'LARGE' : 'DETAILED',
    headerStyle: item ? 'SIMPLE' : 'PROMINENT',
    fontSize: kitchen || item ? 'LARGE' : 'NORMAL',
    copyTitle,
    showStoreName: !item,
    showOrderType: true,
    showOrderNo: !item,
    showOrderTime: true,
    showPickupNo: true,
    showTable: true,
    showItems: true,
    showItemSequence: item,
    showItemOptions: true,
    showPrices: !kitchen && !item,
    showPayment: !kitchen && !item,
    showRemark: true,
    showCustomer: customer,
    showAddress: customer,
    showQrCode: customer,
    showEndMarker: !item,
    endMarkerText: '',
    feedLines: item ? 0 : 3,
    labelWidthMM: 40,
    labelHeightMM: 30,
    customHeader: '',
    customFooter: customer ? '感谢光临，欢迎再次惠顾' : '',
  };
}

function defaultContent(businessType: PrintBusinessType, copyRole: PrintCopyRole): string {
  if (copyRole === 'ITEM') {
    return businessType === 'DELIVERY' ? DELIVERY_LABEL : businessType === 'TAKEOUT' ? TAKEOUT_LABEL : DINE_IN_LABEL;
  }
  const base = businessType === 'DELIVERY' ? DELIVERY_RECEIPT : businessType === 'TAKEOUT' ? TAKEOUT_RECEIPT : DINE_IN_RECEIPT;
  return `【${COPY_ROLE_NAMES[copyRole]}】\n${base}`;
}

function defaultSection(businessType: PrintBusinessType, copyRole: PrintCopyRole): PrintTemplateSection {
  const scene = businessType === 'DELIVERY' ? '外卖' : businessType === 'TAKEOUT' ? '自提' : '店内';
  return {
    templateType: copyRole === 'ITEM' ? 'LABEL' : 'RECEIPT',
    copyRole,
    name: `${scene}${COPY_ROLE_NAMES[copyRole]}`,
    enabled: copyRole === 'MERCHANT' || copyRole === 'ITEM',
    triggerEvent: 'PAYMENT_SUCCESS',
    copies: 1,
    paperWidth: 58,
    templateText: defaultContent(businessType, copyRole),
    layout: defaultLayout(copyRole),
  };
}

export function defaultPrintTemplate(businessType: PrintBusinessType): BusinessPrintTemplate {
  return {
    businessType,
    sections: Object.fromEntries(PRINT_COPY_ROLES.map((copyRole) => [copyRole, defaultSection(businessType, copyRole)])) as Record<PrintCopyRole, PrintTemplateSection>,
  };
}

function normalizePrintTemplateSection(value: PrintTemplateRecord | undefined, fallback: PrintTemplateSection): PrintTemplateSection {
  if (!value) return fallback;
  const raw = record(value);
  const status = stringValue(value.status, raw.status).toUpperCase();
  const rawLayout = value.layout ?? raw.layout ?? raw.layout_json;
  let layout = record(rawLayout);
  if (typeof rawLayout === 'string') {
    try { layout = record(JSON.parse(rawLayout)); } catch { layout = {}; }
  }
  const legacyStructuredLayout = Object.keys(layout).length > 0 && !layout.preset;
  const width = numberValue(value.paperWidth, raw.paper_width, fallback.paperWidth);
  return {
    id: value.id ?? raw.id as PrintTemplateSection['id'],
    templateType: stringValue(value.templateType, raw.template_type, fallback.templateType).toUpperCase() === 'LABEL' ? 'LABEL' : 'RECEIPT',
    copyRole: stringValue(value.copyRole, raw.copy_role, fallback.copyRole).toUpperCase() as PrintCopyRole,
    name: stringValue(value.name, raw.name, fallback.name),
    enabled: value.enabled ?? status !== 'DISABLED',
    triggerEvent: stringValue(value.triggerEvent, raw.trigger_event).toUpperCase() === 'ORDER_CREATED' ? 'ORDER_CREATED' : 'PAYMENT_SUCCESS',
    copies: Math.min(5, Math.max(1, numberValue(value.copies, raw.copies, 1))),
    paperWidth: width === 80 ? 80 : 58,
    templateText: stringValue(value.content, raw.content, raw.content_text, fallback.templateText),
    layout: { ...fallback.layout, ...layout, preset: legacyStructuredLayout ? 'CUSTOM' : layout.preset ?? fallback.layout.preset, schemaVersion: 1 } as PrintTemplateLayout,
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
      copyRole: (stringValue(item.copyRole, raw.copy_role)
        || (stringValue(item.templateType, raw.template_type).toUpperCase() === 'LABEL' ? 'ITEM' : 'MERCHANT')).toUpperCase() as PrintCopyRole,
    };
  }).filter((item) => item.businessType === businessType);
  return {
    businessType,
    sections: Object.fromEntries(PRINT_COPY_ROLES.map((copyRole) => [
      copyRole,
      normalizePrintTemplateSection(normalized.find((item) => item.copyRole === copyRole), fallback.sections[copyRole]),
    ])) as Record<PrintCopyRole, PrintTemplateSection>,
  };
}

export function printTemplatePayload(value: BusinessPrintTemplate, copyRole: PrintCopyRole) {
  const section = value.sections[copyRole];
  return {
    businessType: value.businessType,
    templateType: section.templateType,
    copyRole,
    name: section.name,
    content: section.templateText,
    triggerEvent: section.triggerEvent,
    copies: section.copies,
    paperWidth: section.paperWidth,
    layout: section.layout,
    enabled: section.enabled,
  };
}
