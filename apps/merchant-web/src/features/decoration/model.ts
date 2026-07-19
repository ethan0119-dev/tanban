export type DecorationStatus = 'DRAFT' | 'PUBLISHED';

export type HomeModuleType = 'HERO_BANNER' | 'STORE_HEADER' | 'ANNOUNCEMENT' | 'QUICK_ACTIONS' | 'IMAGE' | 'TEXT' | 'SPACER';

export interface HomeModuleConfig {
  id: string;
  type: HomeModuleType;
  enabled: boolean;
  sortOrder: number;
  title: string;
  subtitle: string;
  imageUrl?: string;
}

export interface ThemeConfig {
  primaryColor: string;
  accentColor: string;
  backgroundColor: string;
  surfaceColor: string;
  textColor: string;
  mutedColor: string;
  navBackgroundColor: string;
  navTextColor: string;
  navSelectedColor: string;
  radius: 'SM' | 'MD' | 'LG';
}

export interface OrderingConfig {
  layout: 'CATEGORY_LEFT' | 'CATEGORY_TOP';
  productLayout: 'LIST' | 'GRID';
  density: 'COMFORTABLE' | 'COMPACT';
  showDescription: boolean;
  showSoldOut: boolean;
  showStock: boolean;
  showSales: boolean;
  loadMode: 'BY_CATEGORY' | 'ALL';
  productActionMode: 'SKU_SHEET' | 'DIRECT_ADD';
}

export type NavigationKey = 'HOME' | 'MENU' | 'ORDERS' | 'PROFILE';

export interface NavigationItemConfig {
  id: string;
  key: NavigationKey;
  label: string;
  enabled: boolean;
}

export interface SplashConfig {
  enabled: boolean;
  imageUrl: string;
  title: string;
  subtitle: string;
  displayMode: 'FULLSCREEN' | 'POPUP';
  autoCloseSeconds: number;
  frequency: 'EVERY_VISIT' | 'DAILY' | 'ONCE_PER_VERSION';
}

export interface DecorationConfig {
  schemaVersion: 1;
  templateKey: string;
  storeName: string;
  homeModules: HomeModuleConfig[];
  theme: ThemeConfig;
  ordering: OrderingConfig;
  navigation: NavigationItemConfig[];
  splash: SplashConfig;
}

export interface DecorationDraft {
  revision: number;
  config: DecorationConfig;
  updatedAt?: string;
}

export interface PublishedDecoration {
  id: string | number;
  versionNo: number;
  config: DecorationConfig;
  note?: string;
  publishedAt?: string;
}

export interface DecorationWorkspace {
  draft: DecorationDraft;
  published: PublishedDecoration | null;
}

export interface DecorationVersion {
  id: string | number;
  versionNo: number;
  note?: string;
  publishedAt?: string;
  publishedBy?: string;
  config?: DecorationConfig;
}

export interface DecorationTemplate {
  id: string | number;
  key: string;
  name: string;
  description: string;
  tone: string;
  config: DecorationConfig;
}

export interface MediaAsset {
  id: string | number;
  name: string;
  url: string;
  type: 'IMAGE' | 'VIDEO';
  createdAt?: string;
}

export type PreviewPage = 'HOME' | 'MENU';

export const HOME_MODULE_LABELS: Record<HomeModuleType, string> = {
  HERO_BANNER: '顶部轮播图',
  STORE_HEADER: '门店信息',
  ANNOUNCEMENT: '门店公告',
  QUICK_ACTIONS: '快捷入口',
  IMAGE: '单图模块',
  TEXT: '图文介绍',
  SPACER: '留白间距',
};

export const NAVIGATION_LABELS: Record<NavigationKey, string> = {
  HOME: '首页',
  MENU: '点单',
  ORDERS: '订单',
  PROFILE: '我的',
};
