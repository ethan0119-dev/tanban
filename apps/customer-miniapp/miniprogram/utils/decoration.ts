import type {
  DecorationAction,
  DecorationConfig,
  DecorationModule,
  DecorationModuleConfig,
  DecorationModuleType,
  DecorationNavigationItem,
  Store,
} from "../types/domain";
import type { TanbanAppOption } from "../app";
import { clearFastFoodContext } from "./fast-food-context";
import { clearTableOrderingContext, tableContextForStore } from "./table-context";

const COLOR = /^#[0-9a-fA-F]{6}$/;
const MODULE_TYPES: DecorationModuleType[] = [
  "HERO_BANNER",
  "STORE_HEADER",
  "ANNOUNCEMENT",
  "QUICK_ACTIONS",
  "IMAGE",
  "HOTSPOT_IMAGE",
  "TEXT",
  "SPACER",
];
const NAV_KEYS: DecorationNavigationItem["key"][] = ["home", "menu", "orders", "profile"];
const ACTIONS: DecorationAction["type"][] = ["NONE", "OPEN_MENU", "OPEN_DINE_IN", "OPEN_TAKEOUT", "OPEN_DELIVERY", "OPEN_ORDERS", "OPEN_PROFILE", "OPEN_RECHARGE", "OPEN_MY_COUPONS", "OPEN_COUPON_CENTER", "CALL_PHONE"];

function color(value: unknown, fallback: string): string {
  return typeof value === "string" && COLOR.test(value) ? value : fallback;
}

function bool(value: unknown, fallback: boolean): boolean {
  return typeof value === "boolean" ? value : fallback;
}

function text(value: unknown, fallback = "", max = 160): string {
  return typeof value === "string" ? value.slice(0, max) : fallback;
}

function numberIn(value: unknown, fallback: number, min: number, max: number): number {
  return typeof value === "number" && Number.isFinite(value) ? Math.min(max, Math.max(min, value)) : fallback;
}

function safeAction(value: unknown): DecorationAction {
  if (!value || typeof value !== "object") return { type: "NONE" };
  const raw = value as Partial<DecorationAction>;
  const type = ACTIONS.includes(raw.type as DecorationAction["type"]) ? raw.type as DecorationAction["type"] : "NONE";
  return type === "CALL_PHONE" ? { type, phone: text(raw.phone, "", 20) } : { type };
}

function legacyBanner(store?: Store): string {
  const candidate = store?.theme?.bannerUrl;
  return typeof candidate === "string" && candidate.startsWith("https://") ? candidate : "";
}

export function defaultDecoration(store?: Store): DecorationConfig {
  const banner = legacyBanner(store);
  return {
    schemaVersion: 1,
    templateKey: "coffee-light",
    theme: {
      primaryColor: "#214d3f",
      accentColor: "#dff06d",
      backgroundColor: "#f6f5f0",
      surfaceColor: "#fffefa",
      textColor: "#17201b",
      mutedColor: "#747b75",
      navBackgroundColor: "#fffefa",
      navTextColor: "#7b807a",
      navSelectedColor: "#214d3f",
      radius: "LG",
    },
    home: {
      modules: [
        { id: "hero", type: "HERO_BANNER", enabled: true, sortOrder: 10, config: { items: banner ? [{ imageUrl: banner, action: { type: "OPEN_MENU" } }] : [] } },
        { id: "store", type: "STORE_HEADER", enabled: true, sortOrder: 20, config: { showLogo: true, showStatus: true, showAddress: true } },
        { id: "notice", type: "ANNOUNCEMENT", enabled: true, sortOrder: 30, config: { prefix: "公告" } },
        {
          id: "quick-actions", type: "QUICK_ACTIONS", enabled: true, sortOrder: 40,
          config: { items: [
            { title: "堂食 / 自提点单", subtitle: "选好口味，在线下单", action: { type: "OPEN_MENU" } },
            { title: "查看我的订单", subtitle: "支付与制作进度", action: { type: "OPEN_ORDERS" } },
          ] },
        },
      ],
    },
    menu: {
      categoryLayout: "LEFT",
      productLayout: "LIST",
      showDescription: true,
      showStock: false,
      showSales: false,
      showSoldOut: true,
      loadMode: "BY_CATEGORY",
      productActionMode: "SKU_SHEET",
      density: "COMFORTABLE",
    },
    navigation: {
      templateKey: "classic",
      backgroundColor: "#fffefa",
      textColor: "#7b807a",
      selectedColor: "#214d3f",
      items: [
        { key: "home", text: "首页", visible: true, sortOrder: 10 },
        { key: "menu", text: "点单", visible: true, sortOrder: 20 },
        { key: "orders", text: "订单", visible: true, sortOrder: 30 },
        { key: "profile", text: "我的", visible: true, sortOrder: 40 },
      ],
    },
    splash: {
      enabled: false,
      displayMode: "POPUP",
      imageUrl: "",
      title: "",
      subtitle: "",
      autoCloseSeconds: 5,
      action: { type: "NONE" },
      frequency: "ONCE_PER_VERSION",
    },
  };
}

function safeModules(value: unknown, fallback: DecorationModule[]): DecorationModule[] {
  if (!Array.isArray(value)) return fallback;
  const ids = new Set<string>();
  return value.slice(0, 30).flatMap((entry, index) => {
    if (!entry || typeof entry !== "object") return [];
    const raw = entry as Partial<DecorationModule>;
    const type = MODULE_TYPES.includes(raw.type as DecorationModuleType) ? raw.type as DecorationModuleType : undefined;
    const id = text(raw.id, `module-${index}`, 64).replace(/[^a-zA-Z0-9_-]/g, "-");
    if (!type || !id || ids.has(id)) return [];
    ids.add(id);
    const source = raw.config && typeof raw.config === "object" ? raw.config : {};
    const align: DecorationModuleConfig["align"] = source.align === "CENTER" || source.align === "RIGHT" ? source.align : "LEFT";
    const textAlign: DecorationModuleConfig["textAlign"] = align === "CENTER" ? "center" : align === "RIGHT" ? "right" : "left";
    const items = Array.isArray(source.items) ? source.items.slice(0, type === "HERO_BANNER" ? 8 : 4).map((item) => {
      const row = item && typeof item === "object" ? item : {};
      return {
        imageUrl: text(row.imageUrl, "", 1024),
        title: text(row.title, "", 60),
        subtitle: text(row.subtitle, "", 160),
        action: safeAction(row.action),
      };
    }) : [];
    const hotspots = Array.isArray(source.hotspots) ? source.hotspots.slice(0, 20).map((item, hotspotIndex) => {
      const row = item && typeof item === "object" ? item as unknown as Record<string, unknown> : {};
      const x = numberIn(row.x, 0, 0, 99);
      const y = numberIn(row.y, 0, 0, 99);
      return {
        id: text(row.id, `hotspot-${hotspotIndex + 1}`, 64).replace(/[^a-zA-Z0-9_-]/g, "-"),
        x,
        y,
        width: numberIn(row.width, 20, 1, 100 - x),
        height: numberIn(row.height, 12, 1, 100 - y),
        label: text(row.label, `热区 ${hotspotIndex + 1}`, 30),
        action: safeAction(row.action),
      };
    }) : [];
    return [{
      id,
      type,
      enabled: bool(raw.enabled, true),
      sortOrder: numberIn(raw.sortOrder, (index + 1) * 10, -10000, 10000),
      config: {
        items,
        showLogo: bool(source.showLogo, true),
        showStatus: bool(source.showStatus, true),
        showAddress: bool(source.showAddress, true),
        prefix: text(source.prefix, "公告", 16),
        imageUrl: text(source.imageUrl, "", 1024),
        alt: text(source.alt, "", 80),
        action: safeAction(source.action),
        title: text(source.title, "", 80),
        body: text(source.body, "", 500),
        align,
        textAlign,
        height: numberIn(source.height, 24, 4, 160),
        hotspots,
      },
    }];
  }).sort((a, b) => a.sortOrder - b.sortOrder);
}

function safeNavigation(value: unknown, fallback: DecorationNavigationItem[]): DecorationNavigationItem[] {
  if (!Array.isArray(value)) return fallback;
  const seen = new Set<string>();
  const items = value.slice(0, 4).flatMap((entry, index) => {
    if (!entry || typeof entry !== "object") return [];
    const raw = entry as Partial<DecorationNavigationItem>;
    if (!NAV_KEYS.includes(raw.key as DecorationNavigationItem["key"]) || seen.has(String(raw.key))) return [];
    seen.add(String(raw.key));
    return [{
      key: raw.key as DecorationNavigationItem["key"],
      text: text(raw.text, String(raw.key), 8),
      visible: bool(raw.visible, true),
      sortOrder: numberIn(raw.sortOrder, (index + 1) * 10, -10000, 10000),
    }];
  });
  return items.some((item) => item.key === "home") && items.some((item) => item.key === "menu")
    ? items.sort((a, b) => a.sortOrder - b.sortOrder)
    : fallback;
}

export function normalizeDecoration(value: unknown, store?: Store): DecorationConfig {
  const fallback = defaultDecoration(store);
  if (!value || typeof value !== "object") return fallback;
  const raw = value as Partial<DecorationConfig>;
  const theme = raw.theme || fallback.theme;
  const menu = raw.menu || fallback.menu;
  const navigation = raw.navigation || fallback.navigation;
  const splash = raw.splash || fallback.splash;
  const modules = safeModules(raw.home?.modules, fallback.home.modules);
  return {
    schemaVersion: 1,
    templateKey: text(raw.templateKey, fallback.templateKey, 64),
    theme: {
      primaryColor: color(theme.primaryColor, fallback.theme.primaryColor),
      accentColor: color(theme.accentColor, fallback.theme.accentColor),
      backgroundColor: color(theme.backgroundColor, fallback.theme.backgroundColor),
      surfaceColor: color(theme.surfaceColor, fallback.theme.surfaceColor),
      textColor: color(theme.textColor, fallback.theme.textColor),
      mutedColor: color(theme.mutedColor, fallback.theme.mutedColor),
      navBackgroundColor: color(theme.navBackgroundColor, fallback.theme.navBackgroundColor),
      navTextColor: color(theme.navTextColor, fallback.theme.navTextColor),
      navSelectedColor: color(theme.navSelectedColor, fallback.theme.navSelectedColor),
      radius: theme.radius === "SM" || theme.radius === "MD" ? theme.radius : "LG",
    },
    home: { modules },
    menu: {
      categoryLayout: menu.categoryLayout === "TOP" ? "TOP" : "LEFT",
      productLayout: menu.productLayout === "GRID" ? "GRID" : "LIST",
      showDescription: bool(menu.showDescription, fallback.menu.showDescription),
      showStock: bool(menu.showStock, fallback.menu.showStock),
      showSales: bool(menu.showSales, fallback.menu.showSales),
      showSoldOut: bool(menu.showSoldOut, fallback.menu.showSoldOut),
      loadMode: menu.loadMode === "ALL" ? "ALL" : "BY_CATEGORY",
      productActionMode: menu.productActionMode === "DIRECT_ADD" ? "DIRECT_ADD" : "SKU_SHEET",
      density: menu.density === "COMPACT" ? "COMPACT" : "COMFORTABLE",
    },
    navigation: {
      templateKey: navigation.templateKey === "soft" || navigation.templateKey === "warm" || navigation.templateKey === "dark" ? navigation.templateKey : "classic",
      items: safeNavigation(navigation.items, fallback.navigation.items),
      backgroundColor: color(navigation.backgroundColor, fallback.navigation.backgroundColor),
      textColor: color(navigation.textColor, fallback.navigation.textColor),
      selectedColor: color(navigation.selectedColor, fallback.navigation.selectedColor),
    },
    splash: {
      enabled: bool(splash.enabled, false),
      displayMode: splash.displayMode === "FULLSCREEN" ? "FULLSCREEN" : "POPUP",
      imageUrl: text(splash.imageUrl, "", 1024),
      title: text(splash.title, "", 60),
      subtitle: text(splash.subtitle, "", 160),
      autoCloseSeconds: numberIn(splash.autoCloseSeconds, 5, 0, 30),
      action: safeAction(splash.action),
      frequency: splash.frequency === "EVERY_VISIT" || splash.frequency === "DAILY" ? splash.frequency : "ONCE_PER_VERSION",
      activeFrom: text(splash.activeFrom, "", 40) || undefined,
      activeTo: text(splash.activeTo, "", 40) || undefined,
    },
  };
}

export function decorationStyle(config: DecorationConfig): string {
  const radius = config.theme.radius === "SM" ? 14 : config.theme.radius === "MD" ? 20 : 28;
  return [
    `--ink:${config.theme.textColor}`,
    `--green:${config.theme.primaryColor}`,
    `--lime:${config.theme.accentColor}`,
    `--coffee:${config.theme.primaryColor}`,
    `--cream:${config.theme.backgroundColor}`,
    `--muted:${config.theme.mutedColor}`,
    `--paper:${config.theme.backgroundColor}`,
    `--surface:${config.theme.surfaceColor}`,
    `--radius:${radius}rpx`,
    `background:${config.theme.backgroundColor}`,
    `color:${config.theme.textColor}`,
  ].join(";");
}

export function applyDecorationChrome(config: DecorationConfig): void {
  wx.setNavigationBarColor({
    frontColor: isDark(config.theme.backgroundColor) ? "#ffffff" : "#000000",
    backgroundColor: config.theme.backgroundColor,
    animation: { duration: 160, timingFunc: "easeIn" },
  });
  wx.setTabBarStyle({
    color: config.navigation.textColor,
    selectedColor: config.navigation.selectedColor,
    backgroundColor: config.navigation.backgroundColor,
    borderStyle: "white",
  });
  const pathByKey: Record<DecorationNavigationItem["key"], number> = { home: 0, menu: 1, orders: 2, profile: 3 };
  config.navigation.items.forEach((item) => {
    const iconRoot = `/assets/tabbar/${config.navigation.templateKey}`;
    wx.setTabBarItem({ index: pathByKey[item.key], text: item.text, iconPath: `${iconRoot}/${item.key}.png`, selectedIconPath: `${iconRoot}/${item.key}-selected.png` });
  });
}

function isDark(value: string): boolean {
  const red = Number.parseInt(value.slice(1, 3), 16);
  const green = Number.parseInt(value.slice(3, 5), 16);
  const blue = Number.parseInt(value.slice(5, 7), 16);
  return red * 0.299 + green * 0.587 + blue * 0.114 < 145;
}

function splashStorageKey(storeCode: string): string {
  return `tanban_splash_v1_${storeCode}`;
}

export function shouldDisplaySplash(config: DecorationConfig, storeCode: string, version: number, now = Date.now()): boolean {
  const splash = config.splash;
  if (!splash.enabled || !splash.imageUrl.startsWith("https://")) return false;
  const from = splash.activeFrom ? Date.parse(splash.activeFrom) : Number.NaN;
  const to = splash.activeTo ? Date.parse(splash.activeTo) : Number.NaN;
  if (!Number.isNaN(from) && now < from || !Number.isNaN(to) && now > to) return false;
  if (splash.frequency === "EVERY_VISIT") return true;
  const record = wx.getStorageSync<{ version?: number; date?: string }>(splashStorageKey(storeCode)) || {};
  if (splash.frequency === "DAILY") return record.date !== new Date(now).toISOString().slice(0, 10);
  return record.version !== version;
}

export function rememberSplash(storeCode: string, version: number, now = Date.now()): void {
  wx.setStorageSync(splashStorageKey(storeCode), { version, date: new Date(now).toISOString().slice(0, 10) });
}

export function runDecorationAction(action: DecorationAction): void {
  switch (action.type) {
    case "OPEN_DINE_IN": {
      const app = getApp<TanbanAppOption>();
      if (tableContextForStore(app.globalData.storeCode)) {
        wx.switchTab({ url: "/pages/menu/index" });
      } else {
        wx.showModal({ title: "请先扫描桌码", content: "堂食订单需要绑定当前桌台，请扫描桌面上的点餐二维码后继续。", showCancel: false });
      }
      break;
    }
    case "OPEN_TAKEOUT": {
      const app = getApp<TanbanAppOption>();
      clearTableOrderingContext();
      clearFastFoodContext();
      app.globalData.tableContext = null;
      app.globalData.fastFoodContext = null;
      app.globalData.routeError = "";
      wx.switchTab({ url: "/pages/menu/index" });
      break;
    }
    case "OPEN_DELIVERY":
      wx.showModal({ title: "外卖暂未开放", content: "当前版本支持堂食和门店自取，外卖配送将在后续版本开放。", showCancel: false });
      break;
    case "OPEN_MENU":
      wx.switchTab({ url: "/pages/menu/index" });
      break;
    case "OPEN_ORDERS":
      wx.switchTab({ url: "/pages/orders/index" });
      break;
    case "OPEN_PROFILE":
      wx.switchTab({ url: "/pages/profile/index" });
      break;
    case "OPEN_RECHARGE":
      wx.navigateTo({ url: "/pages/recharge/index" });
      break;
    case "OPEN_MY_COUPONS":
      wx.navigateTo({ url: "/pages/my-coupons/index" });
      break;
    case "OPEN_COUPON_CENTER":
      wx.navigateTo({ url: "/pages/coupons/index" });
      break;
    case "CALL_PHONE":
      if (action.phone) wx.makePhoneCall({ phoneNumber: action.phone });
      break;
    default:
      break;
  }
}
