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
  orders?: number;
  amount?: number;
}

export interface DashboardData {
  tenantCount?: number;
  activeTenantCount?: number;
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
  cashierEnabled: boolean;
  storeId?: string;
  storeCode?: string;
  storeName?: string;
  orderCount?: number;
  ownerUsername?: string;
  ownerDisplayName?: string;
  ownerStatus?: EntityStatus;
  hasOwner?: boolean;
  paymentMerchantNo?: string;
  paymentProvider?: string;
  paymentSubAppId?: string;
  businessLicenseUrl?: string;
  foodBusinessLicenseUrl?: string;
  paymentStatus?: 'unbound' | 'pending' | 'active' | 'rejected';
  createdAt?: string;
  expiresAt?: string;
  serviceExpired?: boolean;
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
  provider: 'mock' | 'tianque' | 'wechat_partner';
  enabled: boolean;
  environment?: 'sandbox' | 'production';
  orgId?: string;
  apiBaseUrl?: string;
  notifyUrl?: string;
  refundNotifyUrl?: string;
  spMchId?: string;
  spAppId?: string;
  microOnboardingPermissionStatus?: 'NOT_APPLIED' | 'APPLYING' | 'ENABLED' | 'REJECTED';
  publicKeyConfigured?: boolean;
  privateKeyConfigured?: boolean;
  apiCertSerialConfigured?: boolean;
  apiV3KeyConfigured?: boolean;
  wechatPayPublicKeyConfigured?: boolean;
  effectiveProvider?: 'mock' | 'tianque' | 'wechat_partner';
  restartRequired?: boolean;
  tianqueAdapterImplemented?: boolean;
  wechatPartnerAdapterImplemented?: boolean;
  wechatPartnerConfigured?: boolean;
  updatedAt?: string;
}

export interface TenantPaymentSettings {
  provider: 'mock' | 'tianque' | 'wechat_partner';
  merchantNo: string;
  subAppId: string;
  onboardingStatus: 'NOT_APPLIED' | 'REVIEWING' | 'PENDING_SIGNING' | 'ACTIVE' | 'REJECTED';
  productAuthorizationStatus: 'NOT_AUTHORIZED' | 'PENDING' | 'AUTHORIZED' | 'REVOKED';
  refundAuthorized: boolean;
  onboardingApplication?: {
    subjectType: 'MICRO' | 'INDIVIDUAL' | 'ENTERPRISE';
    businessScene: 'STORE' | 'MOBILE';
    merchantShortName: string;
    operatorName: string;
    contactPhone: string;
    applicationStatus: string;
    platformNote: string;
    submittedAt: string;
  };
}

export interface PendingOnboardingApplication {
  tenantId: number;
  tenantName: string;
  tenantCode: string;
  subjectType: string;
  merchantShortName: string;
  operatorName: string;
  contactPhone: string;
  applicationStatus: string;
  platformNote: string;
  submittedAt: string;
  updatedAt: string;
}

export interface TenantMiniAppSettings {
  primaryMode: 'PUBLIC' | 'DEDICATED';
  publicEnabled: boolean;
  publicAppId: string;
  publicConfigured: boolean;
  dedicatedEnabled: boolean;
  dedicatedDisplayName: string;
  dedicatedChannelKey: string;
  dedicatedAppId: string;
  dedicatedAppSecret?: string;
  appSecretConfigured: boolean;
}

export interface SystemSettings {
  platformName?: string;
  supportPhone?: string;
  supportEmail?: string;
  marketingTitle?: string;
  marketingSubtitle?: string;
  contactWechat?: string;
  contactQrUrl?: string;
  marketingPageUrl?: string;
  orderExpireMinutes?: number;
  loginFailureLimit?: number;
  sessionExpireMinutes?: number;
}

export interface PrinterProviderSettings {
  provider: string;
  displayName: string;
  enabled: boolean;
  developerId: string;
  secretSet: boolean;
  baseUrl: string;
  configured: boolean;
  autoRegister: boolean;
  synced?: number;
  syncFailed?: number;
}

export interface PrinterProviderTestResult {
  deviceName: string;
  status: 'ONLINE' | 'OFFLINE' | 'PAPER_OUT' | 'UNREACHABLE';
  message: string;
  checkedAt: string;
}

export type AnnouncementCategory = 'SYSTEM_UPDATE' | 'BUG_FIX' | 'NEW_FEATURE' | 'NOTICE' | 'ACTION_REQUIRED';
export type AnnouncementSeverity = 'INFO' | 'IMPORTANT' | 'URGENT';
export type AnnouncementStatus = 'DRAFT' | 'PUBLISHED' | 'WITHDRAWN';
export type AnnouncementAudience = 'ALL' | 'SELECTED';

export interface PlatformAnnouncement {
  id: string;
  title: string;
  summary: string;
  content: string;
  category: AnnouncementCategory;
  severity: AnnouncementSeverity;
  audienceType: AnnouncementAudience;
  status: AnnouncementStatus;
  tenantIds: string[];
  targetCount: number;
  readCount: number;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  publishedAt?: string;
  withdrawnAt?: string;
}

export interface AnnouncementValues {
  title: string;
  summary?: string;
  content: string;
  category: AnnouncementCategory;
  severity: AnnouncementSeverity;
  audienceType: AnnouncementAudience;
  tenantIds?: string[];
}

export interface WebsiteSettings {
  brandName: string;
  brandEnglishName: string;
  heroEyebrow: string;
  heroTitle: string;
  heroHighlight: string;
  heroSubtitle: string;
  heroImageUrl: string;
  scanOrderImageUrl: string;
  cashierImageUrl: string;
  kitchenImageUrl: string;
  sceneBreakfastImageUrl: string;
  sceneCoffeeTruckImageUrl: string;
  sceneBakeryImageUrl: string;
  sceneNightMarketImageUrl: string;
  sceneCafeImageUrl: string;
  supportPhone: string;
  supportEmail: string;
  contactWechat: string;
  contactQrUrl: string;
  companyName: string;
  companyAddress: string;
  icpNumber: string;
  footerText: string;
  merchantLoginUrl: string;
  metaTitle: string;
  metaDescription: string;
}

export type WebsiteArticleStatus = 'DRAFT' | 'PUBLISHED' | 'WITHDRAWN';

export interface WebsiteArticle {
  id: string;
  slug: string;
  title: string;
  summary: string;
  coverUrl: string;
  content: string;
  status: WebsiteArticleStatus;
  isFeatured: boolean;
  publishedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export type WebsiteArticleValues = Pick<WebsiteArticle, 'slug' | 'title' | 'summary' | 'coverUrl' | 'content' | 'isFeatured'>;

export interface WebsiteMedia {
  id: string;
  name: string;
  altText: string;
  url: string;
  storageKey: string;
  mimeType: string;
  width: number;
  height: number;
  sizeBytes: number;
  status: string;
  createdAt: string;
}
