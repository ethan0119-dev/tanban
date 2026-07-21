import type { CartItem, Category, DecorationConfig, FastFoodOrderingContext, ModifierGroup, Product, ProductOptionGroup, Sku, Store, TableOrderingContext } from "../../types/domain";
import type { TanbanAppOption } from "../../app";
import { addCartItem, cartLineKey, changeCartLineQuantity, readCart } from "../../utils/cart";
import { applyDecorationChrome, decorationStyle, defaultDecoration, normalizeDecoration } from "../../utils/decoration";
import { request } from "../../utils/request";
import { tableContextForStore } from "../../utils/table-context";
import { fastFoodContextForStore } from "../../utils/fast-food-context";
import { clearTableOrderingContext } from "../../utils/table-context";
import { clearFastFoodContext } from "../../utils/fast-food-context";
import { rememberPageAppearance } from "../../utils/page-appearance";
import { customerSafeErrorMessage, showUnavailableFeature } from "../../utils/availability";
import { formatBeijingDateTime } from "../../utils/datetime";

interface Catalog { store?: Store; categories: Category[]; products: Product[]; }
interface MenuProduct extends Product {
  hasMultipleSkus: boolean;
  requiresConfiguration: boolean;
  selectedQuantity: number;
  selectedItems: CartItem[];
}

function decorateProducts(products: Product[], cart: CartItem[]): MenuProduct[] {
  return products.map((product) => {
    const selectedItems = cart
      .filter((item) => item.productId === product.id)
      .map((item) => ({ ...item, lineKey: cartLineKey(item) }));
    return {
      ...product,
      hasMultipleSkus: (product.skus?.filter((sku) => !sku.soldOut).length ?? 0) > 1,
      requiresConfiguration: (product.skus?.filter((sku) => !sku.soldOut).length ?? 0) > 1 || Boolean(product.optionGroups?.length || product.modifierGroups?.length),
      selectedQuantity: selectedItems.reduce((sum, item) => sum + item.quantity, 0),
      selectedItems,
    };
  });
}

Page({
  data: {
    loading: true,
    storeCode: "",
    store: null as Store | null,
    categories: [] as Category[],
    products: [] as MenuProduct[],
    activeCategoryId: 0,
    cart: [] as CartItem[],
    cartQuantity: 0,
    cartAmount: 0,
    selectingProduct: null as MenuProduct | null,
    selectableSkus: [] as Sku[],
    selectedSkuId: 0,
    pickerOptionGroups: [] as ProductOptionGroup[],
    pickerModifierGroups: [] as ModifierGroup[],
    pickerPrice: 0,
    decoration: defaultDecoration() as DecorationConfig,
    appearanceStyle: "",
    menuLayoutClass: "category-left product-list-view density-comfortable",
    tableContext: null as TableOrderingContext | null,
    fastFoodContext: null as FastFoodOrderingContext | null,
    orderMode: "TAKEOUT" as "DINE_IN" | "TAKEOUT",
    routeError: "",
  },
  async onShow() {
    const app = getApp<TanbanAppOption>();
    await app.globalData.routeReady;
    if (app.globalData.routeError) {
      this.setData({ loading: false, routeError: app.globalData.routeError, tableContext: null, fastFoodContext: null });
      return;
    }
    const storeCode = app.globalData.storeCode;
    const tableContext = tableContextForStore(storeCode);
    const fastFoodContext = fastFoodContextForStore(storeCode);
    this.setData({ storeCode, routeError: "", tableContext, fastFoodContext, orderMode: tableContext ? "DINE_IN" : "TAKEOUT" });
    this.setCart(readCart(storeCode));
    await this.loadCatalog();
  },
  onPullDownRefresh() { this.loadCatalog().finally(() => wx.stopPullDownRefresh()); },
  async loadCatalog() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      const catalog = await request<Catalog>({ url: `/public/stores/${encodeURIComponent(storeCode)}/catalog`, method: "GET" });
      if (catalog.store?.nextOpenAt) catalog.store.nextOpenAt = formatBeijingDateTime(catalog.store.nextOpenAt);
      const decoration = normalizeDecoration(catalog.store?.decoration, catalog.store);
      if (catalog.store) rememberPageAppearance(catalog.store);
      const visibleProducts = (catalog.products || []).filter((product) => decoration.menu.showSoldOut || !product.soldOut);
      this.setData({
        store: catalog.store || null,
        categories: catalog.categories || [],
        products: decorateProducts(visibleProducts, this.data.cart),
        activeCategoryId: decoration.menu.loadMode === "ALL" ? 0 : catalog.categories?.[0]?.id || 0,
        decoration,
        appearanceStyle: decorationStyle(decoration),
        menuLayoutClass: [
          decoration.menu.categoryLayout === "TOP" ? "category-top" : "category-left",
          decoration.menu.productLayout === "GRID" ? "product-grid-view" : "product-list-view",
          decoration.menu.density === "COMPACT" ? "density-compact" : "density-comfortable",
        ].join(" "),
        loading: false,
      });
      applyDecorationChrome(decoration);
    } catch (error) {
      this.setData({ loading: false });
      wx.showToast({ title: customerSafeErrorMessage(error, "菜单暂时无法加载，请稍后重试。"), icon: "none" });
    }
  },
  chooseCategory(event: WechatMiniprogram.BaseEvent) { this.setData({ activeCategoryId: Number(event.currentTarget.dataset.id) }); },
  chooseMode(event: WechatMiniprogram.BaseEvent) {
    const mode = String(event.currentTarget.dataset.mode || "TAKEOUT");
    if (mode === "DELIVERY") {
      showUnavailableFeature("DELIVERY");
      return;
    }
    const app = getApp<TanbanAppOption>();
    const storeCode = app.globalData.storeCode;
    if (mode === "DINE_IN") {
      const tableContext = tableContextForStore(storeCode);
      if (!tableContext) {
        wx.showModal({ title: "请先扫描桌码", content: "堂食点餐需要绑定桌台，请扫描桌面上的点餐二维码。", showCancel: false });
        return;
      }
      this.setData({ orderMode: "DINE_IN", tableContext, fastFoodContext: null });
      return;
    }
    clearTableOrderingContext();
    clearFastFoodContext();
    app.globalData.tableContext = null;
    app.globalData.fastFoodContext = null;
    app.globalData.routeError = "";
    this.setData({ orderMode: "TAKEOUT", tableContext: null, fastFoodContext: null });
  },
  addProduct(event: WechatMiniprogram.BaseEvent) {
    if (this.data.store?.businessStatus !== "OPEN") {
      wx.showToast({ title: this.data.store?.businessStatusMessage || "门店休息中，暂时不能下单", icon: "none" });
      return;
    }
    const product = this.data.products.find((item) => item.id === Number(event.currentTarget.dataset.id));
    if (!product || product.soldOut) return;
    const availableSkus = product.skus?.filter((item) => !item.soldOut) || [];
    if (product.skus?.length && !availableSkus.length) {
      wx.showToast({ title: "该商品规格已售罄", icon: "none" });
      return;
    }
    const shouldOpenSheet = availableSkus.length > 1 || Boolean(product.optionGroups?.length || product.modifierGroups?.length) || this.data.decoration.menu.productActionMode === "SKU_SHEET";
    if (shouldOpenSheet) {
      const optionGroups = (product.optionGroups || []).map((group) => ({
        ...group,
        values: group.values.map((value) => ({ ...value, selected: Boolean(value.isDefault) })),
      }));
      const modifierGroups = (product.modifierGroups || []).map((group) => ({
        ...group,
        items: group.items.map((item) => ({ ...item, selected: Boolean(item.isDefault) })),
      }));
      const selectedSkuId = availableSkus[0]?.id || 0;
      this.setData({ selectingProduct: product, selectableSkus: availableSkus, selectedSkuId, pickerOptionGroups: optionGroups, pickerModifierGroups: modifierGroups });
      this.refreshPickerPrice();
      return;
    }
    const sku = availableSkus[0] || product.skus?.[0];
    this.addSkuToCart(product, sku);
  },
  chooseSku(event: WechatMiniprogram.BaseEvent) {
    const sku = this.data.selectableSkus.find((item) => item.id === Number(event.currentTarget.dataset.skuId));
    if (!sku) return;
    this.setData({ selectedSkuId: sku.id });
    this.refreshPickerPrice();
  },
  toggleOption(event: WechatMiniprogram.BaseEvent) {
    const groupId = Number(event.currentTarget.dataset.groupId);
    const valueId = Number(event.currentTarget.dataset.valueId);
    const groups = this.data.pickerOptionGroups.map((group) => {
      if (group.id !== groupId) return group;
      const target = group.values.find((value) => value.id === valueId);
      if (!target) return group;
      if (group.selectionMode === "SINGLE") {
        if (target.selected && group.minSelect === 0) {
          return { ...group, values: group.values.map((value) => ({ ...value, selected: false })) };
        }
        return { ...group, values: group.values.map((value) => ({ ...value, selected: value.id === valueId })) };
      }
      const selectedCount = group.values.filter((value) => value.selected).length;
      if (!target.selected && selectedCount >= group.maxSelect) {
        wx.showToast({ title: `最多选择 ${group.maxSelect} 项`, icon: "none" });
        return group;
      }
      return { ...group, values: group.values.map((value) => value.id === valueId ? { ...value, selected: !value.selected } : value) };
    });
    this.setData({ pickerOptionGroups: groups });
    this.refreshPickerPrice();
  },
  toggleModifier(event: WechatMiniprogram.BaseEvent) {
    const groupId = Number(event.currentTarget.dataset.groupId);
    const itemId = Number(event.currentTarget.dataset.itemId);
    const groups = this.data.pickerModifierGroups.map((group) => {
      if (group.id !== groupId) return group;
      const target = group.items.find((item) => item.id === itemId);
      if (!target) return group;
      const selectedCount = group.items.filter((item) => item.selected).length;
      if (!target.selected && selectedCount >= group.maxSelect) {
        wx.showToast({ title: `最多选择 ${group.maxSelect} 项`, icon: "none" });
        return group;
      }
      return { ...group, items: group.items.map((item) => item.id === itemId ? { ...item, selected: !item.selected } : item) };
    });
    this.setData({ pickerModifierGroups: groups });
    this.refreshPickerPrice();
  },
  refreshPickerPrice() {
    const product = this.data.selectingProduct;
    if (!product) return;
    const sku = this.data.selectableSkus.find((item) => item.id === this.data.selectedSkuId);
    const optionDelta = this.data.pickerOptionGroups.reduce((sum, group) => sum + group.values.filter((value) => value.selected).reduce((valueSum, value) => valueSum + value.priceDeltaCents, 0), 0);
    const modifierDelta = this.data.pickerModifierGroups.reduce((sum, group) => sum + group.items.filter((item) => item.selected).reduce((itemSum, item) => itemSum + item.priceCents, 0), 0);
    this.setData({ pickerPrice: (sku?.price ?? product.price) + optionDelta + modifierDelta });
  },
  confirmConfiguredProduct() {
    const product = this.data.selectingProduct;
    const sku = this.data.selectableSkus.find((item) => item.id === this.data.selectedSkuId);
    if (!product || !sku) return;
    for (const group of this.data.pickerOptionGroups) {
      const count = group.values.filter((value) => value.selected).length;
      if (count < group.minSelect || count > group.maxSelect) {
        wx.showToast({ title: `${group.name}需选择 ${group.minSelect}–${group.maxSelect} 项`, icon: "none" });
        return;
      }
    }
    for (const group of this.data.pickerModifierGroups) {
      const count = group.items.filter((item) => item.selected).length;
      if (count < group.minSelect || count > group.maxSelect) {
        wx.showToast({ title: `${group.name}需选择 ${group.minSelect}–${group.maxSelect} 项`, icon: "none" });
        return;
      }
    }
    const optionValueIds = this.data.pickerOptionGroups.flatMap((group) => group.values.filter((value) => value.selected).map((value) => value.id));
    const modifiers = this.data.pickerModifierGroups.flatMap((group) => group.items.filter((item) => item.selected).map((item) => ({ groupId: group.id, modifierItemId: item.id, quantity: 1 })));
    const optionSummary = [sku.name, ...this.data.pickerOptionGroups.flatMap((group) => group.values.filter((value) => value.selected).map((value) => value.name)), ...this.data.pickerModifierGroups.flatMap((group) => group.items.filter((item) => item.selected).map((item) => `加${item.name}`))].filter(Boolean).join(' / ');
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setCart(addCartItem(storeCode, { productId: product.id, skuId: sku.id, name: product.name, skuName: sku.name, price: this.data.pickerPrice, quantity: 1, optionValueIds, modifiers, optionSummary }));
    this.closeSkuPicker();
  },
  addSkuToCart(product: Product, sku?: Sku) {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setCart(addCartItem(storeCode, { productId: product.id, skuId: sku?.id, name: product.name, skuName: sku?.name, price: sku?.price ?? product.price, quantity: 1 }));
  },
  incrementCartItem(event: WechatMiniprogram.BaseEvent) {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    const key = String(event.currentTarget.dataset.lineKey || '');
    this.setCart(changeCartLineQuantity(storeCode, key, 1));
  },
  decreaseCartItem(event: WechatMiniprogram.BaseEvent) {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    const key = String(event.currentTarget.dataset.lineKey || '');
    this.setCart(changeCartLineQuantity(storeCode, key, -1));
  },
  closeSkuPicker() {
    this.setData({ selectingProduct: null, selectableSkus: [], selectedSkuId: 0, pickerOptionGroups: [], pickerModifierGroups: [], pickerPrice: 0 });
  },
  noop() {},
  setCart(cart: CartItem[]) {
    this.setData({
      cart,
      products: decorateProducts(this.data.products, cart),
      cartQuantity: cart.reduce((sum, item) => sum + item.quantity, 0),
      cartAmount: cart.reduce((sum, item) => sum + item.price * item.quantity, 0),
    });
  },
  countForProduct(productId: number): number { return this.data.cart.find((item) => item.productId === productId)?.quantity || 0; },
  checkout() {
    if (this.data.routeError) return wx.showToast({ title: this.data.routeError, icon: "none" });
    if (this.data.store?.businessStatus !== "OPEN") return wx.showToast({ title: this.data.store?.businessStatusMessage || "门店休息中，暂时不能下单", icon: "none" });
    if (!this.data.cartQuantity) return wx.showToast({ title: "请先选择商品", icon: "none" });
    wx.navigateTo({ url: "/pages/checkout/index" });
  },
});
