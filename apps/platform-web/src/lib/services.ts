import { http, normalizePage } from './api';
import type {
  AuditLog,
  CurrentUser,
  DashboardData,
  LoginResponse,
  PageResult,
  PaymentSettings,
  PlatformAnnouncement,
  AnnouncementValues,
  PlatformUser,
  QueryParams,
  Store,
  SystemSettings,
  Tenant,
  TrendPoint,
} from '../types';

type RawRecord = Record<string, unknown>;

function raw(value: unknown): RawRecord {
  return value && typeof value === 'object' ? (value as RawRecord) : {};
}

function text(value: unknown): string {
  return value === null || value === undefined ? '' : String(value);
}

function numberValue(value: unknown): number {
  const result = Number(value || 0);
  return Number.isFinite(result) ? result : 0;
}

function statusValue(value: unknown): 'active' | 'disabled' | 'pending' {
  const normalized = text(value).toLowerCase();
  if (normalized === 'disabled') return 'disabled';
  if (normalized === 'pending') return 'pending';
  return 'active';
}

function toBackendStatus(value?: string): string {
  return (value || 'active').toUpperCase();
}

function currentUserFromRaw(value: unknown): CurrentUser {
  const item = raw(value);
  return {
    id: text(item.id ?? item.user_id),
    name: text(item.name ?? item.display_name ?? item.username),
    username: text(item.username),
    role: text(item.role),
    status: statusValue(item.status),
    lastLoginAt: text(item.last_login_at ?? item.lastLoginAt) || undefined,
  };
}

function platformUserFromRaw(value: unknown): PlatformUser {
  const item = raw(value);
  return {
    ...currentUserFromRaw(item),
    createdAt: text(item.created_at ?? item.createdAt) || undefined,
    updatedAt: text(item.updated_at ?? item.updatedAt) || undefined,
  };
}

function tenantFromRaw(value: unknown): Tenant {
  const item = raw(value);
  const merchantNo = text(item.payment_merchant_no ?? item.paymentMerchantNo);
  return {
    id: text(item.id),
    code: text(item.code) || undefined,
    name: text(item.name),
    contactName: text(item.contact_name ?? item.contactName) || undefined,
    contactPhone: text(item.contact_phone ?? item.contactPhone) || undefined,
    status: statusValue(item.status),
    storeCount: numberValue(item.store_count ?? item.storeCount),
    orderCount: numberValue(item.order_count ?? item.orderCount),
    ownerUsername: text(item.owner_username ?? item.ownerUsername) || undefined,
    ownerDisplayName: text(item.owner_display_name ?? item.ownerDisplayName) || undefined,
    ownerStatus: text(item.owner_status ?? item.ownerStatus) ? statusValue(item.owner_status ?? item.ownerStatus) : undefined,
    hasOwner: Boolean(item.has_owner ?? item.hasOwner ?? item.owner_username ?? item.ownerUsername),
    paymentMerchantNo: merchantNo || undefined,
    paymentProvider: text(item.payment_provider ?? item.paymentProvider) || undefined,
    paymentSubAppId: text(item.payment_sub_appid ?? item.paymentSubAppId) || undefined,
    businessLicenseUrl: text(item.business_license_url ?? item.businessLicenseUrl) || undefined,
    foodBusinessLicenseUrl: text(item.food_business_license_url ?? item.foodBusinessLicenseUrl) || undefined,
    paymentStatus: text(item.payment_status ?? item.paymentStatus).toLowerCase() as Tenant['paymentStatus'] || (merchantNo ? 'active' : 'unbound'),
    createdAt: text(item.created_at ?? item.createdAt) || undefined,
    expiresAt: text(item.expires_at ?? item.expiresAt) || undefined,
  };
}

function storeFromRaw(value: unknown): Store {
  const item = raw(value);
  return {
    id: text(item.id),
    tenantId: text(item.tenant_id ?? item.tenantId) || undefined,
    tenantName: text(item.tenant_name ?? item.tenantName) || undefined,
    code: text(item.code) || undefined,
    name: text(item.name),
    phone: text(item.phone) || undefined,
    address: text(item.address) || undefined,
    businessHours: text(item.business_hours ?? item.businessHours) || undefined,
    logoUrl: text(item.logo_url ?? item.logoUrl) || undefined,
    bannerUrl: text(item.banner_url ?? item.bannerUrl) || undefined,
    notice: text(item.notice) || undefined,
    status: statusValue(item.status),
    orderCount: numberValue(item.order_count ?? item.orderCount),
    createdAt: text(item.created_at ?? item.createdAt) || undefined,
  };
}

function auditLogFromRaw(value: unknown): AuditLog {
  const item = raw(value);
  const actorID = text(item.actor_user_id ?? item.operatorId);
  const resourceType = text(item.resource_type ?? item.module);
  const resourceID = text(item.resource_id);
  return {
    id: text(item.id),
    operatorId: actorID || undefined,
    operatorName: text(item.operator_name ?? item.operatorName) || (actorID ? `用户 #${actorID}` : '系统任务'),
    action: text(item.action),
    module: resourceType || undefined,
    target: text(item.target) || (resourceType && resourceID ? `${resourceType} #${resourceID}` : undefined),
    ip: text(item.ip) || undefined,
    detail: text(item.details ?? item.detail) || undefined,
    createdAt: text(item.created_at ?? item.createdAt),
  };
}

function announcementFromRaw(value: unknown): PlatformAnnouncement {
  const item = raw(value);
  const tenantIds = item.tenant_ids ?? item.tenantIds;
  return {
    id: text(item.id),
    title: text(item.title),
    summary: text(item.summary),
    content: text(item.content),
    category: text(item.category).toUpperCase() as PlatformAnnouncement['category'],
    severity: text(item.severity).toUpperCase() as PlatformAnnouncement['severity'],
    audienceType: text(item.audience_type ?? item.audienceType).toUpperCase() as PlatformAnnouncement['audienceType'],
    status: text(item.status).toUpperCase() as PlatformAnnouncement['status'],
    tenantIds: Array.isArray(tenantIds) ? tenantIds.map(text) : [],
    targetCount: numberValue(item.target_count ?? item.targetCount),
    readCount: numberValue(item.read_count ?? item.readCount),
    createdBy: text(item.created_by ?? item.createdBy) || undefined,
    createdAt: text(item.created_at ?? item.createdAt),
    updatedAt: text(item.updated_at ?? item.updatedAt),
    publishedAt: text(item.published_at ?? item.publishedAt) || undefined,
    withdrawnAt: text(item.withdrawn_at ?? item.withdrawnAt) || undefined,
  };
}

function announcementPayload(values: AnnouncementValues): RawRecord {
  return {
    title: values.title,
    summary: values.summary || '',
    content: values.content,
    category: values.category,
    severity: values.severity,
    audience_type: values.audienceType,
    tenant_ids: (values.tenantIds || []).map(Number),
  };
}

function backendParams(params: QueryParams = {}): QueryParams {
  const { pageSize, keyword, tenantId, status, ...rest } = params;
  return {
    ...rest,
    page_size: pageSize,
    q: keyword,
    tenant_id: tenantId,
    status: typeof status === 'string' ? status.toUpperCase() : status,
  };
}

async function getPage<T>(
  path: string,
  params: QueryParams = {},
  mapper: (value: unknown) => T,
): Promise<PageResult<T>> {
  const result = await http.get<unknown>(path, backendParams(params));
  const page = normalizePage<unknown>(result.data, result.meta, Number(params.page || 1), Number(params.pageSize || 20));
  return { items: page.items.map(mapper), meta: page.meta };
}

function userPayload(values: Partial<PlatformUser> & { password?: string }): RawRecord {
  return {
    tenant_id: 0,
    username: values.username || '',
    password: values.password || '',
    display_name: values.name || '',
    role: (values.role || 'PLATFORM_OPERATOR').toUpperCase(),
    status: toBackendStatus(values.status),
  };
}

export type TenantCreateValues = Partial<Tenant> & {
  ownerUsername?: string;
  ownerPassword?: string;
  ownerDisplayName?: string;
  initialStoreCode?: string;
  initialStoreName?: string;
};

function tenantPayload(values: TenantCreateValues): RawRecord {
	const item = values;
  return {
    code: item.code || '',
    name: item.name || '',
    contact_name: item.contactName || '',
    contact_phone: item.contactPhone || '',
    status: toBackendStatus(item.status),
    payment_provider: item.paymentProvider || 'mock',
    payment_merchant_no: item.paymentMerchantNo || '',
    payment_sub_appid: item.paymentSubAppId || '',
    owner_username: item.ownerUsername || '',
    owner_password: item.ownerPassword || '',
    owner_display_name: item.ownerDisplayName || '',
    initial_store_code: item.initialStoreCode || '',
    initial_store_name: item.initialStoreName || '',
  };
}

function storePayload(values: Partial<Store>): RawRecord {
  const item = values as Partial<Store> & { logoUrl?: string; bannerUrl?: string; notice?: string };
  return {
    code: item.code || '',
    name: item.name || '',
    logo_url: item.logoUrl || '',
    banner_url: item.bannerUrl || '',
    address: item.address || '',
    phone: item.phone || '',
    business_hours: item.businessHours || '',
    notice: item.notice || '',
    status: toBackendStatus(item.status),
  };
}

export const authService = {
  login: async (account: string, password: string): Promise<LoginResponse> => {
    const response = await http.post<RawRecord>('/auth/login', { username: account, password, portal: 'platform' });
    const result = raw(response.data);
    return {
      token: text(result.token) || undefined,
      accessToken: text(result.accessToken) || undefined,
      access_token: text(result.access_token) || undefined,
      expiresIn: numberValue(result.expiresIn ?? result.expires_in),
      user: result.user ? currentUserFromRaw(result.user) : undefined,
    };
  },
  me: async (): Promise<CurrentUser> => currentUserFromRaw((await http.get<RawRecord>('/auth/me')).data),
};

export const dashboardService = {
  get: async (): Promise<DashboardData> => {
    const item = raw((await http.get<RawRecord>('/platform/dashboard')).data);
    const recentTenants = item.recentTenants ?? item.recent_tenants;
    const trend = item.trend;
    return {
      ...(item as DashboardData),
      tenantCount: numberValue(item.tenantCount ?? item.tenant_count),
      activeTenantCount: numberValue(item.activeTenantCount ?? item.active_tenant_count),
      storeCount: numberValue(item.storeCount ?? item.store_count),
      todayOrderCount: numberValue(item.todayOrderCount ?? item.today_order_count),
      // 平台总览接口中的交易金额统一以“分”传输，展示层统一使用“元”。
      todayTransactionAmount: numberValue(item.today_revenue_cents ?? item.todayTransactionAmount ?? item.today_transaction_amount) / 100,
      monthTransactionAmount: numberValue(item.month_revenue_cents ?? item.monthTransactionAmount ?? item.month_transaction_amount) / 100,
      trend: Array.isArray(trend)
        ? trend.map((point) => {
          const value = raw(point);
          return {
            date: text(value.date),
            tenants: value.tenants === undefined ? undefined : numberValue(value.tenants),
            stores: value.stores === undefined ? undefined : numberValue(value.stores),
            orders: value.orders === undefined ? undefined : numberValue(value.orders),
            amount: numberValue(value.amount_cents ?? value.amount) / 100,
          } satisfies TrendPoint;
        })
        : [],
      recentTenants: Array.isArray(recentTenants) ? recentTenants.map(tenantFromRaw) : [],
    };
  },
};

export const userService = {
  list: (params?: QueryParams) => getPage<PlatformUser>('/platform/users', params, platformUserFromRaw),
  create: async (values: Partial<PlatformUser> & { password: string }) =>
    platformUserFromRaw((await http.post<RawRecord>('/platform/users', userPayload(values))).data),
  update: async (id: string, values: Partial<PlatformUser>) =>
    platformUserFromRaw((await http.put<RawRecord>(`/platform/users/${id}`, userPayload(values))).data),
  resetPassword: async (user: PlatformUser, password: string) =>
    platformUserFromRaw((await http.put<RawRecord>(`/platform/users/${user.id}`, userPayload({ ...user, password }))).data),
};

export const tenantService = {
  list: (params?: QueryParams) => getPage<Tenant>('/platform/tenants', params, tenantFromRaw),
  create: async (values: TenantCreateValues) =>
    tenantFromRaw((await http.post<RawRecord>('/platform/tenants', tenantPayload(values))).data),
  update: async (id: string, values: Partial<Tenant>) =>
    tenantFromRaw((await http.put<RawRecord>(`/platform/tenants/${id}/`, tenantPayload(values))).data),
  createOwner: async (id: string, values: { username: string; password: string; displayName: string }) => {
    await http.post(`/platform/tenants/${id}/owner`, {
      username: values.username,
      password: values.password,
      display_name: values.displayName,
    });
  },
  uploadDocument: async (id: string, type: 'business-license' | 'food-business-license', file: File) => {
    const body = new FormData();
    body.append('file', file);
    return tenantFromRaw((await http.postForm<RawRecord>(`/platform/tenants/${id}/documents/${type}`, body)).data);
  },
};

export const storeService = {
  list: (params?: QueryParams) => getPage<Store>('/platform/stores', params, storeFromRaw),
  get: async (tenantId: string, id: string) =>
    storeFromRaw((await http.get<RawRecord>(`/platform/tenants/${tenantId}/stores/${id}/`)).data),
  create: async (values: Partial<Store>) => {
    if (!values.tenantId) throw new Error('创建门店必须指定所属商户');
    return storeFromRaw((await http.post<RawRecord>(`/platform/tenants/${values.tenantId}/stores`, storePayload(values))).data);
  },
  update: async (tenantId: string, id: string, values: Partial<Store>) => {
    // 列表接口不返回 logo/banner/notice，PUT 又是全量更新。先取详情再合并，
    // 防止只改状态或基本信息时把店铺装修内容清空。
    const existing = storeFromRaw((await http.get<RawRecord>(
      `/platform/tenants/${tenantId}/stores/${id}/`,
    )).data);
    const supplied = Object.fromEntries(
      Object.entries(values).filter(([, value]) => value !== undefined),
    ) as Partial<Store>;
    const merged = { ...existing, ...supplied, tenantId };
    return storeFromRaw((await http.put<RawRecord>(
      `/platform/tenants/${tenantId}/stores/${id}/`,
      storePayload(merged),
    )).data);
  },
};

export const auditService = {
  list: (params?: QueryParams) => getPage<AuditLog>('/platform/audit-logs', params, auditLogFromRaw),
};

export const announcementService = {
  list: (params?: QueryParams) => getPage<PlatformAnnouncement>('/platform/announcements', params, announcementFromRaw),
  get: async (id: string) => announcementFromRaw((await http.get<RawRecord>(`/platform/announcements/${id}`)).data),
  create: async (values: AnnouncementValues) => announcementFromRaw((await http.post<RawRecord>('/platform/announcements', announcementPayload(values))).data),
  update: async (id: string, values: AnnouncementValues) => announcementFromRaw((await http.put<RawRecord>(`/platform/announcements/${id}`, announcementPayload(values))).data),
  publish: async (id: string) => announcementFromRaw((await http.post<RawRecord>(`/platform/announcements/${id}/publish`)).data),
  withdraw: async (id: string) => announcementFromRaw((await http.post<RawRecord>(`/platform/announcements/${id}/withdraw`)).data),
};

export const settingsService = {
  getPayment: async () => (await http.get<PaymentSettings>('/platform/settings/payment')).data,
  updatePayment: async (values: Partial<PaymentSettings>) =>
    (await http.put<PaymentSettings>('/platform/settings/payment', values)).data,
  getSystem: async () => (await http.get<SystemSettings>('/platform/settings/system')).data,
  updateSystem: async (values: SystemSettings) =>
    (await http.put<SystemSettings>('/platform/settings/system', values)).data,
};
