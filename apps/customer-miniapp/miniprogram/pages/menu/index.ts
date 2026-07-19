import type { CartItem, Category, Product, Sku } from "../../types/domain";
import type { TanbanAppOption } from "../../app";
import { addCartItem, changeCartItemQuantity, readCart } from "../../utils/cart";
import { request } from "../../utils/request";

interface Catalog { categories: Category[]; products: Product[]; }
interface MenuProduct extends Product {
  hasMultipleSkus: boolean;
  selectedQuantity: number;
  selectedItems: CartItem[];
}

function lineKey(item: CartItem): string {
  return `${item.productId}:${item.skuId ?? 0}`;
}

function decorateProducts(products: Product[], cart: CartItem[]): MenuProduct[] {
  return products.map((product) => {
    const selectedItems = cart
      .filter((item) => item.productId === product.id)
      .map((item) => ({ ...item, lineKey: lineKey(item) }));
    return {
      ...product,
      hasMultipleSkus: (product.skus?.filter((sku) => !sku.soldOut).length ?? 0) > 1,
      selectedQuantity: selectedItems.reduce((sum, item) => sum + item.quantity, 0),
      selectedItems,
    };
  });
}

Page({
  data: {
    loading: true,
    categories: [] as Category[],
    products: [] as MenuProduct[],
    activeCategoryId: 0,
    cart: [] as CartItem[],
    cartQuantity: 0,
    cartAmount: 0,
    selectingProduct: null as MenuProduct | null,
    selectableSkus: [] as Sku[],
  },
  onShow() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setCart(readCart(storeCode));
    this.loadCatalog();
  },
  onPullDownRefresh() { this.loadCatalog().finally(() => wx.stopPullDownRefresh()); },
  async loadCatalog() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      const catalog = await request<Catalog>({ url: `/public/stores/${encodeURIComponent(storeCode)}/catalog`, method: "GET" });
      this.setData({
        categories: catalog.categories || [],
        products: decorateProducts(catalog.products || [], this.data.cart),
        activeCategoryId: catalog.categories?.[0]?.id || 0,
        loading: false,
      });
    } catch (error) {
      this.setData({ loading: false });
      wx.showToast({ title: error instanceof Error ? error.message : "菜单加载失败", icon: "none" });
    }
  },
  chooseCategory(event: WechatMiniprogram.BaseEvent) { this.setData({ activeCategoryId: Number(event.currentTarget.dataset.id) }); },
  addProduct(event: WechatMiniprogram.BaseEvent) {
    const product = this.data.products.find((item) => item.id === Number(event.currentTarget.dataset.id));
    if (!product || product.soldOut) return;
    const availableSkus = product.skus?.filter((item) => !item.soldOut) || [];
    if (product.skus?.length && !availableSkus.length) {
      wx.showToast({ title: "该商品规格已售罄", icon: "none" });
      return;
    }
    if (availableSkus.length > 1) {
      this.setData({ selectingProduct: product, selectableSkus: availableSkus });
      return;
    }
    const sku = availableSkus[0] || product.skus?.[0];
    this.addSkuToCart(product, sku);
  },
  chooseSku(event: WechatMiniprogram.BaseEvent) {
    const product = this.data.selectingProduct;
    const sku = this.data.selectableSkus.find((item) => item.id === Number(event.currentTarget.dataset.skuId));
    if (!product || !sku) return;
    this.addSkuToCart(product, sku);
    this.closeSkuPicker();
  },
  addSkuToCart(product: Product, sku?: Sku) {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    this.setCart(addCartItem(storeCode, { productId: product.id, skuId: sku?.id, name: product.name, skuName: sku?.name, price: sku?.price ?? product.price, quantity: 1 }));
  },
  incrementCartItem(event: WechatMiniprogram.BaseEvent) {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    const productId = Number(event.currentTarget.dataset.productId);
    const skuIdValue = event.currentTarget.dataset.skuId;
    const skuId = skuIdValue === undefined || skuIdValue === "" ? undefined : Number(skuIdValue);
    this.setCart(changeCartItemQuantity(storeCode, productId, skuId, 1));
  },
  decreaseCartItem(event: WechatMiniprogram.BaseEvent) {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    const productId = Number(event.currentTarget.dataset.productId);
    const skuIdValue = event.currentTarget.dataset.skuId;
    const skuId = skuIdValue === undefined || skuIdValue === "" ? undefined : Number(skuIdValue);
    this.setCart(changeCartItemQuantity(storeCode, productId, skuId, -1));
  },
  closeSkuPicker() {
    this.setData({ selectingProduct: null, selectableSkus: [] });
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
    if (!this.data.cartQuantity) return wx.showToast({ title: "请先选择商品", icon: "none" });
    wx.navigateTo({ url: "/pages/checkout/index" });
  },
});
