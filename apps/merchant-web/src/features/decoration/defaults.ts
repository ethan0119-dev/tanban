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
    fontScale: 'STANDARD',
    surfaceStyle: 'ELEVATED',
    buttonShape: 'ROUNDED',
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
  navigationTemplate: 'classic',
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
  options: {
    navigationTemplate?: DecorationConfig['navigationTemplate'];
    homeOrder?: Array<DecorationConfig['homeModules'][number]['type']>;
  } = {},
): DecorationConfig {
  const config: DecorationConfig = {
    ...structuredClone(DEFAULT_DECORATION),
    templateKey: key,
    theme: { ...DEFAULT_DECORATION.theme, ...theme },
    ordering: { ...DEFAULT_DECORATION.ordering, ...ordering },
    navigationTemplate: options.navigationTemplate ?? DEFAULT_DECORATION.navigationTemplate,
  };
  if (options.homeOrder?.length) {
    const rank = new Map(options.homeOrder.map((type, index) => [type, index]));
    config.homeModules = config.homeModules
      .filter((module) => rank.has(module.type))
      .sort((left, right) => (rank.get(left.type) ?? 99) - (rank.get(right.type) ?? 99))
      .map((module, index) => ({ ...module, sortOrder: (index + 1) * 10 }));
  }
  config.navigation = config.navigation.map((item) => ({ ...item }));
  config.navigationTemplate = options.navigationTemplate ?? config.navigationTemplate;
  config.theme.navBackgroundColor = config.theme.navBackgroundColor || config.theme.surfaceColor;
  config.theme.navSelectedColor = config.theme.navSelectedColor || config.theme.primaryColor;
  return config;
}

export const BUILTIN_TEMPLATES: DecorationTemplate[] = [
  {
    id: 'coffee-night',
    key: 'coffee-night',
    name: '夜色咖啡',
    description: '深绿与荧光黄，适合咖啡车、夜市和创意饮品。',
    tone: 'linear-gradient(135deg, #23483c, #d6f36a)',
    scene: '咖啡 · 夜市',
    highlights: ['沉浸首页', '大图主视觉', '舒适点单'],
    config: withTheme('coffee-night', {}, undefined, { navigationTemplate: 'dark', homeOrder: ['HERO_BANNER', 'STORE_HEADER', 'ANNOUNCEMENT', 'QUICK_ACTIONS', 'TEXT'] }),
  },
  {
    id: 'warm-bakery',
    key: 'warm-bakery',
    name: '暖调烘焙',
    description: '奶油底色与焦糖棕，适合面包、甜点和轻食。',
    tone: 'linear-gradient(135deg, #9f6139, #f2c990)',
    scene: '烘焙 · 甜品',
    highlights: ['暖色品牌', '会员入口', '大圆角卡片'],
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
      fontScale: 'STANDARD',
      surfaceStyle: 'ELEVATED',
      buttonShape: 'PILL',
    }, undefined, { navigationTemplate: 'warm', homeOrder: ['HERO_BANNER', 'QUICK_ACTIONS', 'STORE_HEADER', 'ANNOUNCEMENT', 'TEXT'] }),
  },
  {
    id: 'fresh-tea',
    key: 'fresh-tea',
    name: '鲜果茶铺',
    description: '轻盈的青绿色调，突出水果茶和夏日饮品。',
    tone: 'linear-gradient(135deg, #1f8b76, #ffe36e)',
    scene: '茶饮 · 果饮',
    highlights: ['顶部分类', '宫格商品', '轻盈配色'],
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
      fontScale: 'STANDARD',
      surfaceStyle: 'BORDERED',
      buttonShape: 'PILL',
    }, { layout: 'CATEGORY_TOP', productLayout: 'GRID' }, { navigationTemplate: 'soft', homeOrder: ['STORE_HEADER', 'HERO_BANNER', 'QUICK_ACTIONS', 'ANNOUNCEMENT', 'TEXT'] }),
  },
  {
    id: 'clean-fastfood',
    key: 'clean-fastfood',
    name: '清爽快餐',
    description: '高对比红白配色，信息密度更高、点单更直接。',
    tone: 'linear-gradient(135deg, #df4039, #fff0e8)',
    scene: '快餐 · 小吃',
    highlights: ['紧凑信息', '列表点单', '直接加购'],
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
      fontScale: 'COMPACT',
      surfaceStyle: 'BORDERED',
      buttonShape: 'ROUNDED',
    }, { density: 'COMPACT', productLayout: 'LIST', productActionMode: 'DIRECT_ADD' }, { navigationTemplate: 'classic', homeOrder: ['STORE_HEADER', 'ANNOUNCEMENT', 'QUICK_ACTIONS', 'HERO_BANNER'] }),
  },
  {
    id: 'japanese-minimal',
    key: 'japanese-minimal',
    name: '日式原木',
    description: '克制的原木与米白色，适合寿司、定食和小型餐吧。',
    tone: 'linear-gradient(135deg, #6E5A46, #E7DCCB)',
    scene: '定食 · 餐吧',
    highlights: ['留白首页', '描边卡片', '标准字号'],
    config: withTheme('japanese-minimal', {
      primaryColor: '#6E5A46', accentColor: '#D9BE8B', backgroundColor: '#F5F1E8', surfaceColor: '#FFFCF6',
      textColor: '#2F2A25', mutedColor: '#777067', navBackgroundColor: '#FFFCF6', navTextColor: '#777067', navSelectedColor: '#6E5A46',
      radius: 'SM', fontScale: 'STANDARD', surfaceStyle: 'BORDERED', buttonShape: 'SQUARE',
    }, { layout: 'CATEGORY_TOP' }, { navigationTemplate: 'classic', homeOrder: ['STORE_HEADER', 'HERO_BANNER', 'ANNOUNCEMENT', 'TEXT', 'QUICK_ACTIONS'] }),
  },
  {
    id: 'sea-salt-blue',
    key: 'sea-salt-blue',
    name: '海盐蓝调',
    description: '清透海盐蓝与珊瑚橙，适合冰饮、轻食和夏季活动。',
    tone: 'linear-gradient(135deg, #286F8E, #FFA36C)',
    scene: '冰饮 · 轻食',
    highlights: ['清爽首页', '宫格商品', '高辨识按钮'],
    config: withTheme('sea-salt-blue', {
      primaryColor: '#286F8E', accentColor: '#FFA36C', backgroundColor: '#F2F8FA', surfaceColor: '#FFFFFF',
      textColor: '#173745', mutedColor: '#66818C', navBackgroundColor: '#FFFFFF', navTextColor: '#66818C', navSelectedColor: '#286F8E',
      radius: 'MD', fontScale: 'STANDARD', surfaceStyle: 'ELEVATED', buttonShape: 'PILL',
    }, { layout: 'CATEGORY_TOP', productLayout: 'GRID' }, { navigationTemplate: 'soft', homeOrder: ['HERO_BANNER', 'STORE_HEADER', 'QUICK_ACTIONS', 'ANNOUNCEMENT'] }),
  },
  {
    id: 'cream-dessert',
    key: 'cream-dessert',
    name: '奶油甜品',
    description: '柔和奶油与莓果粉，突出新品、会员和甜品陈列。',
    tone: 'linear-gradient(135deg, #A34D68, #F5D7DE)',
    scene: '蛋糕 · 甜品',
    highlights: ['柔和卡片', '大字号', '胶囊按钮'],
    config: withTheme('cream-dessert', {
      primaryColor: '#A34D68', accentColor: '#F3C36B', backgroundColor: '#FFF6F7', surfaceColor: '#FFFFFF',
      textColor: '#402B32', mutedColor: '#8B737B', navBackgroundColor: '#FFFFFF', navTextColor: '#8B737B', navSelectedColor: '#A34D68',
      radius: 'LG', fontScale: 'LARGE', surfaceStyle: 'ELEVATED', buttonShape: 'PILL',
    }, { productLayout: 'GRID' }, { navigationTemplate: 'soft', homeOrder: ['HERO_BANNER', 'QUICK_ACTIONS', 'TEXT', 'STORE_HEADER', 'ANNOUNCEMENT'] }),
  },
  {
    id: 'night-market-neon',
    key: 'night-market-neon',
    name: '夜市霓虹',
    description: '深色背景与金黄强调，适合烧烤、夜宵和移动摊位。',
    tone: 'linear-gradient(135deg, #171B24, #F4C95D)',
    scene: '烧烤 · 夜宵',
    highlights: ['深色沉浸', '紧凑点单', '高对比导航'],
    config: withTheme('night-market-neon', {
      primaryColor: '#E8A83E', accentColor: '#F4D35E', backgroundColor: '#11151D', surfaceColor: '#1D2430',
      textColor: '#F7F2E7', mutedColor: '#AAB2BE', navBackgroundColor: '#171D27', navTextColor: '#AAB2BE', navSelectedColor: '#F4D35E',
      radius: 'MD', fontScale: 'STANDARD', surfaceStyle: 'ELEVATED', buttonShape: 'ROUNDED',
    }, { density: 'COMPACT' }, { navigationTemplate: 'dark', homeOrder: ['HERO_BANNER', 'ANNOUNCEMENT', 'QUICK_ACTIONS', 'STORE_HEADER'] }),
  },
  {
    id: 'luxury-gold',
    key: 'luxury-gold',
    name: '轻奢黑金',
    description: '炭黑、象牙白与香槟金，适合精品咖啡和高客单门店。',
    tone: 'linear-gradient(135deg, #22201E, #C8A96B)',
    scene: '精品 · 高端',
    highlights: ['品牌故事', '大图主视觉', '克制动效'],
    config: withTheme('luxury-gold', {
      primaryColor: '#2D2925', accentColor: '#C8A96B', backgroundColor: '#F2EFE9', surfaceColor: '#FFFCF6',
      textColor: '#26221F', mutedColor: '#746B63', navBackgroundColor: '#2D2925', navTextColor: '#B8ADA1', navSelectedColor: '#E2C687',
      radius: 'SM', fontScale: 'LARGE', surfaceStyle: 'BORDERED', buttonShape: 'SQUARE',
    }, undefined, { navigationTemplate: 'dark', homeOrder: ['HERO_BANNER', 'TEXT', 'STORE_HEADER', 'ANNOUNCEMENT', 'QUICK_ACTIONS'] }),
  },
  {
    id: 'brand-neutral',
    key: 'brand-neutral',
    name: '品牌通用',
    description: '中性蓝灰与清晰层级，适合尚未确定品牌风格的新门店。',
    tone: 'linear-gradient(135deg, #3F5F73, #D7E5EC)',
    scene: '通用 · 新店',
    highlights: ['易读层级', '标准布局', '便于改色'],
    config: withTheme('brand-neutral', {
      primaryColor: '#3F5F73', accentColor: '#E8B85F', backgroundColor: '#F4F6F7', surfaceColor: '#FFFFFF',
      textColor: '#233039', mutedColor: '#6E7D86', navBackgroundColor: '#FFFFFF', navTextColor: '#6E7D86', navSelectedColor: '#3F5F73',
      radius: 'MD', fontScale: 'STANDARD', surfaceStyle: 'BORDERED', buttonShape: 'ROUNDED',
    }, undefined, { navigationTemplate: 'classic', homeOrder: ['STORE_HEADER', 'ANNOUNCEMENT', 'HERO_BANNER', 'QUICK_ACTIONS', 'TEXT'] }),
  },
];

export const SAMPLE_ASSETS: MediaAsset[] = [
  { id: 'sample-banner', name: '示例 Banner', url: '', type: 'IMAGE' },
  { id: 'sample-logo', name: '门店 Logo 示例', url: '', type: 'IMAGE' },
];

export function cloneDecoration(config: DecorationConfig): DecorationConfig {
  return structuredClone(config);
}
