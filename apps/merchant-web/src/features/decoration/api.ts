import { api } from '../../api/client';
import { BUILTIN_TEMPLATES, cloneDecoration, DEFAULT_DECORATION } from './defaults';
import type {
  DecorationConfig,
  DecorationDraft,
  DecorationTemplate,
  DecorationVersion,
  DecorationWorkspace,
  HomeModuleConfig,
  HomeModuleType,
  MediaAsset,
  PublishedDecoration,
} from './model';

type UnknownRecord = Record<string, unknown>;

interface ApiDecorationModule {
  id: string;
  type: HomeModuleType;
  enabled: boolean;
  sortOrder: number;
  config: UnknownRecord;
}

interface ApiDecorationConfig {
  schemaVersion: 1;
  templateKey: string;
  theme: {
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
  };
  home: { modules: ApiDecorationModule[] };
  menu: {
    categoryLayout: 'LEFT' | 'TOP';
    productLayout: 'LIST' | 'GRID';
    showDescription: boolean;
    showStock: boolean;
    showSales: boolean;
    showSoldOut: boolean;
    loadMode: 'BY_CATEGORY' | 'ALL';
    productActionMode: 'SKU_SHEET' | 'DIRECT_ADD';
    density: 'COMFORTABLE' | 'COMPACT';
  };
  navigation: {
    items: Array<{ key: string; text: string; visible: boolean; sortOrder: number }>;
    backgroundColor: string;
    textColor: string;
    selectedColor: string;
  };
  splash: {
    enabled: boolean;
    displayMode: 'FULLSCREEN' | 'POPUP';
    imageUrl: string;
    title: string;
    subtitle: string;
    autoCloseSeconds: number;
    action: { type: 'NONE' };
    frequency: 'EVERY_VISIT' | 'DAILY' | 'ONCE_PER_VERSION';
  };
}

function record(value: unknown): UnknownRecord {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as UnknownRecord : {};
}

function stringValue(...values: unknown[]): string {
  const value = values.find((item) => typeof item === 'string');
  return typeof value === 'string' ? value : '';
}

function numberValue(fallback: number, ...values: unknown[]): number {
  const value = values.find((item) => item !== undefined && item !== null && Number.isFinite(Number(item)));
  return value === undefined ? fallback : Number(value);
}

function booleanValue(fallback: boolean, ...values: unknown[]): boolean {
  const value = values.find((item) => typeof item === 'boolean');
  return typeof value === 'boolean' ? value : fallback;
}

function arrayPayload(payload: unknown): unknown[] {
  if (Array.isArray(payload)) return payload;
  const value = record(payload);
  const items = value.items ?? value.list ?? value.records ?? value.versions ?? value.templates ?? value.assets;
  return Array.isArray(items) ? items : [];
}

function moduleView(payload: unknown, index: number): HomeModuleConfig {
  const value = record(payload);
  const moduleConfig = record(value.config);
  const items = Array.isArray(moduleConfig.items) ? moduleConfig.items.map(record) : [];
  const first = items[0] ?? {};
  const type = stringValue(value.type) as HomeModuleType;
  const fallback: HomeModuleConfig = {
    id: stringValue(value.id) || `module-${index}`,
    type: type || 'TEXT',
    enabled: booleanValue(true, value.enabled),
    sortOrder: numberValue((index + 1) * 10, value.sortOrder, value.sort_order),
    title: '',
    subtitle: '',
    imageUrl: '',
  };
  if (type === 'HERO_BANNER') {
    return { ...fallback, title: stringValue(first.title), subtitle: stringValue(first.subtitle), imageUrl: stringValue(first.imageUrl, first.image_url) };
  }
  if (type === 'STORE_HEADER') return { ...fallback, title: '营业中', subtitle: '展示门店 Logo、状态和地址' };
  if (type === 'ANNOUNCEMENT') return { ...fallback, title: stringValue(moduleConfig.prefix) || '公告', subtitle: '门店公告正文来自门店设置' };
  if (type === 'QUICK_ACTIONS') return { ...fallback, title: stringValue(first.title) || '堂食 / 自提点单', subtitle: stringValue(first.subtitle) || '选好口味，在线下单' };
  if (type === 'IMAGE') return { ...fallback, title: stringValue(moduleConfig.alt), imageUrl: stringValue(moduleConfig.imageUrl, moduleConfig.image_url) };
  if (type === 'TEXT') return { ...fallback, title: stringValue(moduleConfig.title), subtitle: stringValue(moduleConfig.body) };
  if (type === 'SPACER') return { ...fallback, title: '留白', subtitle: `${numberValue(24, moduleConfig.height)}px` };
  return fallback;
}

export function normalizeConfig(payload: unknown): DecorationConfig {
  const value = record(payload);
  const base = cloneDecoration(DEFAULT_DECORATION);
  const rawTheme = record(value.theme);
  const rawHome = record(value.home);
  const rawMenu = record(value.menu);
  const rawNavigation = record(value.navigation);
  const rawSplash = record(value.splash);
  const modules = Array.isArray(rawHome.modules) ? rawHome.modules : [];
  const navItems = Array.isArray(rawNavigation.items) ? rawNavigation.items : [];
  return {
    ...base,
    schemaVersion: 1,
    templateKey: stringValue(value.templateKey, value.template_key) || base.templateKey,
    theme: {
      primaryColor: stringValue(rawTheme.primaryColor) || base.theme.primaryColor,
      accentColor: stringValue(rawTheme.accentColor) || base.theme.accentColor,
      backgroundColor: stringValue(rawTheme.backgroundColor) || base.theme.backgroundColor,
      surfaceColor: stringValue(rawTheme.surfaceColor) || base.theme.surfaceColor,
      textColor: stringValue(rawTheme.textColor) || base.theme.textColor,
      mutedColor: stringValue(rawTheme.mutedColor) || base.theme.mutedColor,
      navBackgroundColor: stringValue(rawTheme.navBackgroundColor) || base.theme.navBackgroundColor,
      navTextColor: stringValue(rawTheme.navTextColor) || base.theme.navTextColor,
      navSelectedColor: stringValue(rawTheme.navSelectedColor) || base.theme.navSelectedColor,
      radius: (stringValue(rawTheme.radius) || base.theme.radius) as DecorationConfig['theme']['radius'],
    },
    homeModules: modules.length ? modules.map(moduleView).sort((a, b) => a.sortOrder - b.sortOrder) : base.homeModules,
    ordering: {
      layout: stringValue(rawMenu.categoryLayout) === 'TOP' ? 'CATEGORY_TOP' : 'CATEGORY_LEFT',
      productLayout: stringValue(rawMenu.productLayout) === 'GRID' ? 'GRID' : 'LIST',
      density: stringValue(rawMenu.density) === 'COMPACT' ? 'COMPACT' : 'COMFORTABLE',
      showDescription: booleanValue(base.ordering.showDescription, rawMenu.showDescription),
      showSoldOut: booleanValue(base.ordering.showSoldOut, rawMenu.showSoldOut),
      showStock: booleanValue(base.ordering.showStock, rawMenu.showStock),
      showSales: booleanValue(base.ordering.showSales, rawMenu.showSales),
      loadMode: stringValue(rawMenu.loadMode) === 'ALL' ? 'ALL' : 'BY_CATEGORY',
      productActionMode: stringValue(rawMenu.productActionMode) === 'DIRECT_ADD' ? 'DIRECT_ADD' : 'SKU_SHEET',
    },
    navigation: navItems.length ? navItems.map((item, index) => {
      const nav = record(item);
      const key = stringValue(nav.key).toUpperCase() as DecorationConfig['navigation'][number]['key'];
      return {
        id: stringValue(nav.key) || `navigation-${index}`,
        key,
        label: stringValue(nav.text) || base.navigation[index % base.navigation.length].label,
        enabled: booleanValue(true, nav.visible),
      };
    }) : base.navigation,
    splash: {
      enabled: booleanValue(false, rawSplash.enabled),
      imageUrl: stringValue(rawSplash.imageUrl),
      title: stringValue(rawSplash.title),
      subtitle: stringValue(rawSplash.subtitle),
      displayMode: stringValue(rawSplash.displayMode) === 'FULLSCREEN' ? 'FULLSCREEN' : 'POPUP',
      autoCloseSeconds: numberValue(5, rawSplash.autoCloseSeconds),
      frequency: (stringValue(rawSplash.frequency) || 'ONCE_PER_VERSION') as DecorationConfig['splash']['frequency'],
    },
  };
}

function modulePayload(module: HomeModuleConfig, index: number): ApiDecorationModule {
  let config: UnknownRecord;
  switch (module.type) {
    case 'HERO_BANNER':
      config = {
        items: module.imageUrl ? [{ imageUrl: module.imageUrl.trim(), title: module.title, subtitle: module.subtitle, action: { type: 'OPEN_MENU' } }] : [],
      };
      break;
    case 'STORE_HEADER':
      config = { showLogo: true, showStatus: true, showAddress: true };
      break;
    case 'ANNOUNCEMENT':
      config = { prefix: module.title.slice(0, 16) };
      break;
    case 'QUICK_ACTIONS':
      config = { items: [
        { title: module.title, subtitle: module.subtitle, action: { type: 'OPEN_MENU' } },
        { title: '查看我的订单', subtitle: '支付与制作进度', action: { type: 'OPEN_ORDERS' } },
      ] };
      break;
    case 'IMAGE':
      config = { imageUrl: module.imageUrl ?? '', alt: module.title, action: { type: 'NONE' } };
      break;
    case 'SPACER':
      config = { height: Math.min(160, Math.max(4, Number.parseInt(module.subtitle, 10) || 24)) };
      break;
    case 'TEXT':
    default:
      config = { title: module.title, body: module.subtitle, align: 'LEFT' };
      break;
  }
  return { id: module.id, type: module.type, enabled: module.enabled, sortOrder: (index + 1) * 10, config };
}

export function toApiConfig(config: DecorationConfig): ApiDecorationConfig {
  return {
    schemaVersion: 1,
    templateKey: config.templateKey,
    theme: { ...config.theme },
    home: { modules: config.homeModules.map(modulePayload) },
    menu: {
      categoryLayout: config.ordering.layout === 'CATEGORY_TOP' ? 'TOP' : 'LEFT',
      productLayout: config.ordering.productLayout,
      showDescription: config.ordering.showDescription,
      showStock: config.ordering.showStock,
      showSales: config.ordering.showSales,
      showSoldOut: config.ordering.showSoldOut,
      loadMode: config.ordering.loadMode,
      productActionMode: config.ordering.productActionMode,
      density: config.ordering.density,
    },
    navigation: {
      items: config.navigation.map((item, index) => ({ key: item.key.toLowerCase(), text: item.label, visible: item.enabled, sortOrder: (index + 1) * 10 })),
      backgroundColor: config.theme.navBackgroundColor,
      textColor: config.theme.navTextColor,
      selectedColor: config.theme.navSelectedColor,
    },
    splash: {
      enabled: config.splash.enabled,
      displayMode: config.splash.displayMode,
      imageUrl: config.splash.imageUrl,
      title: config.splash.title,
      subtitle: config.splash.subtitle,
      autoCloseSeconds: config.splash.autoCloseSeconds,
      action: { type: 'NONE' },
      frequency: config.splash.frequency,
    },
  };
}

function normalizeDraft(payload: unknown): DecorationDraft {
  const value = record(payload);
  return {
    revision: numberValue(0, value.revision),
    config: normalizeConfig(value.config),
    updatedAt: stringValue(value.updatedAt, value.updated_at),
  };
}

function normalizePublished(payload: unknown): PublishedDecoration | null {
  if (!payload) return null;
  const value = record(payload);
  if (!Object.keys(value).length) return null;
  return {
    id: (value.id as string | number | undefined) ?? '',
    versionNo: numberValue(0, value.versionNo, value.version_no),
    config: normalizeConfig(value.config),
    note: stringValue(value.note),
    publishedAt: stringValue(value.publishedAt, value.published_at),
  };
}

export function normalizeWorkspace(payload: unknown): DecorationWorkspace {
  const value = record(payload);
  const draft = normalizeDraft(value.draft);
  const storeName = stringValue(value.storeName, value.store_name);
  if (storeName) draft.config.storeName = storeName;
  const published = normalizePublished(value.published);
  if (published && storeName) published.config.storeName = storeName;
  return { draft, published };
}

function normalizeVersion(payload: unknown): DecorationVersion {
  const value = record(payload);
  return {
    id: (value.id as string | number | undefined) ?? '',
    versionNo: numberValue(0, value.versionNo, value.version_no),
    note: stringValue(value.note),
    publishedAt: stringValue(value.publishedAt, value.published_at),
    publishedBy: stringValue(value.publishedBy, value.published_by),
    config: value.config ? normalizeConfig(value.config) : undefined,
  };
}

function normalizeTemplate(payload: unknown, index: number): DecorationTemplate {
  const value = record(payload);
  const config = normalizeConfig(value.config);
  const fallback = BUILTIN_TEMPLATES.find((item) => item.key === stringValue(value.key)) ?? BUILTIN_TEMPLATES[index % BUILTIN_TEMPLATES.length];
  return {
    id: (value.id as string | number | undefined) ?? (stringValue(value.key) || fallback.id),
    key: stringValue(value.key, value.code) || fallback.key,
    name: stringValue(value.name) || fallback.name,
    description: stringValue(value.description) || fallback.description,
    tone: stringValue(value.tone, value.previewColor, value.preview_color) || `linear-gradient(135deg, ${config.theme.primaryColor}, ${config.theme.accentColor})`,
    config,
  };
}

function normalizeAsset(payload: unknown): MediaAsset {
  const value = record(payload);
  return {
    id: (value.id as string | number | undefined) ?? '',
    name: stringValue(value.name) || '未命名素材',
    url: stringValue(value.url),
    type: stringValue(value.kind, value.type).toUpperCase() === 'VIDEO' ? 'VIDEO' : 'IMAGE',
    createdAt: stringValue(value.createdAt, value.created_at),
  };
}

function assetPayload(input: Pick<MediaAsset, 'name' | 'url' | 'type'>) {
  return { name: input.name, url: input.url, storageKey: '', mimeType: '', width: 0, height: 0, sizeBytes: 0 };
}

export const decorationApi = {
  async loadWorkspace(): Promise<DecorationWorkspace> {
    return normalizeWorkspace(await api.get('/merchant/decoration'));
  },
  async saveDraft(expectedRevision: number, config: DecorationConfig): Promise<DecorationDraft> {
    return normalizeDraft(await api.put('/merchant/decoration/draft', { expectedRevision, config: toApiConfig(config) }));
  },
  async publish(expectedRevision: number, note: string): Promise<PublishedDecoration> {
    return normalizePublished(await api.post('/merchant/decoration/publish', { expectedRevision, note })) as PublishedDecoration;
  },
  async listTemplates(): Promise<DecorationTemplate[]> {
    return arrayPayload(await api.get('/merchant/decoration/templates')).map(normalizeTemplate);
  },
  async listVersions(): Promise<DecorationVersion[]> {
    return arrayPayload(await api.get('/merchant/decoration/versions')).map(normalizeVersion);
  },
  async getVersion(id: string | number): Promise<DecorationVersion> {
    return normalizeVersion(await api.get(`/merchant/decoration/versions/${encodeURIComponent(String(id))}`));
  },
  async rollback(id: string | number, expectedRevision: number, note: string): Promise<PublishedDecoration> {
    return normalizePublished(await api.post(`/merchant/decoration/versions/${encodeURIComponent(String(id))}/rollback`, { expectedRevision, note })) as PublishedDecoration;
  },
  async listAssets(): Promise<MediaAsset[]> {
    return arrayPayload(await api.get('/merchant/media-assets')).map(normalizeAsset);
  },
  async createAsset(input: Pick<MediaAsset, 'name' | 'url' | 'type'>): Promise<MediaAsset> {
    return normalizeAsset(await api.post('/merchant/media-assets', assetPayload(input)));
  },
  async updateAsset(id: string | number, input: Pick<MediaAsset, 'name' | 'url' | 'type'>): Promise<MediaAsset> {
    return normalizeAsset(await api.put(`/merchant/media-assets/${encodeURIComponent(String(id))}`, assetPayload(input)));
  },
  deleteAsset(id: string | number): Promise<unknown> {
    return api.delete(`/merchant/media-assets/${encodeURIComponent(String(id))}`);
  },
};
