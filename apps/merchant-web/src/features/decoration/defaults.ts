import type { DecorationConfig, DecorationTemplate, MediaAsset } from './model';

export const DEFAULT_DECORATION: DecorationConfig = {
  schemaVersion: 1,
  templateKey: 'coffee-night',
  storeName: '码农咖啡',
  theme: {
    primaryColor: '#345548',
    accentColor: '#D6F36A',
    backgroundColor: '#F7F0DF',
    surfaceColor: '#FFFEFA',
    textColor: '#202922',
    mutedColor: '#747B75',
    navBackgroundColor: '#FFFEFA',
    navTextColor: '#7B807A',
    navSelectedColor: '#345548',
    radius: 'LG',
  },
  homeModules: [
    { id: 'hero', type: 'HERO_BANNER', enabled: true, sortOrder: 10, title: '今晚，来一杯认真做的咖啡', subtitle: 'COFFEE AFTER DARK', imageUrl: '' },
    { id: 'store', type: 'STORE_HEADER', enabled: true, sortOrder: 20, title: '营业中', subtitle: '欢迎扫码点单' },
    { id: 'notice', type: 'ANNOUNCEMENT', enabled: true, sortOrder: 30, title: '公告', subtitle: '下单后请留意取餐码' },
    { id: 'quick-actions', type: 'QUICK_ACTIONS', enabled: true, sortOrder: 40, title: '堂食 / 自提点单', subtitle: '选好口味，在线下单' },
    { id: 'brand-story', type: 'TEXT', enabled: true, sortOrder: 50, title: '认真经营每一个小摊', subtitle: '每一杯都值得认真对待' },
  ],
  ordering: {
    layout: 'CATEGORY_LEFT',
    productLayout: 'LIST',
    density: 'COMFORTABLE',
    showDescription: true,
    showSoldOut: true,
    showStock: false,
    showSales: false,
    loadMode: 'BY_CATEGORY',
    productActionMode: 'SKU_SHEET',
  },
  navigation: [
    { id: 'home', key: 'HOME', label: '首页', enabled: true },
    { id: 'menu', key: 'MENU', label: '点单', enabled: true },
    { id: 'orders', key: 'ORDERS', label: '订单', enabled: true },
    { id: 'profile', key: 'PROFILE', label: '我的', enabled: true },
  ],
  splash: {
    enabled: false,
    imageUrl: '',
    title: '欢迎来到码农咖啡',
    subtitle: '认真做咖啡，也认真生活',
    displayMode: 'POPUP',
    autoCloseSeconds: 5,
    frequency: 'ONCE_PER_VERSION',
  },
};

function withTheme(
  key: string,
  theme: Partial<DecorationConfig['theme']>,
  ordering?: Partial<DecorationConfig['ordering']>,
): DecorationConfig {
  return {
    ...structuredClone(DEFAULT_DECORATION),
    templateKey: key,
    theme: { ...DEFAULT_DECORATION.theme, ...theme },
    ordering: { ...DEFAULT_DECORATION.ordering, ...ordering },
  };
}

export const BUILTIN_TEMPLATES: DecorationTemplate[] = [
  {
    id: 'coffee-night',
    key: 'coffee-night',
    name: '夜色咖啡',
    description: '深绿与荧光黄，适合咖啡车、夜市和创意饮品。',
    tone: 'linear-gradient(135deg, #23483c, #d6f36a)',
    config: withTheme('coffee-night', {}),
  },
  {
    id: 'warm-bakery',
    key: 'warm-bakery',
    name: '暖调烘焙',
    description: '奶油底色与焦糖棕，适合面包、甜点和轻食。',
    tone: 'linear-gradient(135deg, #9f6139, #f2c990)',
    config: withTheme('warm-bakery', {
      primaryColor: '#9A5F3D',
      accentColor: '#F3C780',
      backgroundColor: '#FFF7EA',
      textColor: '#3A2A22',
      surfaceColor: '#FFFFFF',
      mutedColor: '#806F65',
      navBackgroundColor: '#FFFFFF',
      navTextColor: '#806F65',
      navSelectedColor: '#9A5F3D',
      radius: 'LG',
    }),
  },
  {
    id: 'fresh-tea',
    key: 'fresh-tea',
    name: '鲜果茶铺',
    description: '轻盈的青绿色调，突出水果茶和夏日饮品。',
    tone: 'linear-gradient(135deg, #1f8b76, #ffe36e)',
    config: withTheme('fresh-tea', {
      primaryColor: '#17806B',
      accentColor: '#FFE36D',
      backgroundColor: '#F0FAF3',
      textColor: '#173B32',
      surfaceColor: '#FFFFFF',
      mutedColor: '#66857D',
      navBackgroundColor: '#FFFFFF',
      navTextColor: '#66857D',
      navSelectedColor: '#17806B',
      radius: 'MD',
    }, { layout: 'CATEGORY_TOP', productLayout: 'GRID' }),
  },
  {
    id: 'clean-fastfood',
    key: 'clean-fastfood',
    name: '清爽快餐',
    description: '高对比红白配色，信息密度更高、点单更直接。',
    tone: 'linear-gradient(135deg, #df4039, #fff0e8)',
    config: withTheme('clean-fastfood', {
      primaryColor: '#D83C35',
      accentColor: '#FFD15C',
      backgroundColor: '#F7F7F5',
      textColor: '#2E2826',
      surfaceColor: '#FFFFFF',
      mutedColor: '#746967',
      navBackgroundColor: '#FFFFFF',
      navTextColor: '#746967',
      navSelectedColor: '#D83C35',
      radius: 'SM',
    }, { density: 'COMPACT', productLayout: 'LIST' }),
  },
];

export const SAMPLE_ASSETS: MediaAsset[] = [
  { id: 'sample-banner', name: '示例 Banner', url: '', type: 'IMAGE' },
  { id: 'sample-logo', name: '门店 Logo 占位', url: '', type: 'IMAGE' },
];

export function cloneDecoration(config: DecorationConfig): DecorationConfig {
  return structuredClone(config);
}
