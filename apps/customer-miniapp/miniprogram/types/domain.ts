export interface StoreTheme {
  primaryColor?: string;
  bannerUrl?: string;
  announcement?: string;
}

export type DecorationActionType =
  | "NONE"
  | "OPEN_MENU"
  | "OPEN_DINE_IN"
  | "OPEN_TAKEOUT"
  | "OPEN_DELIVERY"
  | "OPEN_ORDERS"
  | "OPEN_PROFILE"
  | "OPEN_RECHARGE"
  | "OPEN_MY_COUPONS"
  | "OPEN_COUPON_CENTER"
  | "CALL_PHONE";

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

export interface DecorationHotspot {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
  label: string;
  action: DecorationAction;
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
  hotspots?: DecorationHotspot[];
}

export type DecorationModuleType =
  | "HERO_BANNER"
  | "STORE_HEADER"
  | "ANNOUNCEMENT"
  | "QUICK_ACTIONS"
  | "IMAGE"
  | "HOTSPOT_IMAGE"
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
  businessStatusReason?: string;
  businessStatusMessage?: string;
  acceptingOrders?: boolean;
  timezone?: string;
  nextOpenAt?: string;
  businessHoursSummary?: string;
  theme?: StoreTheme;
  decoration?: DecorationConfig;
  decorationVersion?: number;
  orderingSettings?: {
    orderingMode: "SINGLE_PERSON" | "MULTI_PERSON";
    distanceCheckEnabled: boolean;
    distanceLimitM: number;
    requireCustomerPhone: boolean;
    allowOrderRemark: boolean;
    allowItemRemark: boolean;
  };
  customerService?: { phone?: string; wechat?: string; qrUrl?: string };
  legal?: { privacyPolicy?: string; userAgreement?: string };
}

export interface MarketingCoupon {
  id: number;
  name: string;
  description?: string;
  coupon_type: "CASH" | "FULL_REDUCTION";
  threshold_cents: number;
  discount_cents: number;
  distribution_mode: "PUBLIC_CLAIM" | "MANUAL_ONLY" | "LOTTERY_ONLY";
  total_stock: number;
  issued_count: number;
  remaining_stock?: number;
  per_subject_limit: number;
  valid_from?: string;
  valid_to?: string;
  valid_days?: number;
  order_types?: Array<"DINE_IN" | "TAKEOUT" | "DELIVERY">;
  status: "DRAFT" | "ACTIVE" | "PAUSED" | "ENDED";
}

export interface MarketingPlacement {
  id: number;
  name: string;
  placement_code: "HOME_POPUP" | "MENU_POPUP" | "CHECKOUT_POPUP" | "ORDER_RESULT_POPUP" | "PROFILE_POPUP";
  image_url: string;
  title?: string;
  subtitle?: string;
  action_type: "NONE" | "OPEN_MENU" | "OPEN_COUPONS" | "CLAIM_COUPON" | "OPEN_LOTTERY";
  action_target_id?: number;
  frequency: "EVERY_VISIT" | "DAILY" | "ONCE_PER_CAMPAIGN";
  priority: number;
}

export interface MarketingLotteryPrize {
  id: number;
  name: string;
  prize_type: "THANKS" | "COUPON";
  coupon_campaign_id?: number;
  weight: number;
  total_stock: number;
  awarded_count: number;
}

export interface MarketingLottery {
  id: number;
  name: string;
  description?: string;
  active_from?: string;
  active_to?: string;
  daily_limit: number;
  total_limit: number;
  draw_count: number;
  terms?: string;
  status: "DRAFT" | "ACTIVE" | "PAUSED" | "ENDED";
  prizes?: MarketingLotteryPrize[];
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

/** A server-verified fast-food pickup plate route kept while the customer orders. */
export interface FastFoodOrderingContext {
  publicId: string;
  storeCode: string;
  storeName: string;
  plateCode: string;
  plateName: string;
  resolvedAt: number;
  validUntil: number;
}

export interface Category {
  id: number;
  name: string;
  sortOrder: number;
  inStoreEnabled?: boolean;
  deliveryEnabled?: boolean;
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
  images?: Array<{
    url: string;
    isPrimary: boolean;
    sortOrder: number;
  }>;
  price: number;
  stock: number;
  soldOut: boolean;
  inStoreEnabled?: boolean;
  deliveryEnabled?: boolean;
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
  fastFoodPlatePublicId?: string;
  fastFoodPlateCode?: string;
  fastFoodPlateName?: string;
  fastFoodPlate?: { publicId?: string; plateCode?: string; plateName?: string };
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
