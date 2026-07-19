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

export interface OrderItem {
  id?: Id;
  productName: string;
  skuName?: string;
  image?: string;
  quantity: number;
  unitPrice: number;
  amount?: number;
  remark?: string;
}

export interface Order {
  id: Id;
  orderNo: string;
  pickupNo?: string;
  status: OrderStatus;
  amount: number;
  paidAmount?: number;
  refundAmount?: number;
  paymentMethod?: string;
  fulfillmentType?: 'PICKUP' | 'DINE_IN';
  customerName?: string;
  customerPhone?: string;
  remark?: string;
  createdAt: string;
  paidAt?: string;
  items: OrderItem[];
  printCount?: number;
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
  description?: string;
  price: number;
  stock: number;
  enabled: boolean;
  soldOut?: boolean;
  autoRestock?: boolean;
  dailyStock?: number;
  skus: Sku[];
  createdAt?: string;
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
  templateText?: string;
}

export interface PrintJob {
  id: Id;
  orderNo: string;
  printerName?: string;
  type: string;
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

export interface PageMeta {
  page?: number;
  pageSize?: number;
  total?: number;
}

export interface ListResult<T> {
  items: T[];
  meta: PageMeta;
}
