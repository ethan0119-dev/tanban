export type EntityStatus = 'active' | 'disabled' | 'pending';

export interface CurrentUser {
  id: string;
  name: string;
  username: string;
  phone?: string;
  email?: string;
  role: string;
  status?: EntityStatus;
  lastLoginAt?: string;
}

export interface LoginResponse {
  token?: string;
  accessToken?: string;
  access_token?: string;
  expiresIn?: number;
  user?: CurrentUser;
}

export interface PageMeta {
  page: number;
  pageSize: number;
  total: number;
}

export interface PageResult<T> {
  items: T[];
  meta: PageMeta;
}

export interface QueryParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
  status?: string;
  tenantId?: string;
  [key: string]: string | number | boolean | undefined;
}

export interface DashboardMetric {
  label: string;
  value: number;
  suffix?: string;
  trend?: number;
}

export interface TrendPoint {
  date: string;
  tenants?: number;
  stores?: number;
  orders?: number;
  amount?: number;
}

export interface DashboardData {
  tenantCount?: number;
  activeTenantCount?: number;
  storeCount?: number;
  todayOrderCount?: number;
  todayTransactionAmount?: number;
  monthTransactionAmount?: number;
  metrics?: DashboardMetric[];
  trend?: TrendPoint[];
  recentTenants?: Tenant[];
}

export interface PlatformUser extends CurrentUser {
  createdAt?: string;
  updatedAt?: string;
}

export interface Tenant {
  id: string;
  name: string;
  code?: string;
  contactName?: string;
  contactPhone?: string;
  legalName?: string;
  status: EntityStatus;
  storeCount?: number;
  orderCount?: number;
  ownerUsername?: string;
  ownerDisplayName?: string;
  ownerStatus?: EntityStatus;
  hasOwner?: boolean;
  paymentMerchantNo?: string;
  paymentProvider?: string;
  paymentSubAppId?: string;
  paymentStatus?: 'unbound' | 'pending' | 'active' | 'rejected';
  createdAt?: string;
  expiresAt?: string;
}

export interface Store {
  id: string;
  tenantId?: string;
  tenantName?: string;
  name: string;
  code?: string;
  phone?: string;
  address?: string;
  businessHours?: string;
  logoUrl?: string;
  bannerUrl?: string;
  notice?: string;
  status: EntityStatus;
  orderCount?: number;
  createdAt?: string;
}

export interface AuditLog {
  id: string;
  operatorName?: string;
  operatorId?: string;
  action: string;
  module?: string;
  target?: string;
  ip?: string;
  detail?: string;
  createdAt: string;
}

export interface PaymentSettings {
  provider: 'mock' | 'tianque';
  enabled: boolean;
  environment?: 'sandbox' | 'production';
  orgId?: string;
  apiBaseUrl?: string;
  notifyUrl?: string;
  publicKeyConfigured?: boolean;
  privateKeyConfigured?: boolean;
  effectiveProvider?: 'mock' | 'tianque';
  restartRequired?: boolean;
  tianqueAdapterImplemented?: boolean;
  updatedAt?: string;
}

export interface SystemSettings {
  platformName?: string;
  supportPhone?: string;
  supportEmail?: string;
  orderExpireMinutes?: number;
  loginFailureLimit?: number;
  sessionExpireMinutes?: number;
}
