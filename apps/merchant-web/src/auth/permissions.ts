import type { MerchantUser } from '../types';

export type MerchantRole = 'MERCHANT_OWNER' | 'MERCHANT_MANAGER' | 'MERCHANT_STAFF';

export type MerchantCapability =
  | 'VIEW_DASHBOARD'
  | 'VIEW_FINANCIALS'
  | 'MANAGE_ORDERS'
  | 'OPERATE_PRINT_JOBS'
  | 'VIEW_NOTIFICATIONS'
  | 'MANAGE_CATALOG'
  | 'MANAGE_STORE'
  | 'MANAGE_DECORATION'
  | 'MANAGE_MARKETING'
  | 'MANAGE_MEMBERS'
  | 'VIEW_PAYMENTS'
  | 'CREATE_REFUNDS'
  | 'MANAGE_STAFF'
  | 'MANAGE_ALL_STAFF'
  | 'ARCHIVE_CUSTOMERS'
  | 'ADJUST_CUSTOMER_BALANCE'
  | 'CREATE_STORED_VALUE_RECORD';

export const MERCHANT_CAPABILITY_LABELS: Record<MerchantCapability, string> = {
  VIEW_DASHBOARD: '经营总览',
  VIEW_FINANCIALS: '经营数据',
  MANAGE_ORDERS: '订单查看与状态处理',
  OPERATE_PRINT_JOBS: '打印与补打',
  VIEW_NOTIFICATIONS: '通知中心',
  MANAGE_CATALOG: '商品与库存',
  MANAGE_STORE: '门店设置',
  MANAGE_DECORATION: '店铺装修',
  MANAGE_MARKETING: '营销活动',
  MANAGE_MEMBERS: '用户与会员',
  VIEW_PAYMENTS: '支付记录',
  CREATE_REFUNDS: '退款',
  MANAGE_STAFF: '店员账号管理',
  MANAGE_ALL_STAFF: '全部员工管理',
  ARCHIVE_CUSTOMERS: '顾客归档',
  ADJUST_CUSTOMER_BALANCE: '余额调账',
  CREATE_STORED_VALUE_RECORD: '手工储值',
};

const COMMON_CAPABILITIES: MerchantCapability[] = [
  'VIEW_DASHBOARD',
  'MANAGE_ORDERS',
  'OPERATE_PRINT_JOBS',
  'VIEW_NOTIFICATIONS',
];

const MANAGEMENT_CAPABILITIES: MerchantCapability[] = [
  'VIEW_FINANCIALS',
  'MANAGE_CATALOG',
  'MANAGE_STORE',
  'MANAGE_DECORATION',
  'MANAGE_MARKETING',
  'MANAGE_MEMBERS',
  'VIEW_PAYMENTS',
  'CREATE_REFUNDS',
  'MANAGE_STAFF',
];

export const MERCHANT_ROLE_CAPABILITIES: Record<MerchantRole, readonly MerchantCapability[]> = {
  MERCHANT_OWNER: [
    ...COMMON_CAPABILITIES,
    ...MANAGEMENT_CAPABILITIES,
    'MANAGE_ALL_STAFF',
    'ARCHIVE_CUSTOMERS',
    'ADJUST_CUSTOMER_BALANCE',
    'CREATE_STORED_VALUE_RECORD',
  ],
  MERCHANT_MANAGER: [...COMMON_CAPABILITIES, ...MANAGEMENT_CAPABILITIES],
  MERCHANT_STAFF: [...COMMON_CAPABILITIES],
};

export const MERCHANT_ROLE_NAMES: Record<MerchantRole, string> = {
  MERCHANT_OWNER: '老板',
  MERCHANT_MANAGER: '店长',
  MERCHANT_STAFF: '店员',
};

const ROLE_ALIASES: Record<string, MerchantRole> = {
  OWNER: 'MERCHANT_OWNER',
  MERCHANT_OWNER: 'MERCHANT_OWNER',
  MANAGER: 'MERCHANT_MANAGER',
  MERCHANT_MANAGER: 'MERCHANT_MANAGER',
  STAFF: 'MERCHANT_STAFF',
  MERCHANT_STAFF: 'MERCHANT_STAFF',
};

export function normalizeMerchantRole(role: unknown): MerchantRole | undefined {
  return ROLE_ALIASES[String(role ?? '').trim().toUpperCase()];
}

export function primaryMerchantRole(user: MerchantUser | null | undefined): string {
  return String(user?.roles?.[0] ?? '').trim().toUpperCase();
}

export function canonicalMerchantRole(user: MerchantUser | null | undefined): MerchantRole | undefined {
  return normalizeMerchantRole(primaryMerchantRole(user));
}

export function capabilitiesForMerchantRole(role: unknown): readonly MerchantCapability[] {
  const canonicalRole = normalizeMerchantRole(role);
  return canonicalRole ? MERCHANT_ROLE_CAPABILITIES[canonicalRole] : [];
}

export function hasMerchantCapability(
  user: MerchantUser | null | undefined,
  capability: MerchantCapability,
): boolean {
  if (Array.isArray(user?.capabilities)) {
    return user.capabilities.includes(capability);
  }
  const role = canonicalMerchantRole(user);
  return role ? MERCHANT_ROLE_CAPABILITIES[role].includes(capability) : false;
}

export function isMerchantStaff(user: MerchantUser | null | undefined): boolean {
  return canonicalMerchantRole(user) === 'MERCHANT_STAFF';
}

export function canManageMerchant(user: MerchantUser | null | undefined): boolean {
  return hasMerchantCapability(user, 'MANAGE_STORE');
}

export function canViewMerchantFinancials(user: MerchantUser | null | undefined): boolean {
  return hasMerchantCapability(user, 'VIEW_FINANCIALS');
}

export function isMerchantManager(user: MerchantUser | null | undefined): boolean {
  return canonicalMerchantRole(user) === 'MERCHANT_MANAGER';
}

export function isMerchantOwner(user: MerchantUser | null | undefined): boolean {
  return canonicalMerchantRole(user) === 'MERCHANT_OWNER';
}

export function assignableMerchantRoles(user: MerchantUser | null | undefined): MerchantRole[] {
  if (hasMerchantCapability(user, 'MANAGE_ALL_STAFF')) {
    return ['MERCHANT_OWNER', 'MERCHANT_MANAGER', 'MERCHANT_STAFF'];
  }
  return hasMerchantCapability(user, 'MANAGE_STAFF') ? ['MERCHANT_STAFF'] : [];
}

export function canManageStaffRole(
  user: MerchantUser | null | undefined,
  targetRole: string,
): boolean {
  if (hasMerchantCapability(user, 'MANAGE_ALL_STAFF')) return true;
  return hasMerchantCapability(user, 'MANAGE_STAFF')
    && normalizeMerchantRole(targetRole) === 'MERCHANT_STAFF';
}
