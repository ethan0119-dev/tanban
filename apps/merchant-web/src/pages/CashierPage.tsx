/* eslint-disable @next/next/no-img-element -- product images are supplied by each merchant */
import {
  AppstoreOutlined,
  ArrowLeftOutlined,
  BellOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CoffeeOutlined,
  DashboardOutlined,
  DollarOutlined,
  LogoutOutlined,
  LockOutlined,
  MenuOutlined,
  MergeCellsOutlined,
  MinusOutlined,
  MoreOutlined,
  PlusOutlined,
  PrinterOutlined,
  QrcodeOutlined,
  ReloadOutlined,
  RetweetOutlined,
  ShoppingOutlined,
  ShopOutlined,
  ShoppingCartOutlined,
  TableOutlined,
  TeamOutlined,
  UserOutlined,
  WalletOutlined,
  WechatOutlined,
  WifiOutlined,
} from '@ant-design/icons';
import {
  App as AntApp,
  Alert,
  Badge,
  Button,
  Checkbox,
  Drawer,
  Dropdown,
  Empty,
  Input,
  InputNumber,
  Modal,
  Radio,
  Segmented,
  Select,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import { BrowserMultiFormatReader, type IScannerControls } from '@zxing/browser';
import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, ApiError, CASHIER_TOKEN_KEY, errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { orderStatusMap } from '../components/OrderStatusTag';
import {
  addItemsBlockedReason,
  canAddItemsToOrder,
  canOperateUnpaidPayAfterOrder,
  canReturnItemsFromOrder,
  dineInOperationBlockedReason,
  isMergeableTableBill,
} from '../features/orders/editability';
import { canRunOrderWorkflowAction, nextOrderWorkflowAction, orderWorkflowActionText } from '../features/orders/workflow';
import { normalizeOrder } from '../features/storefront/model';
import type {
  DashboardData,
  MerchantSettings,
  Order,
  OrderItem,
  OrderReturnRequest,
  PaymentRecord,
  TableBoardResponse,
  TableBoardTable,
} from '../types';
import { dateTime, yuan } from '../utils/format';
import '../cashier.css';

interface CashierContext {
  storeId: string | number;
  storeCode: string;
  storeName: string;
  logoUrl?: string;
  operatorName: string;
  role: string;
  paymentProvider?: string;
  wechatCodePaymentEnabled?: boolean;
  wechatCodePaymentReason?: string;
}

interface WechatCodePaymentResult {
  paymentId: number;
  providerOrderNo?: string;
  providerTransactionNo?: string;
  paymentMethod?: string;
  status: 'CREATING' | 'PENDING' | 'SUCCESS' | 'FAILED' | 'CLOSED';
  errorCode?: string;
  message?: string;
  needCustomerAction?: boolean;
  retryAfterSeconds?: number;
}

interface CatalogSku {
  id: number;
  name: string;
  price: number;
  stock: number;
  soldOut: boolean;
}

interface CatalogOptionValue {
  id: number;
  name: string;
  priceDeltaCents: number;
  isDefault: boolean;
}

interface CatalogOptionGroup {
  id: number;
  name: string;
  selectionMode: 'SINGLE' | 'MULTIPLE';
  minSelect: number;
  maxSelect: number;
  values: CatalogOptionValue[];
}

interface CatalogModifierItem {
  id: number;
  name: string;
  priceCents: number;
  isDefault: boolean;
}

interface CatalogModifierGroup {
  id: number;
  name: string;
  minSelect: number;
  maxSelect: number;
  items: CatalogModifierItem[];
}

interface CatalogProduct {
  id: number;
  categoryId: number;
  name: string;
  description?: string;
  imageUrl?: string;
  price: number;
  stock: number;
  soldOut: boolean;
  skus: CatalogSku[];
  optionGroups: CatalogOptionGroup[];
  modifierGroups: CatalogModifierGroup[];
}

interface CashierCatalog {
  categories: Array<{ id: number; name: string }>;
  products: CatalogProduct[];
}

interface CartLine {
  key: string;
  productId: number;
  skuId: number;
  name: string;
  skuName: string;
  priceCents: number;
  quantity: number;
  optionValueIds: number[];
  modifiers: Array<{ groupId: number; modifierItemId: number; quantity: number }>;
  summary: string;
  itemRemark: string;
}

type CashierMode = 'DINE_IN' | 'TAKEOUT';
type CashierLayoutMode = 'COMPACT' | 'STANDARD';
type CashierTableFilter = 'ALL' | 'UNSETTLED' | 'SETTLED' | 'OVERDUE';

const CASHIER_LAYOUT_MODE_KEY = 'tanban_cashier_layout_mode';
const returnReasonOptions = [
  '顾客点错',
  '顾客临时取消',
  '重复下单',
  '菜品尚未制作',
  '缺货无法制作',
  '菜品质量问题',
];

const tableMeta: Record<TableBoardTable['state'], { label: string; className: string }> = {
  UNOPENED: { label: '空闲', className: 'is-free' },
  PENDING_PAYMENT: { label: '待付款', className: 'is-pending-payment' },
  SETTLED: { label: '已结账', className: 'is-settled' },
  DINING: { label: '制作中', className: 'is-dining' },
  READY: { label: '待清台', className: 'is-ready' },
  UNSETTLED: { label: '待结账', className: 'is-unsettled' },
};

const demoOrder: Order = {
  id: 5031,
  orderNo: 'D260725150041',
  status: 'READY',
  paymentStatus: 'UNPAID',
  settlementMode: 'PAY_AFTER',
  additionCount: 3,
  canAddItems: true,
  dinerCount: 3,
  amount: 132,
  remainingAmount: 132,
  paidAmount: 0,
  memberDiscount: 12,
  orderType: 'DINE_IN',
  tableCodeId: 5,
  tableNo: 'B03',
  tableName: 'B03',
  tableAreaName: '大厅',
  createdAt: '2026-07-25 14:41:00',
  items: [
    { id: 1, productName: '水煮牛肉', quantity: 1, unitPrice: 48, amount: 48, additionSequence: 1 },
    { id: 2, productName: '鱼香肉丝', quantity: 1, unitPrice: 22, amount: 22, additionSequence: 1 },
    { id: 3, productName: '清炒时蔬', quantity: 1, unitPrice: 16, amount: 16, additionSequence: 1 },
    { id: 4, productName: '麻婆豆腐', quantity: 1, unitPrice: 18, amount: 18, additionSequence: 2 },
    { id: 5, productName: '米饭', quantity: 1, unitPrice: 8, amount: 8, additionSequence: 3 },
  ],
};

function fixtureTable(id: number, name: string, capacity: number, state: TableBoardTable['state'], order?: Partial<Order>): TableBoardTable {
  return {
    id,
    publicId: `table-public-${id}`,
    areaId: id < 9 ? 1 : 2,
    areaName: id < 9 ? '大厅' : '包间',
    name,
    tableCode: name,
    capacity,
    state,
    orderId: order?.id,
    orderNo: order?.orderNo,
    orderStatus: order?.status,
    paymentStatus: order?.paymentStatus,
    settlementMode: order?.settlementMode,
    additionCount: order?.additionCount,
    dinerCount: order?.dinerCount,
    totalCents: order?.amount ? order.amount * 100 : undefined,
    openedAt: order ? '2026-07-25 14:41:00' : undefined,
  };
}

const previewBoard: TableBoardResponse = {
  settlementMode: 'PAY_AFTER',
  orderingMode: 'MULTI_PERSON',
  areas: [
    {
      id: 1,
      name: '大厅',
      tables: [
        fixtureTable(1, 'A01', 4, 'UNOPENED'),
        fixtureTable(2, 'A02', 2, 'READY', { id: 5028, orderNo: 'D260725151122', status: 'READY', paymentStatus: 'PAID', settlementMode: 'PAY_BEFORE', dinerCount: 2 }),
        fixtureTable(3, 'A03', 4, 'UNOPENED'),
        fixtureTable(4, 'A05', 4, 'UNOPENED'),
        fixtureTable(5, 'B03', 4, 'UNSETTLED', demoOrder),
        fixtureTable(6, 'B05', 2, 'DINING', { id: 5029, orderNo: 'D260725145800', status: 'PREPARING', dinerCount: 2, amount: 92 }),
        fixtureTable(7, 'B06', 4, 'UNSETTLED', { id: 5030, orderNo: 'D260725143011', status: 'READY', paymentStatus: 'UNPAID', settlementMode: 'PAY_AFTER', dinerCount: 4, amount: 186 }),
      ],
    },
    {
      id: 3,
      name: '靠窗',
      tables: [],
    },
    {
      id: 2,
      name: '包间',
      tables: [
        fixtureTable(9, '包间1', 6, 'UNSETTLED', { id: 5027, orderNo: 'D260725140506', status: 'READY', paymentStatus: 'UNPAID', settlementMode: 'PAY_AFTER', dinerCount: 6, amount: 568 }),
        fixtureTable(10, '包间2', 10, 'UNOPENED'),
        fixtureTable(11, '包间3', 6, 'DINING', { id: 5026, orderNo: 'D260725145212', status: 'PREPARING', dinerCount: 5, amount: 248 }),
        fixtureTable(13, '包点单', 6, 'SETTLED', { id: 5032, orderNo: 'D260725152512', status: 'PAID', dinerCount: 6 }),
        fixtureTable(12, '包间6', 12, 'UNOPENED'),
      ],
    },
  ],
};

const previewTakeoutOrders: Order[] = [
  { id: 6101, orderNo: 'T260725152201', pickupNo: '038', status: 'PREPARING', paymentStatus: 'PAID', settlementMode: 'PAY_BEFORE', amount: 42, paidAmount: 42, orderType: 'TAKEOUT', createdAt: '2026-07-25 15:22:01', items: [{ productName: '招牌牛肉饭', quantity: 1, unitPrice: 32 }, { productName: '冰豆浆', quantity: 1, unitPrice: 10 }] },
  { id: 6102, orderNo: 'T260725151825', pickupNo: '037', status: 'READY', paymentStatus: 'PAID', settlementMode: 'PAY_BEFORE', amount: 28, paidAmount: 28, orderType: 'TAKEOUT', createdAt: '2026-07-25 15:18:25', items: [{ productName: '辣椒炒肉拌面', quantity: 1, unitPrice: 28 }] },
  { id: 6103, orderNo: 'T260725151403', pickupNo: '036', status: 'PENDING_PAYMENT', paymentStatus: 'UNPAID', settlementMode: 'PAY_BEFORE', amount: 56, remainingAmount: 56, orderType: 'TAKEOUT', createdAt: '2026-07-25 15:14:03', items: [{ productName: '双人套餐', quantity: 1, unitPrice: 56 }] },
];

const previewPaymentRecords: PaymentRecord[] = [];
const previewReturnRequests: OrderReturnRequest[] = [];

const previewCatalog: CashierCatalog = {
  categories: [{ id: 1, name: '热销' }, { id: 2, name: '主食' }, { id: 3, name: '小菜' }],
  products: [
    { id: 1, categoryId: 1, name: '水煮牛肉', description: '招牌热销', price: 4800, stock: 30, soldOut: false, skus: [{ id: 11, name: '默认', price: 4800, stock: 30, soldOut: false }], optionGroups: [], modifierGroups: [] },
    { id: 2, categoryId: 1, name: '鱼香肉丝', price: 2200, stock: 26, soldOut: false, skus: [{ id: 21, name: '默认', price: 2200, stock: 26, soldOut: false }], optionGroups: [], modifierGroups: [] },
    { id: 3, categoryId: 2, name: '麻婆豆腐', price: 1800, stock: 18, soldOut: false, skus: [{ id: 31, name: '默认', price: 1800, stock: 18, soldOut: false }], optionGroups: [{ id: 301, name: '辣度', selectionMode: 'SINGLE', minSelect: 1, maxSelect: 1, values: [{ id: 3011, name: '微辣', priceDeltaCents: 0, isDefault: true }, { id: 3012, name: '中辣', priceDeltaCents: 0, isDefault: false }] }], modifierGroups: [] },
    { id: 4, categoryId: 2, name: '米饭', price: 800, stock: 99, soldOut: false, skus: [{ id: 41, name: '默认', price: 800, stock: 99, soldOut: false }], optionGroups: [], modifierGroups: [] },
    { id: 5, categoryId: 3, name: '清炒时蔬', price: 1600, stock: 14, soldOut: false, skus: [{ id: 51, name: '小份', price: 1600, stock: 14, soldOut: false }, { id: 52, name: '大份', price: 2200, stock: 12, soldOut: false }], optionGroups: [], modifierGroups: [] },
  ],
};

function normalizeDashboard(raw: unknown): DashboardData {
  const value = (raw ?? {}) as Record<string, unknown>;
  return {
    todayRevenue: value.today_revenue_cents !== undefined ? Number(value.today_revenue_cents) / 100 : Number(value.todayRevenue ?? 0),
    todayOrders: Number(value.todayOrders ?? value.today_orders ?? 0),
    pendingOrders: Number(value.pendingOrders ?? value.pending_orders ?? value.active_orders ?? 0),
    averageOrderValue: Number(value.averageOrderValue ?? value.average_order_value ?? 0),
  };
}

function normalizeCatalog(raw: unknown): CashierCatalog {
  const value = (raw ?? {}) as Record<string, unknown>;
  return {
    categories: ((value.categories ?? []) as Array<Record<string, unknown>>).map((item) => ({
      id: Number(item.id),
      name: String(item.name ?? ''),
    })),
    products: ((value.products ?? []) as Array<Record<string, unknown>>).map((item) => ({
      id: Number(item.id),
      categoryId: Number(item.categoryId ?? item.category_id),
      name: String(item.name ?? ''),
      description: item.description ? String(item.description) : undefined,
      imageUrl: item.imageUrl ? String(item.imageUrl) : undefined,
      price: Number(item.price ?? 0),
      stock: Number(item.stock ?? 0),
      soldOut: Boolean(item.soldOut ?? item.sold_out),
      skus: ((item.skus ?? []) as CatalogSku[]).map((sku) => ({ ...sku, id: Number(sku.id), price: Number(sku.price), stock: Number(sku.stock) })),
      optionGroups: ((item.optionGroups ?? []) as CatalogOptionGroup[]).map((group) => ({
        ...group,
        id: Number(group.id),
        minSelect: Number(group.minSelect),
        maxSelect: Number(group.maxSelect),
        values: group.values.map((option) => ({ ...option, id: Number(option.id), priceDeltaCents: Number(option.priceDeltaCents) })),
      })),
      modifierGroups: ((item.modifierGroups ?? []) as CatalogModifierGroup[]).map((group) => ({
        ...group,
        id: Number(group.id),
        minSelect: Number(group.minSelect),
        maxSelect: Number(group.maxSelect),
        items: group.items.map((modifier) => ({ ...modifier, id: Number(modifier.id), priceCents: Number(modifier.priceCents) })),
      })),
    })),
  };
}

function elapsedLabel(openedAt?: string): string {
  if (!openedAt) return '';
  const parsed = new Date(openedAt.replace(' ', 'T')).getTime();
  if (!Number.isFinite(parsed)) return '';
  const minutes = Math.max(0, Math.floor((Date.now() - parsed) / 60_000));
  return minutes >= 60
    ? `${Math.floor(minutes / 60).toString().padStart(2, '0')}:${(minutes % 60).toString().padStart(2, '0')}:00`
    : `${minutes.toString().padStart(2, '0')}:00`;
}

function elapsedMinutes(openedAt?: string): number {
  if (!openedAt) return 0;
  const parsed = new Date(openedAt.replace(' ', 'T')).getTime();
  if (!Number.isFinite(parsed)) return 0;
  return Math.max(0, Math.floor((Date.now() - parsed) / 60_000));
}

function elapsedLabelMinutes(label: string): number {
  const parts = label.split(':').map(Number);
  if (parts.some((part) => !Number.isFinite(part))) return 0;
  if (parts.length === 3) return parts[0] * 60 + parts[1];
  if (parts.length === 2) return parts[0];
  return 0;
}

function previewElapsed(tableName?: string): string {
  return ({
    A02: '00:05',
    B03: '45:20',
    B05: '18:10',
    B06: '12:35',
    包间1: '01:05:33',
    包间3: '25:40',
    包点单: '00:12',
  } as Record<string, string>)[tableName || ''] || '';
}

function orderItemTotal(item: OrderItem): number {
  return Number(item.amount ?? item.unitPrice * item.quantity);
}

function normalizeReturnRequest(value: OrderReturnRequest): OrderReturnRequest {
  const raw = value as unknown as Record<string, unknown>;
  return {
    ...value,
    id: value.id ?? raw.id as string | number,
    orderItemId: value.orderItemId ?? raw.order_item_id as string | number,
    skuId: value.skuId ?? raw.sku_id as string | number,
    productName: value.productName ?? String(raw.product_name ?? ''),
    quantity: Number(value.quantity ?? raw.quantity ?? 0),
    amount: raw.amount_cents !== undefined ? Number(raw.amount_cents) / 100 : Number(value.amount ?? 0),
    reason: value.reason ?? String(raw.reason ?? ''),
    status: String(value.status ?? raw.status ?? 'APPROVED') as OrderReturnRequest['status'],
    createdAt: value.createdAt ?? String(raw.created_at ?? ''),
  };
}

function normalizeCashierPayment(value: PaymentRecord): PaymentRecord {
  const raw = value as unknown as Record<string, unknown>;
  const method = String(value.method ?? raw.payment_method ?? raw.provider ?? '').toUpperCase();
  return {
    ...value,
    orderId: value.orderId ?? raw.order_id as string | number,
    orderNo: value.orderNo ?? String(raw.order_no ?? ''),
    paymentNo: value.paymentNo ?? String(raw.provider_order_no ?? ''),
    provider: value.provider ?? String(raw.provider ?? ''),
    method,
    amount: raw.amount_cents !== undefined ? Number(raw.amount_cents) / 100 : Number(value.amount ?? 0),
    status: String(value.status ?? raw.status ?? ''),
    paidAt: value.paidAt ?? (raw.paid_at ? String(raw.paid_at) : undefined),
    createdAt: value.createdAt ?? String(raw.created_at ?? ''),
  };
}

function cashierPaymentMethodLabel(payment: PaymentRecord): string {
  const method = String(payment.method || payment.provider || '').toUpperCase();
  if (method === 'BALANCE') return '会员余额';
  if (method === 'WECHAT_MICROPAY' || method === 'WECHAT_PARTNER') return '微信付款码';
  if (method === 'OFFLINE_CASH' || method === 'CASH') return '现金';
  if (method === 'EXTERNAL') return '系统外支付';
  return payment.method || payment.provider || '其他支付';
}

function nextIdempotencyKey(): string {
  return globalThis.crypto?.randomUUID?.() || `cashier-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function cashierOrderStatusText(order: Pick<Order, 'status' | 'paymentStatus' | 'settlementMode'>): string {
  if (order.paymentStatus === 'UNPAID') {
    return order.settlementMode === 'PAY_AFTER' ? '待结账' : '待付款';
  }
  return orderStatusMap[order.status]?.text || order.status;
}

export function CashierPage({ previewMode = false }: { previewMode?: boolean }) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { message, modal } = AntApp.useApp();
  const [loading, setLoading] = useState(!previewMode);
  const [refreshing, setRefreshing] = useState(false);
  const [mode, setMode] = useState<CashierMode>('DINE_IN');
  const [layoutMode, setLayoutMode] = useState<CashierLayoutMode>(() => {
    const storage = typeof window === 'undefined' ? undefined : window.localStorage;
    const saved = typeof storage?.getItem === 'function' ? storage.getItem(CASHIER_LAYOUT_MODE_KEY) : '';
    return saved === 'STANDARD' ? 'STANDARD' : 'COMPACT';
  });
  const [board, setBoard] = useState<TableBoardResponse | null>(previewMode ? previewBoard : null);
  const [takeoutOrders, setTakeoutOrders] = useState<Order[]>(previewMode ? previewTakeoutOrders : []);
  const [selectedTable, setSelectedTable] = useState<TableBoardTable | null>(previewMode ? previewBoard.areas[0].tables[4] : null);
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(previewMode ? demoOrder : null);
  const [returnRequests, setReturnRequests] = useState<OrderReturnRequest[]>(previewMode ? previewReturnRequests : []);
  const [paymentRecords, setPaymentRecords] = useState<PaymentRecord[]>(previewMode ? previewPaymentRecords : []);
  const [context, setContext] = useState<CashierContext | null>(previewMode ? {
    storeId: 1, storeCode: 'preview-store', storeName: '川味小馆（天府店）', operatorName: '张小雨', role: 'MERCHANT_MANAGER',
    paymentProvider: 'wechat_partner', wechatCodePaymentEnabled: true,
  } : null);
  const [dashboard, setDashboard] = useState<DashboardData>(previewMode ? { todayRevenue: 3286.5, todayOrders: 48, pendingOrders: 7, averageOrderValue: 68.47 } : { todayRevenue: 0, todayOrders: 0, pendingOrders: 0, averageOrderValue: 0 });
  const [catalog, setCatalog] = useState<CashierCatalog>(previewMode ? previewCatalog : { categories: [], products: [] });
  const [areaID, setAreaID] = useState<string>('ALL');
  const [tableFilter, setTableFilter] = useState<CashierTableFilter>('ALL');
  const [cartOpen, setCartOpen] = useState(false);
  const [cartMode, setCartMode] = useState<CashierMode>('DINE_IN');
  const [cartTable, setCartTable] = useState<TableBoardTable | null>(null);
  const [cart, setCart] = useState<CartLine[]>([]);
  const [activeCategory, setActiveCategory] = useState<number | 'ALL'>('ALL');
  const [productSearch, setProductSearch] = useState('');
  const [configuringProduct, setConfiguringProduct] = useState<CatalogProduct | null>(null);
  const [skuID, setSkuID] = useState<number>(0);
  const [optionSelections, setOptionSelections] = useState<Record<number, number[]>>({});
  const [modifierSelections, setModifierSelections] = useState<Record<number, number[]>>({});
  const [itemQuantity, setItemQuantity] = useState(1);
  const [itemRemark, setItemRemark] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [returnOpen, setReturnOpen] = useState(false);
  const [returnItemID, setReturnItemID] = useState<string | number>();
  const [returnQuantity, setReturnQuantity] = useState(1);
  const [returnReason, setReturnReason] = useState('');
  const [handoverOpen, setHandoverOpen] = useState(false);
  const [handoverRemark, setHandoverRemark] = useState('');
  const [detailFocused, setDetailFocused] = useState(false);
  const [checkoutOpen, setCheckoutOpen] = useState(false);
  const [wechatScanOpen, setWechatScanOpen] = useState(false);
  const [wechatCodeInput, setWechatCodeInput] = useState('');
  const [wechatPayment, setWechatPayment] = useState<WechatCodePaymentResult | null>(null);
  const [cameraError, setCameraError] = useState('');
  const boardRef = useRef<HTMLDivElement | null>(null);
  const orderLoadSequenceRef = useRef(0);
  const scanVideoRef = useRef<HTMLVideoElement | null>(null);
  const scanControlsRef = useRef<IScannerControls | null>(null);
  const scanSubmittingRef = useRef(false);
  const [clock, setClock] = useState(new Date());

  const allTables = useMemo(() => board?.areas.flatMap((area) => area.tables) ?? [], [board]);
  const unsettledTables = allTables.filter((table) => table.state === 'UNSETTLED');
  const settledTables = allTables.filter((table) => table.state === 'SETTLED');
  const overdueTables = allTables.filter((table) => table.orderId && (
    previewMode
      ? elapsedLabelMinutes(previewElapsed(table.name)) >= 60
      : elapsedMinutes(table.openedAt) >= 60
  ));
  const visibleTables = useMemo(() => {
    const byArea = areaID === 'ALL' ? allTables : allTables.filter((table) => String(table.areaId) === areaID);
    if (tableFilter === 'UNSETTLED') return byArea.filter((table) => table.state === 'UNSETTLED');
    if (tableFilter === 'SETTLED') return byArea.filter((table) => table.state === 'SETTLED');
    if (tableFilter === 'OVERDUE') {
      const overdueIDs = new Set(overdueTables.map((table) => String(table.id)));
      return byArea.filter((table) => overdueIDs.has(String(table.id)));
    }
    return byArea;
  }, [allTables, areaID, overdueTables, tableFilter]);
  const pendingAmount = unsettledTables.reduce((sum, table) => sum + Number(table.totalCents ?? 0) / 100, 0);
  const cartTotalCents = cart.reduce((sum, item) => sum + item.priceCents * item.quantity, 0);
  const approvedReturnRequests = returnRequests.filter((request) => request.status === 'APPROVED');
  const orderPaidAmount = Number(selectedOrder?.paidAmount ?? 0);
  const orderRemainingAmount = Number(selectedOrder?.remainingAmount ?? selectedOrder?.amount ?? 0);
  const canOperateUnpaidDineIn = mode === 'DINE_IN' && canOperateUnpaidPayAfterOrder(selectedOrder);
  const canAddSelectedOrder = mode === 'DINE_IN' && canAddItemsToOrder(selectedOrder);
  const canReturnSelectedOrder = mode === 'DINE_IN' && canReturnItemsFromOrder(selectedOrder);
  const addBlockedReason = selectedTable?.orderId ? addItemsBlockedReason(selectedOrder) : '';
  const transferableTables = allTables.filter((table) => !table.orderId);
  const mergeableTables = allTables.filter((table) => isMergeableTableBill(table, selectedOrder?.id));
  const transferBlockedReason = !canOperateUnpaidDineIn
    ? dineInOperationBlockedReason('TRANSFER', selectedOrder)
    : transferableTables.length === 0 ? '当前没有可转入的空闲桌台' : '';
  const mergeBlockedReason = !canOperateUnpaidDineIn
    ? dineInOperationBlockedReason('MERGE', selectedOrder)
    : mergeableTables.length === 0 ? '当前没有其他可合并的未收款后付账桌台' : '';
  const returnBlockedReason = !canReturnSelectedOrder ? dineInOperationBlockedReason('RETURN', selectedOrder) : '';
  const workflowAction = selectedOrder ? nextOrderWorkflowAction[selectedOrder.status] : undefined;
  const workflowActionLabel = selectedOrder ? orderWorkflowActionText(selectedOrder.status, selectedOrder.orderType) : '';
  const workflowActionEnabled = Boolean(
    selectedOrder
    && workflowAction
    && canRunOrderWorkflowAction(selectedOrder.status, selectedOrder.settlementMode, selectedOrder.paymentStatus),
  );
  const canOpenSelectedTableOrder = Boolean(
    mode === 'DINE_IN'
    && selectedTable
    && (!selectedTable.orderId || canAddSelectedOrder),
  );

  const loadOrder = useCallback(async (orderID: string | number) => {
    const requestSequence = ++orderLoadSequenceRef.current;
    if (previewMode) {
      const order = String(orderID) === String(demoOrder.id)
        ? demoOrder
        : previewTakeoutOrders.find((item) => String(item.id) === String(orderID)) ?? demoOrder;
      setSelectedOrder(order);
      setReturnRequests(String(order.id) === String(demoOrder.id) ? previewReturnRequests : []);
      setPaymentRecords(String(order.id) === String(demoOrder.id) ? previewPaymentRecords : []);
      return;
    }
    try {
      const [rawOrder, rawReturns, rawPayments] = await Promise.all([
        api.get<Order>(`/merchant/orders/${orderID}`),
        api.getList<OrderReturnRequest>(`/merchant/orders/${orderID}/return-requests`),
        api.getList<PaymentRecord>(`/merchant/orders/${orderID}/payments`),
      ]);
      if (requestSequence !== orderLoadSequenceRef.current) return;
      setSelectedOrder(normalizeOrder(rawOrder));
      setReturnRequests(rawReturns.items.map(normalizeReturnRequest));
      setPaymentRecords(rawPayments.items.map(normalizeCashierPayment));
    } catch (error) {
      if (requestSequence !== orderLoadSequenceRef.current) return;
      message.error(errorMessage(error));
    }
  }, [message, previewMode]);

  const load = useCallback(async (quiet = false) => {
    if (previewMode) return;
    quiet ? setRefreshing(true) : setLoading(true);
    try {
      const cashierContext = await api.get<CashierContext>('/merchant/cashier/context');
      const [nextBoard, orderResult, rawDashboard, rawCatalog] = await Promise.all([
        api.get<TableBoardResponse>('/merchant/table-board'),
        api.getList<Order>('/merchant/orders', { order_type: 'TAKEOUT', cashier_active: true, page: 1, page_size: 100 }),
        api.get<DashboardData>('/merchant/dashboard'),
        api.get<unknown>(`/public/stores/${encodeURIComponent(cashierContext.storeCode)}/catalog`),
      ]);
      setContext(cashierContext);
      setBoard(nextBoard);
      const nextTakeoutOrders = orderResult.items.map(normalizeOrder);
      setTakeoutOrders(nextTakeoutOrders);
      setDashboard(normalizeDashboard(rawDashboard));
      setCatalog(normalizeCatalog(rawCatalog));
      setSelectedTable((current) => {
        if (!current) return nextBoard.areas.flatMap((area) => area.tables).find((table) => table.orderId) ?? nextBoard.areas[0]?.tables[0] ?? null;
        return nextBoard.areas.flatMap((area) => area.tables).find((table) => String(table.id) === String(current.id)) ?? null;
      });
      if (selectedOrder?.id) {
        if (mode === 'TAKEOUT' && !nextTakeoutOrders.some((order) => String(order.id) === String(selectedOrder.id))) {
          setSelectedOrder(null);
        } else {
          await loadOrder(selectedOrder.id);
        }
      }
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [loadOrder, message, mode, previewMode, selectedOrder?.id]);

  useEffect(() => {
    if (previewMode) return;
    let active = true;
    const initialize = async () => {
      try {
        const session = await api.post<{ accessToken: string }>('/merchant/cashier/session');
        if (!active) return;
        localStorage.setItem(CASHIER_TOKEN_KEY, session.accessToken);
        await load();
      } catch (error) {
        if (active) message.error(errorMessage(error));
      }
    };
    void initialize();
    return () => { active = false; };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (previewMode) return;
    const timer = window.setInterval(() => void load(true), 30_000);
    const refresh = () => void load(true);
    window.addEventListener('focus', refresh);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener('focus', refresh);
    };
  }, [load, previewMode]);

  useEffect(() => {
    const timer = window.setInterval(() => setClock(new Date()), 30_000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (mode === 'DINE_IN' && selectedTable?.orderId) void loadOrder(selectedTable.orderId);
  }, [loadOrder, mode, selectedTable?.orderId]);

  const selectTable = (table: TableBoardTable) => {
    setSelectedTable(table);
    setSelectedOrder(null);
    setReturnRequests([]);
    setPaymentRecords([]);
    setDetailFocused(layoutMode === 'STANDARD');
    if (table.orderId) void loadOrder(table.orderId);
  };

  const selectTakeoutOrder = (order: Order) => {
    setSelectedTable(null);
    setSelectedOrder(order);
    setReturnRequests([]);
    setPaymentRecords([]);
    setDetailFocused(layoutMode === 'STANDARD');
    void loadOrder(order.id);
  };

  const switchMode = (nextMode: CashierMode) => {
    if (nextMode === mode) return;
    setMode(nextMode);
    setSelectedOrder(null);
    setReturnRequests([]);
    setPaymentRecords([]);
    setDetailFocused(false);
    if (nextMode === 'TAKEOUT') {
      setSelectedTable(null);
      return;
    }
    setSelectedTable(allTables.find((table) => table.orderId) ?? allTables[0] ?? null);
  };

  const switchLayoutMode = (nextMode: CashierLayoutMode) => {
    setLayoutMode(nextMode);
    if (typeof window.localStorage?.setItem === 'function') {
      window.localStorage.setItem(CASHIER_LAYOUT_MODE_KEY, nextMode);
    }
    setDetailFocused(false);
    window.requestAnimationFrame(() => boardRef.current?.scrollTo?.({ top: 0, behavior: 'auto' }));
  };

  const openOrdering = (nextMode: CashierMode, table?: TableBoardTable | null) => {
    if (nextMode === 'DINE_IN' && !table) {
      const freeTable = allTables.find((item) => item.state === 'UNOPENED');
      if (!freeTable) {
        message.warning('当前没有空闲桌台');
        return;
      }
      table = freeTable;
      setSelectedTable(freeTable);
    }
    if (nextMode === 'DINE_IN' && !table?.publicId) {
      message.error('桌台缺少公开点单标识，请在桌码管理中重新生成桌码');
      return;
    }
    if (nextMode === 'DINE_IN' && table?.orderId && !canAddSelectedOrder) {
      message.warning(addItemsBlockedReason(selectedOrder));
      return;
    }
    setCartMode(nextMode);
    setCartTable(nextMode === 'DINE_IN' ? table ?? null : null);
    setCart([]);
    setCartOpen(true);
  };

  const configureProduct = (product: CatalogProduct) => {
    if (product.soldOut) return;
    const firstSku = product.skus.find((sku) => !sku.soldOut);
    if (!firstSku) {
      message.warning('该商品当前无可售规格');
      return;
    }
    const defaults: Record<number, number[]> = {};
    for (const group of product.optionGroups) {
      defaults[group.id] = group.values.filter((value) => value.isDefault).map((value) => value.id);
    }
    const modifierDefaults: Record<number, number[]> = {};
    for (const group of product.modifierGroups) {
      modifierDefaults[group.id] = group.items.filter((item) => item.isDefault).map((item) => item.id);
    }
    setSkuID(firstSku.id);
    setOptionSelections(defaults);
    setModifierSelections(modifierDefaults);
    setItemQuantity(1);
    setItemRemark('');
    if (product.skus.length === 1 && product.optionGroups.length === 0 && product.modifierGroups.length === 0) {
      addConfiguredProduct(product, firstSku.id, defaults, modifierDefaults, 1, '');
      return;
    }
    setConfiguringProduct(product);
  };

  const addConfiguredProduct = (
    product: CatalogProduct,
    selectedSkuID: number,
    selectedOptions: Record<number, number[]>,
    selectedModifiers: Record<number, number[]>,
    quantity: number,
    remark: string,
  ) => {
    const sku = product.skus.find((item) => item.id === selectedSkuID);
    if (!sku) return;
    for (const group of product.optionGroups) {
      const count = selectedOptions[group.id]?.length ?? 0;
      if (count < group.minSelect || count > group.maxSelect) {
        message.warning(`${group.name}需选择 ${group.minSelect}${group.maxSelect !== group.minSelect ? `–${group.maxSelect}` : ''} 项`);
        return;
      }
    }
    for (const group of product.modifierGroups) {
      const count = selectedModifiers[group.id]?.length ?? 0;
      if (count < group.minSelect || count > group.maxSelect) {
        message.warning(`${group.name}需选择 ${group.minSelect}${group.maxSelect !== group.minSelect ? `–${group.maxSelect}` : ''} 项`);
        return;
      }
    }
    const optionIDs = Object.values(selectedOptions).flat();
    const modifierIDs = Object.values(selectedModifiers).flat();
    const optionNames = product.optionGroups.flatMap((group) => group.values.filter((item) => optionIDs.includes(item.id)).map((item) => item.name));
    const modifierNames = product.modifierGroups.flatMap((group) => group.items.filter((item) => modifierIDs.includes(item.id)).map((item) => item.name));
    const optionDelta = product.optionGroups.flatMap((group) => group.values).filter((item) => optionIDs.includes(item.id)).reduce((sum, item) => sum + item.priceDeltaCents, 0);
    const modifierDelta = product.modifierGroups.flatMap((group) => group.items).filter((item) => modifierIDs.includes(item.id)).reduce((sum, item) => sum + item.priceCents, 0);
    const summary = [sku.name === '默认' ? '' : sku.name, ...optionNames, ...modifierNames].filter(Boolean).join('、');
    const modifiers = product.modifierGroups.flatMap((group) => (selectedModifiers[group.id] ?? []).map((modifierItemId) => ({ groupId: group.id, modifierItemId, quantity: 1 })));
    const key = JSON.stringify([product.id, sku.id, optionIDs.sort(), modifierIDs.sort(), remark]);
    setCart((current) => {
      const existing = current.find((item) => item.key === key);
      if (existing) return current.map((item) => item.key === key ? { ...item, quantity: item.quantity + quantity } : item);
      return [...current, {
        key,
        productId: product.id,
        skuId: sku.id,
        name: product.name,
        skuName: sku.name,
        priceCents: sku.price + optionDelta + modifierDelta,
        quantity,
        optionValueIds: optionIDs,
        modifiers,
        summary,
        itemRemark: remark,
      }];
    });
    setConfiguringProduct(null);
  };

  const changeCartQuantity = (key: string, delta: number) => {
    setCart((current) => current.flatMap((item) => {
      if (item.key !== key) return [item];
      const quantity = item.quantity + delta;
      return quantity > 0 ? [{ ...item, quantity }] : [];
    }));
  };

  const submitOrder = async () => {
    if (!context || cart.length === 0) return;
    setSubmitting(true);
    try {
      if (previewMode) {
        if (cartMode === 'DINE_IN' && cartTable?.orderId && selectedOrder) {
          setSelectedOrder({
            ...selectedOrder,
            additionCount: Number(selectedOrder.additionCount || 1) + 1,
            amount: selectedOrder.amount + cartTotalCents / 100,
            remainingAmount: Number(selectedOrder.remainingAmount ?? selectedOrder.amount) + cartTotalCents / 100,
            status: selectedOrder.status === 'READY' ? 'PAID' : selectedOrder.status,
          });
        }
        message.success(cartMode === 'DINE_IN' && cartTable?.orderId ? '加菜单已提交，账单已更新' : '订单已创建并打印');
        setCartOpen(false);
        return;
      }
      const updated = normalizeOrder(await api.postIdempotent<Order>(`/public/stores/${encodeURIComponent(context.storeCode)}/orders`, {
        fulfillmentType: cartMode === 'DINE_IN' ? 'DINE_IN' : 'PICKUP',
        orderType: cartMode,
        dinerCount: cartMode === 'DINE_IN' ? Math.max(1, cartTable?.dinerCount || 1) : 1,
        ...(cartMode === 'DINE_IN' ? { order_scene: 'DINE_IN', table_public_id: cartTable?.publicId } : {}),
        remark: '收银台点单',
        items: cart.map((item) => ({
          productId: item.productId,
          skuId: item.skuId,
          quantity: item.quantity,
          optionValueIds: item.optionValueIds,
          modifiers: item.modifiers,
          itemRemark: item.itemRemark,
        })),
      }, nextIdempotencyKey()));
      setSelectedOrder(updated);
      message.success(cartMode === 'DINE_IN' && cartTable?.orderId
        ? `第 ${Math.max(Number(updated.additionCount || 1) - 1, 1)} 次加菜已提交，账单已更新`
        : '订单已创建并打印');
      setCartOpen(false);
      await load(true);
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setSubmitting(false);
    }
  };

  const printCustomerCopy = async () => {
    if (!selectedOrder) return;
    if (previewMode) {
      message.success('客户核对联打印任务已提交');
      return;
    }
    try {
      await api.post(`/merchant/orders/${selectedOrder.id}/reprint`, { output_type: 'RECEIPT', copy_role: 'CUSTOMER' });
      message.success('客户核对联打印任务已提交');
    } catch (error) {
      message.error(errorMessage(error));
    }
  };

  const settle = () => {
    if (!selectedOrder) return;
    setWechatPayment(null);
    setCheckoutOpen(true);
  };

  const confirmSettlement = async (method: 'CASH' | 'EXTERNAL') => {
    if (!selectedOrder) return;
    setSubmitting(true);
    try {
      if (previewMode) {
        setSelectedOrder({ ...selectedOrder, status: selectedOrder.settlementMode === 'PAY_AFTER' ? 'COMPLETED' : 'PAID', paymentStatus: 'PAID', paidAmount: selectedOrder.amount, remainingAmount: 0 });
        setPaymentRecords((current) => [{
          id: `preview-payment-${Date.now()}`,
          orderId: selectedOrder.id,
          orderNo: selectedOrder.orderNo,
          amount: orderRemainingAmount,
          method,
          provider: method === 'CASH' ? 'offline_cash' : 'external',
          status: 'SUCCESS',
          paidAt: '2026-07-25 15:30:00',
        }, ...current]);
        setCheckoutOpen(false);
        message.success('收款已登记，订单状态已更新');
        return;
      }
      const path = selectedOrder.settlementMode === 'PAY_AFTER'
        ? `/merchant/orders/${selectedOrder.id}/settle`
        : `/merchant/orders/${selectedOrder.id}/cashier-settle`;
      const updated = normalizeOrder(await api.post<Order>(path, { method, remark: `收银台确认${method === 'CASH' ? '现金收款' : '系统外支付'}` }));
      setSelectedOrder(updated);
      setCheckoutOpen(false);
      message.success('收款已登记，订单状态已更新');
      await load(true);
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setSubmitting(false);
    }
  };

  const finishWechatPayment = useCallback(async (result: WechatCodePaymentResult) => {
    setWechatPayment(result);
    if (result.status !== 'SUCCESS') return;
    scanControlsRef.current?.stop();
    scanControlsRef.current = null;
    message.success(result.paymentMethod === 'BALANCE' ? '会员余额支付成功' : '微信支付成功');
    setWechatScanOpen(false);
    setCheckoutOpen(false);
    if (previewMode && selectedOrder) {
      setSelectedOrder({
        ...selectedOrder,
        status: selectedOrder.settlementMode === 'PAY_AFTER' ? 'COMPLETED' : 'PAID',
        paymentStatus: 'PAID',
        paidAmount: selectedOrder.amount,
        remainingAmount: 0,
      });
      setPaymentRecords((current) => [{
        id: `preview-wechat-${Date.now()}`,
        orderId: selectedOrder.id,
        orderNo: selectedOrder.orderNo,
        paymentNo: result.providerTransactionNo,
        amount: orderRemainingAmount,
        method: result.paymentMethod || 'WECHAT_MICROPAY',
        provider: 'wechat_partner',
        status: 'SUCCESS',
        paidAt: '2026-07-25 15:30:00',
      }, ...current]);
      return;
    }
    await load(true);
  }, [load, message, orderRemainingAmount, previewMode, selectedOrder]);

  const submitWechatCode = useCallback(async (rawCode: string) => {
    const code = rawCode.trim();
    if (!selectedOrder || scanSubmittingRef.current) return;
    if (!/^(10|11|12|13|14|15)\d{16}$/.test(code)) {
      message.warning('请扫描微信中的 18 位付款码');
      return;
    }
    scanSubmittingRef.current = true;
    scanControlsRef.current?.stop();
    scanControlsRef.current = null;
    setWechatCodeInput('');
    setSubmitting(true);
    setWechatPayment({
      paymentId: 0, status: 'CREATING', message: '正在向微信确认付款，请勿重复扫码',
    });
    try {
      if (previewMode) {
        await finishWechatPayment({
          paymentId: 9001, status: 'SUCCESS', message: '微信支付成功',
          providerTransactionNo: '4200000000000000001',
        });
        return;
      }
      const result = await api.post<WechatCodePaymentResult>(
        `/merchant/orders/${selectedOrder.id}/wechat-code-pay`,
        { auth_code: code, device_id: `cashier-${context?.storeCode || 'terminal'}` },
      );
      await finishWechatPayment(result);
    } catch (error) {
      const outcomeUnknown = error instanceof ApiError &&
        (error.code === 'PAYMENT_IN_PROGRESS' || error.status === undefined);
      setWechatPayment({
        paymentId: 0,
        status: outcomeUnknown ? 'PENDING' : 'FAILED',
        message: outcomeUnknown ? '请求结果未知，正在通过订单号持续查单，请勿重复扫码。' : errorMessage(error),
      });
      outcomeUnknown ? message.warning('支付结果仍在确认，请勿重复扫码') : message.error(errorMessage(error));
    } finally {
      scanSubmittingRef.current = false;
      setSubmitting(false);
    }
  }, [context?.storeCode, finishWechatPayment, message, previewMode, selectedOrder]);

  const openWechatScanner = () => {
    if (!context?.wechatCodePaymentEnabled) {
      message.warning(context?.wechatCodePaymentReason || '微信付款码支付尚未配置');
      return;
    }
    setCameraError('');
    setWechatPayment(null);
    setWechatCodeInput('');
    setWechatScanOpen(true);
  };

  const cancelWechatPayment = async () => {
    if (!selectedOrder) return;
    setSubmitting(true);
    try {
      if (previewMode) {
        setWechatPayment({ paymentId: 9001, status: 'CLOSED', message: '本次付款已撤销' });
        return;
      }
      const result = await api.post<WechatCodePaymentResult>(
        `/merchant/orders/${selectedOrder.id}/wechat-code-pay/cancel`,
        {},
      );
      setWechatPayment(result);
      if (result.status === 'CLOSED') message.success('本次微信付款已撤销，可以重新扫码');
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setSubmitting(false);
    }
  };

  useEffect(() => {
    if (!wechatScanOpen || !selectedOrder || wechatPayment?.status === 'CREATING' ||
      wechatPayment?.status === 'PENDING' || wechatPayment?.status === 'SUCCESS') return;
    let active = true;
    const startCamera = async () => {
      if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
        setCameraError('当前浏览器环境无法调用摄像头，请使用 HTTPS 打开收银台，或使用扫码枪/手动输入。');
        return;
      }
      try {
        const reader = new BrowserMultiFormatReader();
        const controls = await reader.decodeFromConstraints(
          { audio: false, video: { facingMode: { ideal: 'environment' } } },
          scanVideoRef.current ?? undefined,
          (result) => {
            if (result && active) void submitWechatCode(result.getText());
          },
        );
        if (!active) {
          controls.stop();
          return;
        }
        scanControlsRef.current = controls;
      } catch {
        if (active) setCameraError('摄像头未授权或不可用，请允许相机权限，或改用扫码枪/手动输入。');
      }
    };
    void startCamera();
    return () => {
      active = false;
      scanControlsRef.current?.stop();
      scanControlsRef.current = null;
    };
  }, [selectedOrder, submitWechatCode, wechatPayment?.status, wechatScanOpen]);

  useEffect(() => {
    if (!wechatScanOpen || previewMode || !selectedOrder ||
      !wechatPayment || !['CREATING', 'PENDING'].includes(wechatPayment.status)) return;
    let active = true;
    const poll = async () => {
      try {
        const result = await api.get<WechatCodePaymentResult>(
          `/merchant/orders/${selectedOrder.id}/wechat-code-pay/status`,
        );
        if (active) await finishWechatPayment(result);
      } catch {
        // A transient query failure must not turn an unknown provider result
        // into a local failure. The next interval continues the same query.
      }
    };
    const timer = window.setInterval(() => void poll(), 5_000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [finishWechatPayment, previewMode, selectedOrder, wechatPayment, wechatScanOpen]);

  const updateDinerCount = () => {
    if (!selectedOrder) return;
    let nextCount = selectedOrder.dinerCount || 1;
    modal.confirm({
      title: '修改就餐人数',
      content: <div className="cashier-number-field"><InputNumber min={1} max={99} defaultValue={nextCount} onChange={(value) => { nextCount = Number(value || 1); }} /><span>人</span></div>,
      okText: '保存人数',
      cancelText: '取消',
      onOk: async () => {
        try {
          if (previewMode) {
            setSelectedOrder((current) => current ? { ...current, dinerCount: nextCount } : current);
            message.success('就餐人数已更新');
            return;
          }
          const updated = normalizeOrder(await api.post<Order>(`/merchant/orders/${selectedOrder.id}/diner-count`, { diner_count: nextCount }));
          setSelectedOrder(updated);
          message.success('就餐人数已更新');
          await load(true);
        } catch (error) {
          message.error(errorMessage(error));
          throw error;
        }
      },
    });
  };

  const transferTable = () => {
    if (!selectedOrder) return;
    const options = transferableTables.map((table) => ({ label: `${table.areaName} · ${table.name}`, value: String(table.id) }));
    let target = options[0]?.value;
    modal.confirm({
      title: '转台',
      content: options.length ? <Select options={options} defaultValue={target} onChange={(value) => { target = value; }} style={{ width: '100%' }} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有可用空桌" />,
      okText: '确认转台',
      okButtonProps: { disabled: !options.length },
      onOk: async () => {
        try {
          if (previewMode) {
            message.success('转台完成');
            return;
          }
          await api.post(`/merchant/orders/${selectedOrder.id}/transfer-table`, { target_table_id: Number(target), remark: '收银台转台' });
          message.success('转台完成');
          await load(true);
        } catch (error) {
          message.error(errorMessage(error));
          throw error;
        }
      },
    });
  };

  const mergeTable = () => {
    if (!selectedOrder) return;
    const options = mergeableTables.map((table) => ({ label: `${table.areaName} · ${table.name}`, value: String(table.orderId) }));
    let sourceOrderID = options[0]?.value;
    modal.confirm({
      title: '并台',
      content: options.length ? <Select options={options} defaultValue={sourceOrderID} onChange={(value) => { sourceOrderID = value; }} style={{ width: '100%' }} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有可合并桌台" />,
      okText: '确认并台',
      okButtonProps: { disabled: !options.length },
      onOk: async () => {
        try {
          if (previewMode) {
            message.success('并台完成');
            return;
          }
          await api.post(`/merchant/orders/${selectedOrder.id}/merge`, { source_order_id: Number(sourceOrderID), remark: '收银台并台' });
          message.success('并台完成');
          await load(true);
        } catch (error) {
          message.error(errorMessage(error));
          throw error;
        }
      },
    });
  };

  const submitReturn = async () => {
    if (!selectedOrder || !returnItemID || !returnReason.trim()) {
      message.warning('请选择商品和退菜原因');
      return;
    }
    setSubmitting(true);
    try {
      if (previewMode) {
        const selectedItem = selectedOrder.items.find((item) => String(item.id) === String(returnItemID));
        if (!selectedItem) return;
        const returnedAmount = selectedItem.unitPrice * returnQuantity;
        setSelectedOrder({
          ...selectedOrder,
          amount: Math.max(0, selectedOrder.amount - returnedAmount),
          remainingAmount: Math.max(0, Number(selectedOrder.remainingAmount ?? selectedOrder.amount) - returnedAmount),
          items: selectedOrder.items
            .map((item) => String(item.id) === String(returnItemID)
              ? { ...item, quantity: item.quantity - returnQuantity, amount: orderItemTotal(item) - returnedAmount }
              : item)
            .filter((item) => item.quantity > 0),
        });
        setReturnRequests((current) => [{
          id: `preview-return-${Date.now()}`,
          orderItemId: returnItemID,
          skuId: 0,
          productName: selectedItem.productName,
          quantity: returnQuantity,
          amount: returnedAmount,
          reason: returnReason,
          status: 'APPROVED',
          createdAt: '2026-07-25 15:28:00',
        }, ...current]);
      } else {
        await api.post<Order>(`/merchant/orders/${selectedOrder.id}/return-requests`, {
          order_item_id: Number(returnItemID),
          quantity: returnQuantity,
          reason: returnReason.trim(),
        });
        await loadOrder(selectedOrder.id);
        await load(true);
      }
      message.success('退菜已完成，账单和库存已同步更新');
      setReturnOpen(false);
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setSubmitting(false);
    }
  };

  const updateStatus = async (status: Order['status']) => {
    if (!selectedOrder) return;
    const currentOrderID = selectedOrder.id;
    try {
      if (previewMode) {
        setSelectedOrder({ ...selectedOrder, status });
      } else {
        const updated = normalizeOrder(await api.post<Order>(`/merchant/orders/${currentOrderID}/status`, { status }));
        if (mode === 'TAKEOUT' && status === 'COMPLETED') {
          setTakeoutOrders((current) => current.filter((order) => String(order.id) !== String(currentOrderID)));
          setSelectedOrder(null);
        } else {
          setSelectedOrder(updated);
        }
        await load(true);
      }
      message.success('订单状态已更新');
    } catch (error) {
      message.error(errorMessage(error));
    }
  };

  const leaveCashier = () => {
    if (previewMode) return;
    navigate('/dashboard');
  };

  const toggleTableFilter = (nextFilter: CashierTableFilter) => {
    setAreaID('ALL');
    setTableFilter((current) => current === nextFilter ? 'ALL' : nextFilter);
  };

  const startHandover = () => {
    setHandoverRemark('');
    setHandoverOpen(true);
  };

  const submitHandover = async () => {
    setSubmitting(true);
    try {
      if (!previewMode) {
        await api.post('/merchant/cashier/handover', { remark: handoverRemark.trim() });
        localStorage.removeItem(CASHIER_TOKEN_KEY);
      }
      setHandoverOpen(false);
      message.success('交接记录已保存，已退出当前收银终端');
      if (!previewMode) navigate('/dashboard');
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      setSubmitting(false);
    }
  };

  const signOut = () => {
    if (previewMode) return;
    logout();
    navigate('/login', { replace: true });
  };

  const filteredProducts = catalog.products.filter((product) =>
    (activeCategory === 'ALL' || product.categoryId === activeCategory) &&
    (!productSearch.trim() || product.name.includes(productSearch.trim())));

  const currentOrderLabel = mode === 'DINE_IN'
    ? selectedTable?.name || '请选择桌台'
    : selectedOrder ? `取餐号 #${selectedOrder.pickupNo || '--'}` : '请选择带走订单';
  const wechatPaymentPending = wechatPayment?.status === 'CREATING' || wechatPayment?.status === 'PENDING';

  return (
    <div className="cashier-shell">
      <aside className="cashier-rail">
        <button className="cashier-rail-brand" type="button" aria-label="返回后台" onClick={leaveCashier} />
        <button className="cashier-rail-item active" type="button"><ShopOutlined /><span>收银</span></button>
        <button className="cashier-rail-item" type="button" onClick={() => navigate('/dine-in/orders')}><MenuOutlined /><span>订单</span></button>
        <button className="cashier-rail-item" type="button" onClick={() => navigate('/products')}><ShoppingOutlined /><span>商品</span></button>
        <button className="cashier-rail-item" type="button" onClick={leaveCashier}><AppstoreOutlined /><span>更多</span></button>
        <Tooltip title="退出当前账号" placement="right"><button className="cashier-rail-item cashier-rail-logout" type="button" onClick={signOut}><LogoutOutlined /><span>退出</span></button></Tooltip>
      </aside>

      <main className="cashier-stage">
        <header className="cashier-topbar">
          <div className="cashier-title"><strong>摊伴收银台</strong><i /><span>{context?.storeName || user?.storeName || '当前门店'}</span></div>
          <Tag bordered={false} className="cashier-shift-tag">早班营业中</Tag>
          <span className="cashier-operator">收银员：{context?.operatorName || user?.name || '店员'}</span>
          <Segmented
            className="cashier-layout-switch"
            aria-label="收银台模式"
            value={layoutMode}
            onChange={(value) => switchLayoutMode(value as CashierLayoutMode)}
            options={[
              { label: '简洁模式', value: 'COMPACT' },
              { label: '标准模式', value: 'STANDARD' },
            ]}
          />
          <div className="cashier-clock"><strong>{previewMode ? '15:26' : clock.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })}</strong><small>{previewMode ? '2026-07-25　周六' : clock.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', weekday: 'short' })}</small></div>
          <div className="cashier-device"><PrinterOutlined /><span>打印机<small><i />正常</small></span></div>
          <div className="cashier-device"><WifiOutlined /><span>网络<small><i />正常</small></span></div>
          <Button className="cashier-handover" onClick={startHandover}>交接班</Button>
        </header>

        <div className={`cashier-workspace is-${layoutMode.toLowerCase()} ${detailFocused ? 'is-detail-focused' : ''}`}>
          <section className="cashier-board" ref={boardRef}>
            <div className="cashier-mode-row">
              <div className="cashier-mode-tabs">
                <button type="button" className={mode === 'DINE_IN' ? 'active' : ''} onClick={() => switchMode('DINE_IN')}><TeamOutlined /><span>堂食点单</span></button>
                <button type="button" className={mode === 'TAKEOUT' ? 'active' : ''} onClick={() => switchMode('TAKEOUT')}><ShoppingOutlined /><span>带走点单</span></button>
              </div>
              <div className="cashier-create-actions">
                {mode === 'DINE_IN'
                  ? <Button className="is-dine-in" icon={<PlusOutlined />} onClick={() => openOrdering('DINE_IN', selectedTable?.state === 'UNOPENED' ? selectedTable : null)}>新开桌</Button>
                  : <Button className="is-takeout" icon={<PlusOutlined />} onClick={() => openOrdering('TAKEOUT')}>新建带走单</Button>}
              </div>
            </div>

            {mode === 'DINE_IN' ? (
              <>
                <div className="cashier-area-tabs">
                  <button type="button" className={areaID === 'ALL' && tableFilter === 'ALL' ? 'active' : ''} onClick={() => { setAreaID('ALL'); setTableFilter('ALL'); }}>全部</button>
                  {board?.areas.map((area) => <button type="button" className={areaID === String(area.id) && tableFilter === 'ALL' ? 'active' : ''} key={String(area.id)} onClick={() => { setAreaID(String(area.id)); setTableFilter('ALL'); }}>{area.name}</button>)}
                </div>
                <div className="cashier-alerts">
                  <button type="button" className={tableFilter === 'UNSETTLED' ? 'active' : ''} aria-pressed={tableFilter === 'UNSETTLED'} onClick={() => toggleTableFilter('UNSETTLED')}><DollarOutlined /><span>{unsettledTables.length}桌待结账</span><b>›</b></button>
                  <button type="button" className={tableFilter === 'SETTLED' ? 'active' : ''} aria-pressed={tableFilter === 'SETTLED'} onClick={() => toggleTableFilter('SETTLED')}><BellOutlined /><span>{settledTables.length}桌已结账</span><b>›</b></button>
                  <button type="button" className={tableFilter === 'OVERDUE' ? 'active' : ''} aria-pressed={tableFilter === 'OVERDUE'} onClick={() => toggleTableFilter('OVERDUE')}><ClockCircleOutlined /><span>{overdueTables.length}单超时待取</span><b>›</b></button>
                </div>
                <div className="cashier-table-grid" aria-busy={loading || refreshing}>
                  {visibleTables.map((table) => {
                    const meta = tableMeta[table.state];
                    const selected = String(selectedTable?.id) === String(table.id);
                    return (
                      <button type="button" className={`cashier-table-card ${meta.className} ${table.totalCents ? 'has-total' : ''} ${selected ? 'selected' : ''}`} key={String(table.id)} onClick={() => selectTable(table)}>
                        <strong>{table.name}</strong>
                        <span className="cashier-table-state">{meta.label}</span>
                        <div className="cashier-table-meta">
                          <span>{table.orderId ? <UserOutlined /> : <TableOutlined />}{table.dinerCount ? `${table.dinerCount}人` : `${table.capacity}人`}</span>
                          {table.openedAt && <span>{previewMode ? previewElapsed(table.name) : elapsedLabel(table.openedAt)}</span>}
                        </div>
                        {table.totalCents ? <b>{yuan(table.totalCents / 100)}</b> : null}
                      </button>
                    );
                  })}
                </div>
                {!visibleTables.length && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前区域暂无桌台" />}
                <div className="cashier-grid-count"><i />共 {visibleTables.length} 桌<i /></div>
              </>
            ) : (
              <div className="cashier-takeout-board">
                <div className="cashier-takeout-head"><strong>带走订单</strong><span>按取餐号处理制作、出餐和结账</span></div>
                <div className="cashier-takeout-grid">
                  {takeoutOrders.map((order) => (
                    <button type="button" className={`cashier-takeout-card status-${order.status.toLowerCase()} ${String(selectedOrder?.id) === String(order.id) ? 'selected' : ''}`} key={String(order.id)} onClick={() => selectTakeoutOrder(order)}>
                      <div><span>取餐号</span><strong>#{order.pickupNo || '--'}</strong><Tag>{cashierOrderStatusText(order)}</Tag></div>
                      <p>{order.items.map((item) => `${item.productName}×${item.quantity}`).join('、')}</p>
                      <footer><span>{dateTime(order.createdAt).slice(11, 16)}</span><b>{yuan(order.amount)}</b></footer>
                    </button>
                  ))}
                </div>
                {!takeoutOrders.length && <Empty description="暂无带走订单" />}
              </div>
            )}
          </section>

          {layoutMode === 'COMPACT' && <aside className="cashier-operation">
            <div className="cashier-operation-title">
              <strong>当前操作</strong>
              <Button type="text" icon={<ReloadOutlined spin={refreshing} />} onClick={() => void load(true)}>刷新</Button>
            </div>
            <div className="cashier-order-card">
              <div className="cashier-order-panel">
                <div className="cashier-order-hero">
                <Tag color={selectedOrder?.paymentStatus === 'UNPAID' ? 'error' : selectedOrder?.status === 'READY' ? 'cyan' : 'success'}>{selectedOrder ? cashierOrderStatusText(selectedOrder) : '空闲'}</Tag>
                <strong>{currentOrderLabel}</strong>
                {mode === 'DINE_IN' && <span>{selectedOrder?.dinerCount || selectedTable?.capacity || 0} 位</span>}
                {mode === 'DINE_IN' && selectedTable?.openedAt && <small>就餐时长 {previewMode ? previewElapsed(selectedTable.name) : elapsedLabel(selectedTable.openedAt)}</small>}
                </div>
                {selectedOrder ? (
                  <>
                  <div className="cashier-order-meta"><span>订单号：{selectedOrder.orderNo}</span><span>下单时间：{dateTime(selectedOrder.createdAt).slice(11, 16)}</span></div>
                  {Number(selectedOrder.additionCount) > 1 && <div className="cashier-addition-tags">{Array.from({ length: Number(selectedOrder.additionCount) - 1 }, (_, index) => <Tag key={index}>第{index + 1}次加菜</Tag>)}<span>共{selectedOrder.items.length}件商品</span></div>}
                  <div className="cashier-order-items">
                    {selectedOrder.items.map((item, index) => (
                      <Fragment key={String(item.id ?? index)}>
                        {Number(item.additionSequence || 1) > 1 && Number(selectedOrder.items[index - 1]?.additionSequence || 1) !== Number(item.additionSequence) && (
                          <div className="cashier-addition-divider"><i />第{Number(item.additionSequence) - 1}次加菜<i /></div>
                        )}
                        <div className="cashier-order-item">
                          <span>{item.productName}{item.skuName && item.skuName !== '默认' ? <small>{item.skuName}</small> : null}</span>
                          <b>×{item.quantity}</b>
                          <strong>{yuan(orderItemTotal(item))}</strong>
                        </div>
                      </Fragment>
                    ))}
                    {approvedReturnRequests.map((request) => (
                      <div className="cashier-order-item is-returned" key={`return-${String(request.id)}`}>
                        <span><i className="cashier-return-icon">退</i>{request.productName}<small>{request.reason}</small></span>
                        <b>−{request.quantity}</b>
                        <strong>−{yuan(request.amount)}</strong>
                      </div>
                    ))}
                  </div>
                  </>
                ) : (
                  <div className="cashier-empty-operation">
                    <CoffeeOutlined />
                    <strong>{selectedTable ? `${selectedTable.name} 当前空闲` : '请选择订单'}</strong>
                    <span>{selectedTable ? '可以直接为该桌点单开台' : '从左侧选择桌台或带走订单'}</span>
                  </div>
                )}
              </div>
              {selectedOrder && (
                <div className="cashier-order-total">
                  {Number(selectedOrder.memberDiscount || 0) > 0 && <div><span>会员优惠</span><b>-{yuan(selectedOrder.memberDiscount || 0)}</b></div>}
                  <div><span>合计</span><strong>{yuan(selectedOrder.remainingAmount ?? selectedOrder.amount)}</strong><em>{selectedOrder.paymentStatus === 'UNPAID' ? '待结账' : '已收款'}</em></div>
                </div>
              )}
            </div>

            <div className="cashier-action-dock">
              <div className={`cashier-primary-actions ${mode === 'TAKEOUT' ? 'is-takeout' : ''}`}>
                {mode === 'DINE_IN' && (
                  <Tooltip title={!canOpenSelectedTableOrder ? addBlockedReason : ''}>
                    <span className="cashier-action-wrapper">
                      <Button size="large" icon={<PlusOutlined />} disabled={!canOpenSelectedTableOrder} onClick={() => openOrdering('DINE_IN', selectedTable)}>{selectedTable?.orderId ? '加菜' : '点单开台'}</Button>
                    </span>
                  </Tooltip>
                )}
                {mode === 'DINE_IN' && <Button size="large" icon={<TeamOutlined />} disabled={!selectedOrder} onClick={updateDinerCount}>修改人数</Button>}
                <Button className="cashier-action-print" size="large" icon={<PrinterOutlined />} disabled={!selectedOrder} onClick={() => void printCustomerCopy()}>打印客户联/预结单</Button>
                <Button size="large" type="primary" danger icon={<WalletOutlined />} disabled={!selectedOrder || selectedOrder.paymentStatus !== 'UNPAID'} loading={submitting} onClick={settle}>结账 {selectedOrder?.paymentStatus === 'UNPAID' ? yuan(selectedOrder.remainingAmount ?? selectedOrder.amount) : ''}</Button>
              </div>
              {mode === 'DINE_IN' && selectedTable?.orderId && !canAddSelectedOrder && (
                <div className="cashier-order-lock-note"><LockOutlined />{addBlockedReason}</div>
              )}
              {mode === 'DINE_IN' && <div className="cashier-secondary-actions">
                <Tooltip title={transferBlockedReason}>
                  <span className="cashier-action-wrapper"><Button icon={<RetweetOutlined />} disabled={Boolean(transferBlockedReason)} onClick={transferTable}>转台</Button></span>
                </Tooltip>
                <Tooltip title={mergeBlockedReason}>
                  <span className="cashier-action-wrapper"><Button icon={<MergeCellsOutlined />} disabled={Boolean(mergeBlockedReason)} onClick={mergeTable}>并台</Button></span>
                </Tooltip>
                <Tooltip title={returnBlockedReason}>
                  <span className="cashier-action-wrapper"><Button icon={<MinusOutlined />} disabled={!canReturnSelectedOrder} onClick={() => { setReturnItemID(selectedOrder?.items[0]?.id); setReturnQuantity(1); setReturnReason(''); setReturnOpen(true); }}>退菜</Button></span>
                </Tooltip>
              </div>}
              {mode === 'DINE_IN' && <Dropdown
                trigger={['click']}
                menu={{
                  items: [
                    { key: 'transfer', icon: <RetweetOutlined />, label: transferBlockedReason ? `转台（${transferBlockedReason}）` : '转台', disabled: Boolean(transferBlockedReason), onClick: transferTable },
                    { key: 'merge', icon: <MergeCellsOutlined />, label: mergeBlockedReason ? `并台（${mergeBlockedReason}）` : '并台', disabled: Boolean(mergeBlockedReason), onClick: mergeTable },
                    { key: 'return', icon: <MinusOutlined />, label: returnBlockedReason ? `退菜（${returnBlockedReason}）` : '退菜', disabled: !canReturnSelectedOrder, onClick: () => { setReturnItemID(selectedOrder?.items[0]?.id); setReturnQuantity(1); setReturnReason(''); setReturnOpen(true); } },
                  ],
                }}
              >
                <Button className="cashier-more-actions" icon={<MoreOutlined />}>更多操作</Button>
              </Dropdown>}
              {selectedOrder && workflowAction && (
                <div className="cashier-status-actions">
                  <Button type="primary" block disabled={!workflowActionEnabled} onClick={() => void updateStatus(workflowAction.status)}>
                    {workflowActionEnabled ? workflowActionLabel : '请先结账，再完成订单/清台'}
                  </Button>
                </div>
              )}
            </div>
          </aside>}

          {layoutMode === 'STANDARD' && detailFocused && (
            <section className="cashier-standard-detail">
              <header className="cashier-standard-head">
                <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => setDetailFocused(false)}>
                  返回{mode === 'DINE_IN' ? '桌台列表' : '带走订单'}
                </Button>
                <div>
                  <strong>{currentOrderLabel}</strong>
                  <span>{selectedOrder ? `订单号 ${selectedOrder.orderNo}` : '当前桌台尚未开台'}</span>
                </div>
                <Tag color={selectedOrder?.paymentStatus === 'UNPAID' ? 'error' : selectedOrder ? 'success' : 'default'}>
                  {selectedOrder ? cashierOrderStatusText(selectedOrder) : '空闲'}
                </Tag>
                <Button type="text" icon={<ReloadOutlined spin={refreshing} />} onClick={() => void load(true)}>刷新</Button>
              </header>

              <div className="cashier-standard-grid">
                <section className="cashier-standard-bill">
                  <div className="cashier-standard-section-title">
                    <strong>订单与桌台操作</strong>
                    {selectedOrder && <span>{selectedOrder.dinerCount || 1} 位 · {dateTime(selectedOrder.createdAt).slice(11, 16)} 下单</span>}
                  </div>
                  {selectedOrder ? (
                    <>
                      <div className="cashier-checkout-bill-head"><span>商品</span><span>数量</span><span>金额</span></div>
                      <div className="cashier-standard-items">
                        {selectedOrder.items.map((item, index) => (
                          <Fragment key={String(item.id ?? index)}>
                            {Number(item.additionSequence || 1) > 1 && Number(selectedOrder.items[index - 1]?.additionSequence || 1) !== Number(item.additionSequence) && (
                              <div className="cashier-addition-divider"><i />第{Number(item.additionSequence) - 1}次加菜<i /></div>
                            )}
                            <div className="cashier-checkout-item">
                              <span><strong>{item.productName}</strong>{item.skuName && item.skuName !== '默认' && <small>{item.skuName}</small>}</span>
                              <b>×{item.quantity}</b>
                              <strong>{yuan(orderItemTotal(item))}</strong>
                            </div>
                          </Fragment>
                        ))}
                        {approvedReturnRequests.map((request) => (
                          <div className="cashier-checkout-item is-returned" key={`standard-return-${String(request.id)}`}>
                            <span><strong><i className="cashier-return-icon">退</i>{request.productName}</strong><small>原因：{request.reason}</small></span>
                            <b>−{request.quantity}</b>
                            <strong>−{yuan(request.amount)}</strong>
                          </div>
                        ))}
                      </div>
                    </>
                  ) : (
                    <div className="cashier-standard-empty">
                      <CoffeeOutlined />
                      <strong>{selectedTable?.name || '当前桌台'}为空闲桌台</strong>
                      <span>点击下方“点单开台”开始录单</span>
                    </div>
                  )}
                  <div className="cashier-standard-actions">
                    {mode === 'DINE_IN' && (
                      <Tooltip title={!canOpenSelectedTableOrder ? addBlockedReason : ''}>
                        <span><Button icon={<PlusOutlined />} disabled={!canOpenSelectedTableOrder} onClick={() => openOrdering('DINE_IN', selectedTable)}>{selectedTable?.orderId ? '加菜' : '点单开台'}</Button></span>
                      </Tooltip>
                    )}
                    {mode === 'DINE_IN' && <Button icon={<TeamOutlined />} disabled={!selectedOrder} onClick={updateDinerCount}>修改人数</Button>}
                    <Button icon={<PrinterOutlined />} disabled={!selectedOrder} onClick={() => void printCustomerCopy()}>打印预结单</Button>
                    {mode === 'DINE_IN' && <Button icon={<RetweetOutlined />} disabled={Boolean(transferBlockedReason)} onClick={transferTable}>转台</Button>}
                    {mode === 'DINE_IN' && <Button icon={<MergeCellsOutlined />} disabled={Boolean(mergeBlockedReason)} onClick={mergeTable}>并台</Button>}
                    {mode === 'DINE_IN' && <Button danger icon={<MinusOutlined />} disabled={!canReturnSelectedOrder} onClick={() => { setReturnItemID(selectedOrder?.items[0]?.id); setReturnQuantity(1); setReturnReason(''); setReturnOpen(true); }}>退菜</Button>}
                    {selectedOrder && workflowAction && (
                      <Button type="primary" disabled={!workflowActionEnabled} onClick={() => void updateStatus(workflowAction.status)}>
                        {workflowActionEnabled ? workflowActionLabel : '请先结账'}
                      </Button>
                    )}
                  </div>
                </section>

                <section className="cashier-standard-payment">
                  <div className="cashier-standard-section-title">
                    <strong>结账与支付</strong>
                    <span>选择顾客本次实际使用的支付方式</span>
                  </div>
                  <div className="cashier-standard-amount">
                    <span>{selectedOrder?.paymentStatus === 'UNPAID' ? '本次应收' : '订单实收'}</span>
                    <strong>{yuan(selectedOrder?.paymentStatus === 'UNPAID' ? orderRemainingAmount : orderPaidAmount || selectedOrder?.amount || 0)}</strong>
                    {Number(selectedOrder?.memberDiscount || 0) > 0 && <small>已优惠 {yuan(selectedOrder?.memberDiscount || 0)}</small>}
                  </div>
                  <div className="cashier-payment-methods">
                    <Button icon={<DollarOutlined />} disabled={!selectedOrder || selectedOrder.paymentStatus !== 'UNPAID'} onClick={() => void confirmSettlement('CASH')}>
                      <strong>现金收款</strong><small>确认现金已收妥</small>
                    </Button>
                    <Tooltip title={context?.wechatCodePaymentEnabled ? '' : context?.wechatCodePaymentReason}>
                      <span>
                        <Button
                          className="is-wechat"
                          icon={<WechatOutlined />}
                          disabled={!selectedOrder || selectedOrder.paymentStatus !== 'UNPAID' || !context?.wechatCodePaymentEnabled}
                          onClick={openWechatScanner}
                        >
                          <strong>微信付款码</strong><small>摄像头或扫码枪收款</small>
                        </Button>
                      </span>
                    </Tooltip>
                    <Button icon={<WalletOutlined />} disabled={!selectedOrder || selectedOrder.paymentStatus !== 'UNPAID'} onClick={() => void confirmSettlement('EXTERNAL')}>
                      <strong>系统外支付</strong><small>其他渠道已确认到账</small>
                    </Button>
                  </div>
                  {selectedOrder?.paymentStatus === 'PAID' && <Alert type="success" showIcon message="本单已完成收款" description="如需退款，请前往支付与退款页面操作。" />}
                  {!selectedOrder && <Alert type="info" showIcon message="请先点单开台" />}
                </section>

                <aside className="cashier-standard-payments">
                  <div className="cashier-standard-section-title">
                    <strong>支付记录</strong>
                    <span>{paymentRecords.length ? `共 ${paymentRecords.length} 笔` : '收款后自动记录'}</span>
                  </div>
                  {paymentRecords.length ? (
                    <div className="cashier-payment-record-list">
                      {paymentRecords.map((payment) => (
                        <article key={String(payment.id)}>
                          <div>
                            <strong>{cashierPaymentMethodLabel(payment)}</strong>
                            <Tag color={payment.status === 'SUCCESS' ? 'success' : ['PENDING', 'CREATING'].includes(payment.status) ? 'processing' : 'default'}>
                              {payment.status === 'SUCCESS' ? '支付成功' : payment.status === 'PENDING' ? '确认中' : payment.status === 'CREATING' ? '发起中' : payment.status === 'CLOSED' ? '已关闭' : payment.status}
                            </Tag>
                          </div>
                          <b>{yuan(payment.amount)}</b>
                          <span>{dateTime(payment.paidAt || payment.createdAt)}</span>
                          {payment.paymentNo && <small>{payment.paymentNo}</small>}
                        </article>
                      ))}
                    </div>
                  ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无支付记录" />}
                </aside>
              </div>
            </section>
          )}
        </div>

        <footer className="cashier-summary">
          <div><DashboardOutlined /><span>今日实收<strong>{yuan(dashboard.todayRevenue)}</strong><small>· {dashboard.todayOrders}单</small></span></div>
          <div><ClockCircleOutlined /><span>待结账<strong>{yuan(pendingAmount)}</strong></span></div>
          <div><ShoppingCartOutlined /><span>今日订单<strong>{dashboard.todayOrders} 单</strong></span></div>
          <div><WalletOutlined /><span>平均客单<strong>{yuan(dashboard.averageOrderValue)}</strong></span></div>
        </footer>
      </main>

      <Modal
        className="cashier-compact-checkout"
        width={520}
        footer={null}
        open={checkoutOpen && layoutMode === 'COMPACT'}
        onCancel={() => {
          if (wechatPaymentPending) {
            message.warning('微信支付结果仍在确认，请先查明结果或撤销本次收款');
            return;
          }
          setCheckoutOpen(false);
        }}
        closable={!wechatPaymentPending}
        maskClosable={!wechatPaymentPending}
        title={`${currentOrderLabel} · 选择结账方式`}
      >
        {selectedOrder && (
          <div className="cashier-compact-checkout-body">
            <div className="cashier-compact-due">
              <span>本次应收</span>
              <strong>{yuan(orderRemainingAmount)}</strong>
              <small>
                订单 {selectedOrder.orderNo}
                {Number(selectedOrder.memberDiscount || 0) > 0 ? ` · 已优惠 ${yuan(selectedOrder.memberDiscount || 0)}` : ''}
              </small>
            </div>
            <div className="cashier-payment-methods">
              <Button icon={<DollarOutlined />} onClick={() => void confirmSettlement('CASH')}>
                <strong>现金收款</strong><small>确认现金已收妥</small>
              </Button>
              <Tooltip title={context?.wechatCodePaymentEnabled ? '' : context?.wechatCodePaymentReason}>
                <span>
                  <Button
                    className="is-wechat"
                    icon={<WechatOutlined />}
                    disabled={!context?.wechatCodePaymentEnabled}
                    onClick={openWechatScanner}
                  >
                    <strong>微信付款码</strong>
                    <small>{context?.wechatCodePaymentEnabled ? '摄像头或扫码枪收款' : '等待商户支付配置'}</small>
                  </Button>
                </span>
              </Tooltip>
              <Button icon={<WalletOutlined />} onClick={() => void confirmSettlement('EXTERNAL')}>
                <strong>系统外支付</strong><small>其他渠道已确认到账</small>
              </Button>
            </div>
            {!context?.wechatCodePaymentEnabled && (
              <Alert
                type="info"
                showIcon
                message="微信付款码支付暂不可用"
                description={context?.wechatCodePaymentReason || '补充服务商、特约商户和 API 证书配置后自动开放。'}
              />
            )}
            <div className="cashier-compact-checkout-footer">
              <Button icon={<PrinterOutlined />} onClick={() => void printCustomerCopy()}>打印预结单</Button>
              <span><LockOutlined />结账期间锁定订单，避免重复收款</span>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        className="cashier-wechat-modal"
        width={620}
        title={<div className="cashier-wechat-title"><WechatOutlined /><span>微信付款码收款</span><strong>{yuan(orderRemainingAmount)}</strong></div>}
        open={wechatScanOpen}
        footer={null}
        closable={!wechatPaymentPending}
        maskClosable={false}
        destroyOnHidden
        onCancel={() => {
          if (!wechatPaymentPending) setWechatScanOpen(false);
        }}
      >
        <div className="cashier-wechat-scan">
          {!wechatPaymentPending && wechatPayment?.status !== 'SUCCESS' && (
            <>
              <div className="cashier-camera">
                <video ref={scanVideoRef} muted playsInline aria-label="微信付款码扫描画面" />
                <div className="cashier-camera-frame"><i /><span>将顾客微信付款码放入框内</span></div>
              </div>
              {cameraError && <Alert type="warning" showIcon message={cameraError} />}
              <div className="cashier-scanner-fallback">
                <span>扫码枪/手动输入</span>
                <Input.Password
                  value={wechatCodeInput}
                  autoComplete="off"
                  inputMode="numeric"
                  maxLength={18}
                  prefix={<QrcodeOutlined />}
                  placeholder="18 位微信付款码"
                  onChange={(event) => setWechatCodeInput(event.target.value.replace(/\D/g, '').slice(0, 18))}
                  onPressEnter={() => void submitWechatCode(wechatCodeInput)}
                />
                <Button type="primary" disabled={wechatCodeInput.length !== 18} onClick={() => void submitWechatCode(wechatCodeInput)}>确认收款</Button>
              </div>
            </>
          )}
          {wechatPayment && (
            <div className={`cashier-wechat-status status-${wechatPayment.status.toLowerCase()}`} aria-live="polite">
              {wechatPaymentPending ? <Spin size="large" /> : wechatPayment.status === 'SUCCESS' ? <CheckCircleOutlined /> : <QrcodeOutlined />}
              <strong>{wechatPayment.status === 'CREATING' ? '正在发起微信支付' : wechatPayment.status === 'PENDING' ? '正在确认支付结果' : wechatPayment.status === 'SUCCESS' ? '支付成功' : wechatPayment.status === 'CLOSED' ? '本次付款已撤销' : '请重新扫码'}</strong>
              <span>{wechatPayment.needCustomerAction ? '请顾客在手机上完成密码验证，请勿重复扫码。' : wechatPayment.message}</span>
              {wechatPayment.providerTransactionNo && <small>微信支付单号：{wechatPayment.providerTransactionNo}</small>}
              {wechatPaymentPending && <Button danger loading={submitting} onClick={() => void cancelWechatPayment()}>查单并撤销本次收款</Button>}
            </div>
          )}
          <p className="cashier-wechat-notice">付款码不会保存到摊伴系统；支付结果不明确时只查单，不会重复扣款。</p>
        </div>
      </Modal>

      <Drawer
        className="cashier-ordering-drawer"
        width="min(1080px, 96vw)"
        title={<div className="cashier-drawer-title"><strong>{cartMode === 'DINE_IN' ? `${cartTable?.name || ''} ${cartTable?.orderId ? '加菜' : '点单开台'}` : '新建带走单'}</strong><span>选择商品后确认下单</span></div>}
        open={cartOpen}
        onClose={() => setCartOpen(false)}
        footer={<div className="cashier-cart-footer"><span>已选 <b>{cart.reduce((sum, item) => sum + item.quantity, 0)}</b> 件</span><strong>{yuan(cartTotalCents / 100)}</strong><Button size="large" type="primary" disabled={!cart.length} loading={submitting} onClick={() => void submitOrder()}>确认下单并打印</Button></div>}
      >
        <div className="cashier-ordering">
          <div className="cashier-catalog">
            <div className="cashier-catalog-toolbar">
              <div className="cashier-categories">
                <button type="button" className={activeCategory === 'ALL' ? 'active' : ''} onClick={() => setActiveCategory('ALL')}>全部</button>
                {catalog.categories.map((category) => <button type="button" className={activeCategory === category.id ? 'active' : ''} onClick={() => setActiveCategory(category.id)} key={category.id}>{category.name}</button>)}
              </div>
              <Input allowClear value={productSearch} onChange={(event) => setProductSearch(event.target.value)} prefix={<AppstoreOutlined />} placeholder="搜索商品" />
            </div>
            <div className="cashier-product-grid">
              {filteredProducts.map((product) => (
                <button type="button" disabled={product.soldOut} className="cashier-product-card" key={product.id} onClick={() => configureProduct(product)}>
                  {product.imageUrl ? <img src={product.imageUrl} alt="" /> : <span className="cashier-product-fallback"><CoffeeOutlined /></span>}
                  <span><strong>{product.name}</strong><small>{product.description || `库存 ${product.stock}`}</small><b>{yuan(product.price / 100)}</b></span>
                  <i><PlusOutlined /></i>
                </button>
              ))}
            </div>
          </div>
          <aside className="cashier-cart">
            <div className="cashier-cart-head"><strong>当前已选</strong><Button type="link" disabled={!cart.length} onClick={() => setCart([])}>清空</Button></div>
            {cart.length ? cart.map((item) => (
              <div className="cashier-cart-line" key={item.key}>
                <div><strong>{item.name}</strong>{item.summary && <small>{item.summary}</small>}{item.itemRemark && <small>备注：{item.itemRemark}</small>}<b>{yuan(item.priceCents / 100)}</b></div>
                <Space.Compact><Button icon={<MinusOutlined />} onClick={() => changeCartQuantity(item.key, -1)} /><span>{item.quantity}</span><Button icon={<PlusOutlined />} onClick={() => changeCartQuantity(item.key, 1)} /></Space.Compact>
              </div>
            )) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="点击左侧商品加入订单" />}
          </aside>
        </div>
      </Drawer>

      <Modal title={configuringProduct?.name || '选择规格'} open={Boolean(configuringProduct)} onCancel={() => setConfiguringProduct(null)} okText={`加入订单 · ${configuringProduct ? yuan(((configuringProduct.skus.find((item) => item.id === skuID)?.price || 0) * itemQuantity) / 100) : ''}`} onOk={() => configuringProduct && addConfiguredProduct(configuringProduct, skuID, optionSelections, modifierSelections, itemQuantity, itemRemark)}>
        {configuringProduct && <div className="cashier-product-config">
          {configuringProduct.skus.length > 1 && <section><strong>规格</strong><Radio.Group value={skuID} onChange={(event) => setSkuID(Number(event.target.value))}>{configuringProduct.skus.map((sku) => <Radio.Button disabled={sku.soldOut} value={sku.id} key={sku.id}>{sku.name} · {yuan(sku.price / 100)}</Radio.Button>)}</Radio.Group></section>}
          {configuringProduct.optionGroups.map((group) => <section key={group.id}><strong>{group.name}<small>选 {group.minSelect}{group.maxSelect !== group.minSelect ? `–${group.maxSelect}` : ''} 项</small></strong>{group.selectionMode === 'SINGLE'
            ? <Radio.Group value={optionSelections[group.id]?.[0]} onChange={(event) => setOptionSelections((current) => ({ ...current, [group.id]: [Number(event.target.value)] }))}>{group.values.map((item) => <Radio.Button value={item.id} key={item.id}>{item.name}{item.priceDeltaCents ? ` +${yuan(item.priceDeltaCents / 100)}` : ''}</Radio.Button>)}</Radio.Group>
            : <Checkbox.Group value={optionSelections[group.id]} onChange={(values) => setOptionSelections((current) => ({ ...current, [group.id]: values.map(Number).slice(0, group.maxSelect) }))}>{group.values.map((item) => <Checkbox value={item.id} key={item.id}>{item.name}{item.priceDeltaCents ? ` +${yuan(item.priceDeltaCents / 100)}` : ''}</Checkbox>)}</Checkbox.Group>}</section>)}
          {configuringProduct.modifierGroups.map((group) => <section key={group.id}><strong>{group.name}<small>选 {group.minSelect}{group.maxSelect !== group.minSelect ? `–${group.maxSelect}` : ''} 项</small></strong><Checkbox.Group value={modifierSelections[group.id]} onChange={(values) => setModifierSelections((current) => ({ ...current, [group.id]: values.map(Number).slice(0, group.maxSelect) }))}>{group.items.map((item) => <Checkbox value={item.id} key={item.id}>{item.name}{item.priceCents ? ` +${yuan(item.priceCents / 100)}` : ''}</Checkbox>)}</Checkbox.Group></section>)}
          <section><strong>数量</strong><InputNumber min={1} max={99} value={itemQuantity} onChange={(value) => setItemQuantity(Number(value || 1))} /></section>
          <section><strong>单品备注</strong><Input.TextArea value={itemRemark} onChange={(event) => setItemRemark(event.target.value)} maxLength={100} placeholder="例如：少盐、不要葱" /></section>
        </div>}
      </Modal>

      <Modal title="确认退菜" open={returnOpen} onCancel={() => setReturnOpen(false)} okText="确认退菜并更新账单" okButtonProps={{ danger: true }} confirmLoading={submitting} onOk={() => void submitReturn()}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert type="warning" showIcon message="确认后立即生效" description="系统会直接扣减订单金额、恢复对应库存，并在订单记录中保留一条负金额退菜明细。" />
          <Select value={returnItemID} onChange={(value) => { setReturnItemID(value); setReturnQuantity(1); }} options={selectedOrder?.items.filter((item) => item.id).map((item) => ({ label: `${item.productName} ×${item.quantity}`, value: item.id! }))} placeholder="选择退菜商品" style={{ width: '100%' }} />
          <div className="cashier-number-field">
            <InputNumber value={returnQuantity} onChange={(value) => setReturnQuantity(Number(value || 1))} min={1} max={selectedOrder?.items.find((item) => String(item.id) === String(returnItemID))?.quantity || 1} />
            <span>份</span>
          </div>
          <Select
            value={returnReason || undefined}
            onChange={setReturnReason}
            options={returnReasonOptions.map((reason) => ({ label: reason, value: reason }))}
            placeholder="选择退菜原因"
            style={{ width: '100%' }}
          />
        </Space>
      </Modal>

      <Modal title="交接班" open={handoverOpen} onCancel={() => setHandoverOpen(false)} okText="确认交接并退出收银台" cancelText="继续本班" confirmLoading={submitting} onOk={() => void submitHandover()}>
        <div className="cashier-handover-modal">
          <p>用于把未结账桌台、设备情况和异常事项交给下一位收银员。系统会记录操作人、交接时间和备注，完成后退出当前收银终端。</p>
          <div className="cashier-handover-summary">
            <span>今日实收<strong>{yuan(dashboard.todayRevenue)}</strong></span>
            <span>今日订单<strong>{dashboard.todayOrders} 单</strong></span>
            <span>待结账<strong>{unsettledTables.length} 桌 / {yuan(pendingAmount)}</strong></span>
          </div>
          <Alert type="info" showIcon message="当前为轻量交接记录" description="上方当日概览用于现场核对；系统暂不管理实体钱箱的备用金、现金实点和长短款，现金差额仍需按门店制度线下核对。" />
          <Input.TextArea value={handoverRemark} onChange={(event) => setHandoverRemark(event.target.value)} maxLength={255} rows={3} showCount placeholder="交接备注（可选），例如待处理订单、设备情况" />
        </div>
      </Modal>

      {(loading && !previewMode) && <div className="cashier-loading"><Spin size="large" /><span>正在同步桌台与订单...</span></div>}
    </div>
  );
}
