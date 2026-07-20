export type Id = string | number;

export interface MerchantUser {
  id: Id;
  name: string;
  phone?: string;
  avatar?: string;
  merchantName?: string;
  storeName?: string;
  roles?: string[];
}

export type OrderStatus =
  | 'PENDING_PAYMENT'
  | 'PAID'
  | 'ACCEPTED'
  | 'PREPARING'
  | 'READY'
  | 'COMPLETED'
  | 'CLOSED'
  | 'REFUNDED'
  | 'PAYMENT_EXCEPTION';

/**
 * 订单所属经营域。店内域可以继续通过 fulfillmentType 区分桌边堂食和到店自取，
 * 外卖域则承载配送地址、骑手等后续能力。
 */
export type OrderBusinessType = 'DINE_IN' | 'DELIVERY';
export type OrderType = 'DINE_IN' | 'TAKEOUT' | 'DELIVERY';
export type PrintBusinessType = OrderType;

export interface OrderItem {
  id?: Id;
  productName: string;
  skuName?: string;
  image?: string;
  quantity: number;
  unitPrice: number;
  amount?: number;
  remark?: string;
  itemRemark?: string;
  configuration?: {
    options?: Array<{ groupName?: string; valueName?: string }>;
    modifiers?: Array<{ groupName?: string; name?: string; quantity?: number }>;
  };
}

export interface Order {
  id: Id;
  orderNo: string;
  pickupNo?: string;
  fastFoodPlatePublicId?: string;
  fastFoodPlateCode?: string;
  fastFoodPlateName?: string;
  status: OrderStatus;
  amount: number;
  paidAmount?: number;
  refundAmount?: number;
  paymentMethod?: string;
  businessType?: OrderBusinessType;
  orderType?: OrderType;
  fulfillmentType?: 'PICKUP' | 'TAKEOUT' | 'DINE_IN' | 'DELIVERY';
  tableCodeId?: Id;
  tableNo?: string;
  tableName?: string;
  tableAreaName?: string;
  customerName?: string;
  customerPhone?: string;
  remark?: string;
  createdAt: string;
  paidAt?: string;
  items: OrderItem[];
  printCount?: number;
}

export interface TableCode {
  id: Id;
  areaId: Id;
  areaName: string;
  tableNo: string;
  tableName: string;
  seats: number;
  status: 'ACTIVE' | 'DISABLED';
  publicId?: string;
  remark?: string;
  sortOrder?: number;
  /** 后端生成的、不暴露租户主键的稳定扫码参数。 */
  scene: string;
  miniappPath: string;
  qrCodeUrl?: string;
  orderCount?: number;
  lastScannedAt?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface TableArea {
  id: Id;
  name: string;
  sortOrder: number;
  status: 'ACTIVE' | 'DISABLED';
}

export interface PrintTemplateSection {
  id?: Id;
  templateType: 'RECEIPT' | 'LABEL';
  copyRole: PrintCopyRole;
  name: string;
  enabled: boolean;
  triggerEvent: 'ORDER_CREATED' | 'PAYMENT_SUCCESS';
  copies: number;
  paperWidth: 58 | 80;
  templateText: string;
  layout: PrintTemplateLayout;
  updatedAt?: string;
}

export interface BusinessPrintTemplate {
  businessType: PrintBusinessType;
  sections: Record<PrintCopyRole, PrintTemplateSection>;
}

export type PrintCopyRole = 'MERCHANT' | 'CUSTOMER' | 'KITCHEN' | 'ITEM';

/**
 * 与具体打印机厂商无关的票据布局。服务端会把它渲染成 58/80mm 定宽文本；
 * 厂商适配器接入后，同一份结构还可以映射为 ESC/POS 或云打印指令。
 */
export interface PrintTemplateLayout {
  schemaVersion: 1;
  headerStyle: 'SIMPLE' | 'PROMINENT';
  fontSize: 'NORMAL' | 'LARGE';
  showStoreName: boolean;
  showOrderType: boolean;
  showOrderNo: boolean;
  showPickupNo: boolean;
  showTable: boolean;
  showItems: boolean;
  showItemOptions: boolean;
  showPrices: boolean;
  showPayment: boolean;
  showRemark: boolean;
  showCustomer: boolean;
  showAddress: boolean;
  showQrCode: boolean;
  customHeader: string;
  customFooter: string;
}

export interface PrintTemplateRecord {
  id: Id;
  businessType: PrintBusinessType;
  templateType: 'RECEIPT' | 'LABEL';
  copyRole?: PrintCopyRole;
  name: string;
  content: string;
  triggerEvent: 'ORDER_CREATED' | 'PAYMENT_SUCCESS';
  copies: number;
  paperWidth?: 58 | 80;
  layout?: Partial<PrintTemplateLayout>;
  enabled?: boolean;
  status: 'ACTIVE' | 'DISABLED';
  updatedAt?: string;
}

export interface Category {
  id: Id;
  name: string;
  sort?: number;
  productCount?: number;
  enabled?: boolean;
}

export interface Sku {
  id?: Id;
  name: string;
  price: number;
  stock: number;
  expectedStock?: number;
  originalStock?: number;
  code?: string;
  attributes?: Record<string, string>;
}

export interface Product {
  id: Id;
  name: string;
  categoryId: Id;
  categoryName?: string;
  image?: string;
  images?: ProductImage[];
  description?: string;
  price: number;
  stock: number;
  enabled: boolean;
  recommended?: boolean;
  salesCount?: number;
  soldOut?: boolean;
  autoRestock?: boolean;
  dailyStock?: number;
  skus: Sku[];
  createdAt?: string;
}

export interface ProductImage {
  id?: Id;
  mediaAssetId?: Id;
  url: string;
  isPrimary: boolean;
  sortOrder: number;
}

export interface PaymentRecord {
  id: Id;
  orderId: Id;
  orderNo: string;
  paymentNo?: string;
  providerOrderNo?: string;
  amount: number;
  refundableAmount?: number;
  method?: string;
  status: string;
  paidAt?: string;
  createdAt?: string;
}

export interface RefundRecord {
  id: Id;
  refundNo: string;
  orderNo: string;
  amount: number;
  reason?: string;
  operatorName?: string;
  status: string;
  createdAt: string;
  completedAt?: string;
}

export interface Printer {
  id: Id;
  name: string;
  vendor?: string;
  provider?: string;
  model?: string;
  sn: string;
  type: 'VIRTUAL' | 'RECEIPT' | 'LABEL';
  status: 'ONLINE' | 'OFFLINE' | 'PAPER_OUT' | 'DISABLED';
  enabled: boolean;
  lastSeenAt?: string;
  paperWidth?: number;
  printTrigger?: 'ORDER_CREATED' | 'PAYMENT_SUCCESS';
  outputType?: 'RECEIPT' | 'LABEL';
  copyRoles?: PrintCopyRole[];
  templateText?: string;
}

export interface PrintJob {
  id: Id;
  orderNo: string;
  printerName?: string;
  type: string;
  templateId?: Id;
  copyRole?: PrintCopyRole;
  paperWidth?: 58 | 80;
  status: 'PENDING' | 'PROCESSING' | 'PRINTING' | 'SUCCESS' | 'FAILED' | 'UNKNOWN';
  retryCount?: number;
  errorMessage?: string;
  createdAt: string;
  completedAt?: string;
}

export interface Staff {
  id: Id;
  name: string;
  phone: string;
  role: string;
  roleName?: string;
  enabled: boolean;
  lastLoginAt?: string;
  createdAt?: string;
}

export interface DashboardData {
  todayRevenue: number;
  todayOrders: number;
  pendingOrders: number;
  averageOrderValue: number;
  yesterdayRevenue?: number;
  refundAmount?: number;
  revenueTrend?: Array<{ label: string; value: number }>;
  popularProducts?: Array<{ name: string; count: number; amount?: number }>;
  recentOrders?: Order[];
}

export interface MerchantSettings {
  storeName: string;
  logo?: string;
  phone?: string;
  address?: string;
  announcement?: string;
  businessHours?: [string, string];
  autoAcceptOrder?: boolean;
  orderVoiceReminder?: boolean;
  printTrigger?: 'ORDER_CREATED' | 'PAYMENT_SUCCESS';
  autoPrintReceipt?: boolean;
  autoPrintLabel?: boolean;
  pickupMode?: boolean;
  allowLatePayment?: boolean;
  paymentTimeoutMinutes?: number;
}

export interface StoreBusinessPeriod {
  id?: Id;
  start: string;
  end: string;
}

export interface StoreBusinessDay {
  weekday: number;
  periods: StoreBusinessPeriod[];
}

export interface StoreBusinessHours {
  storeId: Id;
  timezone: string;
  weeklySchedule: StoreBusinessDay[];
  businessStatus: 'OPEN' | 'CLOSED';
  businessStatusReason: string;
  businessStatusMessage: string;
  acceptingOrders: boolean;
  businessDate?: string;
  nextOpenAt?: string;
  temporaryOverride?: {
    id: Id;
    status: 'OPEN' | 'CLOSED';
    startsAt: string;
    endsAt: string;
    reason?: string;
  };
}

export interface PageMeta {
  page?: number;
  pageSize?: number;
  total?: number;
}

export interface ListResult<T> {
  items: T[];
  meta: PageMeta;
}
