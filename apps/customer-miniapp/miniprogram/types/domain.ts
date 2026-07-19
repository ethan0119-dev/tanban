export interface StoreTheme {
  primaryColor?: string;
  bannerUrl?: string;
  announcement?: string;
}

export type DecorationActionType = "NONE" | "OPEN_MENU" | "OPEN_ORDERS" | "OPEN_PROFILE" | "CALL_PHONE";

export interface DecorationAction {
  type: DecorationActionType;
  phone?: string;
}

export interface DecorationTheme {
  primaryColor: string;
  accentColor: string;
  backgroundColor: string;
  surfaceColor: string;
  textColor: string;
  mutedColor: string;
  navBackgroundColor: string;
  navTextColor: string;
  navSelectedColor: string;
  radius: "SM" | "MD" | "LG";
}

export interface DecorationModuleItem {
  imageUrl?: string;
  title?: string;
  subtitle?: string;
  action?: DecorationAction;
}

export interface DecorationModuleConfig {
  items?: DecorationModuleItem[];
  showLogo?: boolean;
  showStatus?: boolean;
  showAddress?: boolean;
  prefix?: string;
  imageUrl?: string;
  alt?: string;
  action?: DecorationAction;
  title?: string;
  body?: string;
  align?: "LEFT" | "CENTER" | "RIGHT";
  textAlign?: "left" | "center" | "right";
  height?: number;
}

export type DecorationModuleType =
  | "HERO_BANNER"
  | "STORE_HEADER"
  | "ANNOUNCEMENT"
  | "QUICK_ACTIONS"
  | "IMAGE"
  | "TEXT"
  | "SPACER";

export interface DecorationModule {
  id: string;
  type: DecorationModuleType;
  enabled: boolean;
  sortOrder: number;
  config: DecorationModuleConfig;
}

export interface DecorationMenu {
  categoryLayout: "LEFT" | "TOP";
  productLayout: "LIST" | "GRID";
  showDescription: boolean;
  showStock: boolean;
  showSales: boolean;
  showSoldOut: boolean;
  loadMode: "BY_CATEGORY" | "ALL";
  productActionMode: "SKU_SHEET" | "DIRECT_ADD";
  density: "COMPACT" | "COMFORTABLE";
}

export interface DecorationNavigationItem {
  key: "home" | "menu" | "orders" | "profile";
  text: string;
  visible: boolean;
  sortOrder: number;
}

export interface DecorationNavigation {
  items: DecorationNavigationItem[];
  backgroundColor: string;
  textColor: string;
  selectedColor: string;
}

export interface DecorationSplash {
  enabled: boolean;
  displayMode: "FULLSCREEN" | "POPUP";
  imageUrl: string;
  title: string;
  subtitle: string;
  autoCloseSeconds: number;
  action: DecorationAction;
  frequency: "EVERY_VISIT" | "DAILY" | "ONCE_PER_VERSION";
  activeFrom?: string;
  activeTo?: string;
}

export interface DecorationConfig {
  schemaVersion: 1;
  templateKey: string;
  theme: DecorationTheme;
  home: { modules: DecorationModule[] };
  menu: DecorationMenu;
  navigation: DecorationNavigation;
  splash: DecorationSplash;
}

export interface Store {
  id: number;
  code: string;
  name: string;
  logoUrl?: string;
  address?: string;
  businessStatus: "OPEN" | "CLOSED";
  theme?: StoreTheme;
  decoration?: DecorationConfig;
  decorationVersion?: number;
}

/** A server-verified dine-in table route kept while the customer orders. */
export interface TableOrderingContext {
  publicScene: string;
  storeCode: string;
  storeName: string;
  tablePublicId: string;
  tableName: string;
  tableCode?: string;
  areaName?: string;
  resolvedAt: number;
  validUntil: number;
}

export interface Category {
  id: number;
  name: string;
  sortOrder: number;
}

export interface Sku {
  id: number;
  name: string;
  price: number;
  stock: number;
  soldOut: boolean;
}

export interface ProductOptionValue {
  id: number;
  name: string;
  priceDeltaCents: number;
  isDefault: boolean;
  selected?: boolean;
}

export interface ProductOptionGroup {
  id: number;
  name: string;
  selectionMode: "SINGLE" | "MULTIPLE";
  minSelect: number;
  maxSelect: number;
  values: ProductOptionValue[];
}

export interface ModifierItem {
  id: number;
  name: string;
  priceCents: number;
  isDefault: boolean;
  selected?: boolean;
}

export interface ModifierGroup {
  id: number;
  name: string;
  minSelect: number;
  maxSelect: number;
  items: ModifierItem[];
}

export interface CartModifierSelection {
  groupId: number;
  modifierItemId: number;
  quantity: number;
}

export interface Product {
  id: number;
  categoryId: number;
  name: string;
  description?: string;
  imageUrl?: string;
  price: number;
  stock: number;
  soldOut: boolean;
  sales?: number;
  skus?: Sku[];
  optionGroups?: ProductOptionGroup[];
  modifierGroups?: ModifierGroup[];
}

export interface CartItem {
  productId: number;
  skuId?: number;
  name: string;
  skuName?: string;
  price: number;
  quantity: number;
  lineKey?: string;
  optionValueIds?: number[];
  modifiers?: CartModifierSelection[];
  optionSummary?: string;
  itemRemark?: string;
}

export interface Order {
  id: number;
  orderNo: string;
  pickupCode?: string;
  status: string;
  paymentStatus: string;
  fulfillmentType?: "PICKUP" | "DINE_IN";
  orderScene?: "TAKEOUT" | "DINE_IN";
  order_scene?: "TAKEOUT" | "DINE_IN";
  tablePublicId?: string;
  tableName?: string;
  tableCode?: string;
  tableAreaName?: string;
  table?: {
    publicId?: string;
    name?: string;
    tableCode?: string;
    areaName?: string;
  };
  remark?: string;
  amount: number;
  createdAt: string;
  items?: Array<CartItem & { amount: number }>;
}
