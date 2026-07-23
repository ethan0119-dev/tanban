import { env } from "../../config/env";
import type { TanbanAppOption } from "../../app";
import type { CartItem, FastFoodOrderingContext, MarketingCoupon, Order, Store, TableOrderingContext } from "../../types/domain";
import { cartLineKey, clearCart, readCart } from "../../utils/cart";
import { checkoutBlockedByStoreStatus, checkoutFlowFor, checkoutNeedsFreshOrder, checkoutOrderIsClosed, clearCheckoutFlow, markCheckoutSubmitted, rememberCheckoutDetails, rememberCheckoutOrder } from "../../utils/checkout";
import { customerGuestKey } from "../../utils/customer";
import { rememberOrder } from "../../utils/orders";
import { ApiError, request } from "../../utils/request";
import { revalidateTableOrderingContext, sameTableContext, tableContextForStore, tableOrderFields } from "../../utils/table-context";
import { fastFoodContextForStore, revalidateFastFoodContext, sameFastFoodContext } from "../../utils/fast-food-context";
import { rememberPageAppearance } from "../../utils/page-appearance";
import { customerSafeErrorMessage } from "../../utils/availability";
import { formatBeijingDateTime } from "../../utils/datetime";
import { bestEligibleCoupon, forgetClaimedCoupon } from "../../utils/coupon-wallet";

interface PaymentResult { id: number; provider: string; status: string; wxPayParams?: WechatMiniprogram.RequestPaymentOption; }
interface TextInputEvent extends WechatMiniprogram.BaseEvent { detail: { value: string } }

function customerLocation(): Promise<{ customerLatitude: number; customerLongitude: number }> {
  return new Promise((resolve, reject) => wx.getLocation({
    type: "gcj02",
    success: (result) => resolve({ customerLatitude: result.latitude, customerLongitude: result.longitude }),
    fail: () => reject(new Error("门店已启用距离校验，请在微信设置中允许定位后再下单")),
  }));
}

Page({
  data: { storeCode: "", store: null as Store | null, cart: [] as CartItem[], subtotalAmount: 0, discountAmount: 0, amount: 0, selectedCoupon: null as MarketingCoupon | null, remark: "", customerPhone: "", fulfillmentType: "PICKUP" as "PICKUP" | "DINE_IN", tableContext: null as TableOrderingContext | null, fastFoodContext: null as FastFoodOrderingContext | null, detailsLocked: false, submitting: false, checkoutKey: "", orderNo: "", paymentMode: env.paymentMode, appearanceStyle: "" },
  async onLoad() {
    const app = getApp<TanbanAppOption>();
    await app.globalData.routeReady;
    if (app.globalData.routeError) {
      wx.showModal({
        title: "暂时无法下单",
        content: app.globalData.routeError,
        showCancel: false,
        complete: () => wx.navigateBack(),
      });
      return;
    }
    const storeCode = app.globalData.storeCode;
    let store: Store | null = null;
    try {
      store = await request<Store>({ url: `/public/stores/${encodeURIComponent(storeCode)}`, method: "GET" });
      if (store.nextOpenAt) store.nextOpenAt = formatBeijingDateTime(store.nextOpenAt);
      this.setData({ appearanceStyle: rememberPageAppearance(store).appearanceStyle });
    } catch {
      // 创建订单时服务端仍会做最终营业状态校验。
    }
    const tableContext = tableContextForStore(storeCode);
    const fastFoodContext = fastFoodContextForStore(storeCode);
    const cart = readCart(storeCode);
    if (!cart.length) {
      void wx.navigateBack();
      return;
    }
    const flow = checkoutFlowFor(storeCode, cart, tableContext, fastFoodContext);
    const subtotalAmount = cart.reduce((sum, item) => sum + item.price * item.quantity, 0);
    const selectedCoupon = bestEligibleCoupon(storeCode, subtotalAmount, tableContext ? "DINE_IN" : "TAKEOUT");
    const discountAmount = Math.min(subtotalAmount, selectedCoupon?.discount_cents || 0);
    this.setData({
      storeCode,
      store,
      tableContext,
      fastFoodContext,
      cart: cart.map((item) => ({ ...item, lineKey: cartLineKey(item) })),
      subtotalAmount,
      discountAmount,
      amount: subtotalAmount - discountAmount,
      selectedCoupon,
      fulfillmentType: flow.fulfillmentType,
      remark: flow.remark,
      detailsLocked: flow.submitted,
      submitting: Boolean(flow.orderNo),
      checkoutKey: flow.idempotencyKey,
      orderNo: flow.orderNo,
    });
    if (flow.orderNo) void this.restoreExistingOrder(flow.idempotencyKey, flow.orderNo);
  },
  async restoreExistingOrder(flowKey: string, orderNo: string) {
    try {
      const order = await request<Order>({ url: `/public/orders/${encodeURIComponent(orderNo)}`, method: "GET" });
      if (checkoutOrderIsClosed(order.status)) {
        clearCheckoutFlow(flowKey);
        const fresh = checkoutFlowFor(this.data.storeCode, this.data.cart, this.data.tableContext, this.data.fastFoodContext);
        this.setData({ checkoutKey: fresh.idempotencyKey, orderNo: "", fulfillmentType: fresh.fulfillmentType, remark: fresh.remark, detailsLocked: false });
        return;
      }
      this.setData({
        amount: order.amount,
        discountAmount: Math.max(0, this.data.subtotalAmount - order.amount),
        fulfillmentType: this.data.tableContext || order.fulfillmentType === "DINE_IN" ? "DINE_IN" : "PICKUP",
        remark: order.remark || "",
        detailsLocked: true,
      });
    } catch (error) {
      wx.showToast({ title: customerSafeErrorMessage(error, "订单暂时无法恢复，请稍后重试。"), icon: "none" });
    } finally {
      this.setData({ submitting: false });
    }
  },
  setRemark(event: TextInputEvent) {
    if (this.data.detailsLocked) return;
    const remark = event.detail.value;
    this.setData({ remark });
    rememberCheckoutDetails(this.data.checkoutKey, this.data.fulfillmentType, remark);
  },
  setCustomerPhone(event: TextInputEvent) {
    if (this.data.detailsLocked) return;
    this.setData({ customerPhone: event.detail.value.trim() });
  },
  chooseFulfillment() {
    if (this.data.tableContext) {
      wx.showToast({ title: `当前为${this.data.tableContext.tableName}堂食点单`, icon: "none" });
      return;
    }
    if (this.data.detailsLocked) {
      wx.showToast({ title: "订单已生成，取餐信息不能修改", icon: "none" });
      return;
    }
    const fulfillmentType = "PICKUP" as const;
    this.setData({ fulfillmentType });
    rememberCheckoutDetails(this.data.checkoutKey, fulfillmentType, this.data.remark);
  },
  async submitOrder() {
    if (this.data.submitting) return;
    this.setData({ submitting: true });
    try {
      const storeCode = this.data.storeCode;
      const app = getApp<TanbanAppOption>();
      await app.globalData.routeReady;
      if (app.globalData.routeError) throw new Error(app.globalData.routeError);
      if (app.globalData.storeCode !== storeCode) {
        throw new Error("当前门店已切换，请返回菜单后重新确认商品");
      }
      const activeTableContext = tableContextForStore(storeCode);
      if (!sameTableContext(activeTableContext, this.data.tableContext)) {
        throw new Error("点单桌台已切换，请返回菜单并重新确认");
      }
      const activeFastFoodContext = fastFoodContextForStore(storeCode);
      if (!sameFastFoodContext(activeFastFoodContext, this.data.fastFoodContext)) {
        throw new Error("快餐码牌已切换，请返回菜单并重新确认");
      }
      const latestStore = await request<Store>({ url: `/public/stores/${encodeURIComponent(storeCode)}`, method: "GET" });
      this.setData({ store: latestStore });
      if (checkoutBlockedByStoreStatus(latestStore.businessStatus, this.data.orderNo) || (latestStore.acceptingOrders === false && !this.data.orderNo)) {
        throw new Error(latestStore.businessStatusMessage || "门店休息中，暂时不能下单");
      }
      if (latestStore.orderingSettings?.requireCustomerPhone && !/^1\d{10}$/.test(this.data.customerPhone)) {
        throw new Error("请填写可联系的 11 位手机号");
      }
      const locationFields = latestStore.orderingSettings?.distanceCheckEnabled ? await customerLocation() : {};
      const orderRemark = latestStore.orderingSettings?.allowOrderRemark === false ? "" : this.data.remark;
      const allowItemRemark = latestStore.orderingSettings?.allowItemRemark !== false;
      let tableContext = this.data.tableContext;
      let fastFoodContext = this.data.fastFoodContext;
      if (tableContext) {
        const storedContext = tableContextForStore(storeCode);
        if (!storedContext || !sameTableContext(storedContext, tableContext)) {
          throw new Error("桌码已失效或当前门店已切换，请重新扫码后下单");
        }
        // Never trust a cached table at the money boundary. Disabled/rebound table
        // codes fail here before an order or payment can be created.
        tableContext = await revalidateTableOrderingContext(tableContext);
        app.globalData.tableContext = tableContext;
        app.globalData.storeCode = tableContext.storeCode;
        this.setData({ tableContext, fulfillmentType: "DINE_IN" });
      }
      if (fastFoodContext) {
        fastFoodContext = await revalidateFastFoodContext(fastFoodContext);
        app.globalData.fastFoodContext = fastFoodContext;
        app.globalData.storeCode = fastFoodContext.storeCode;
        this.setData({ fastFoodContext, fulfillmentType: "PICKUP" });
      }
      // 每次提交都重新经过持久化 flow 校验，这样页面长时间停留时 TTL 也会生效。
      let flow = checkoutFlowFor(storeCode, this.data.cart, tableContext, fastFoodContext);
      if (flow.idempotencyKey !== this.data.checkoutKey || flow.orderNo !== this.data.orderNo) {
        this.setData({ checkoutKey: flow.idempotencyKey, orderNo: flow.orderNo });
      }
      if (!flow.submitted) {
        rememberCheckoutDetails(flow.idempotencyKey, tableContext ? "DINE_IN" : "PICKUP", this.data.remark);
        markCheckoutSubmitted(flow.idempotencyKey);
        flow = checkoutFlowFor(storeCode, this.data.cart, tableContext, fastFoodContext);
        this.setData({ detailsLocked: true });
      }
      let order: Order;
      if (flow.orderNo) {
        order = await request<Order>({ url: `/public/orders/${encodeURIComponent(flow.orderNo)}`, method: "GET" });
        if (checkoutOrderIsClosed(order.status)) {
          clearCheckoutFlow(flow.idempotencyKey);
          flow = checkoutFlowFor(storeCode, this.data.cart, tableContext, fastFoodContext);
          rememberCheckoutDetails(flow.idempotencyKey, tableContext ? "DINE_IN" : "PICKUP", this.data.remark);
          markCheckoutSubmitted(flow.idempotencyKey);
          flow = checkoutFlowFor(storeCode, this.data.cart, tableContext, fastFoodContext);
          this.setData({ checkoutKey: flow.idempotencyKey, orderNo: "", detailsLocked: true });
          order = await request<Order>({
            url: `/public/stores/${encodeURIComponent(storeCode)}/orders`, method: "POST",
            header: { "Idempotency-Key": flow.idempotencyKey },
            data: { customerKey: customerGuestKey(), customer_phone: this.data.customerPhone, couponCampaignId: this.data.selectedCoupon?.id || 0, ...locationFields, fulfillmentType: tableContext ? "DINE_IN" : "PICKUP", ...tableOrderFields(tableContext), ...(fastFoodContext ? { fastFoodPlatePublicId: fastFoodContext.publicId } : {}), remark: orderRemark, items: this.data.cart.map((item) => ({ productId: item.productId, skuId: item.skuId, quantity: item.quantity, optionValueIds: item.optionValueIds || [], modifiers: item.modifiers || [], itemRemark: allowItemRemark ? (item.itemRemark || '') : '' })) },
          });
          rememberCheckoutOrder(flow.idempotencyKey, order.orderNo);
          this.setData({ checkoutKey: flow.idempotencyKey, orderNo: order.orderNo });
        }
      } else {
        order = await request<Order>({
          url: `/public/stores/${encodeURIComponent(storeCode)}/orders`, method: "POST",
          header: { "Idempotency-Key": flow.idempotencyKey },
          data: { customerKey: customerGuestKey(), customer_phone: this.data.customerPhone, couponCampaignId: this.data.selectedCoupon?.id || 0, ...locationFields, fulfillmentType: tableContext ? "DINE_IN" : "PICKUP", ...tableOrderFields(tableContext), ...(fastFoodContext ? { fastFoodPlatePublicId: fastFoodContext.publicId } : {}), remark: orderRemark, items: this.data.cart.map((item) => ({ productId: item.productId, skuId: item.skuId, quantity: item.quantity, optionValueIds: item.optionValueIds || [], modifiers: item.modifiers || [], itemRemark: allowItemRemark ? (item.itemRemark || '') : '' })) },
        });
        rememberCheckoutOrder(flow.idempotencyKey, order.orderNo);
        this.setData({ checkoutKey: flow.idempotencyKey, orderNo: order.orderNo });
      }
      this.setData({
        fulfillmentType: tableContext || order.fulfillmentType === "DINE_IN" ? "DINE_IN" : "PICKUP",
        remark: order.remark || "",
      });
      rememberOrder(storeCode, order.orderNo);
      if (Number(order.amount) !== Number(this.data.amount)) {
        const previousAmount = this.data.amount;
        this.setData({ amount: order.amount, discountAmount: Math.max(0, this.data.subtotalAmount - order.amount) });
        const confirmed = await new Promise<boolean>((resolve) => wx.showModal({
          title: "订单金额已更新",
          content: `商品价格或优惠已重新核算，最新金额为 ¥${(order.amount / 100).toFixed(2)}（原预计 ¥${(previousAmount / 100).toFixed(2)}），请确认后继续支付。`,
          confirmText: "确认支付",
          cancelText: "暂不支付",
          success: (result) => resolve(result.confirm),
          fail: () => resolve(false),
        }));
        if (!confirmed) return;
      }
      if (order.paymentStatus === "SUCCEEDED") {
        if (this.data.selectedCoupon) forgetClaimedCoupon(storeCode, this.data.selectedCoupon.id);
        clearCart(storeCode);
        clearCheckoutFlow(flow.idempotencyKey);
        wx.redirectTo({ url: `/pages/order-detail/index?orderNo=${encodeURIComponent(order.orderNo)}` });
        return;
      }
      const payment = await request<PaymentResult>({ url: `/public/orders/${order.orderNo}/payments`, method: "POST", data: { provider: env.paymentMode } });
      if (payment.provider === "mock") {
        await request({ url: `/public/payments/${payment.id}/mock-confirm`, method: "POST" });
      } else if (payment.wxPayParams) {
        await new Promise<void>((resolve, reject) => wx.requestPayment({ ...payment.wxPayParams!, success: () => resolve(), fail: reject }));
      } else {
        throw new Error("支付参数缺失，请稍后重试");
      }
      clearCart(storeCode);
      if (this.data.selectedCoupon) forgetClaimedCoupon(storeCode, this.data.selectedCoupon.id);
      clearCheckoutFlow(flow.idempotencyKey);
      wx.redirectTo({ url: `/pages/order-detail/index?orderNo=${encodeURIComponent(order.orderNo)}` });
    } catch (error) {
      const orderNoLongerPayable = checkoutNeedsFreshOrder(error);
      const storeClosed = error instanceof ApiError && error.code === "STORE_CLOSED";
      if ((orderNoLongerPayable || storeClosed) && this.data.checkoutKey) {
        clearCheckoutFlow(this.data.checkoutKey);
        this.setData({ checkoutKey: "", orderNo: "", detailsLocked: false });
      }
      const invalidCart = error instanceof ApiError && ["ITEM_UNAVAILABLE", "INVALID_CONFIGURATION", "INVALID_ITEM"].includes(error.code || "");
      if (invalidCart) {
        wx.showModal({
          title: "购物车需要更新",
          content: "商品可能已下架、售罄或修改了选项。可清空购物车后重新选择。",
          confirmText: "清空并重选",
          cancelText: "暂不处理",
          success: (result) => {
            if (!result.confirm) return;
            clearCart(this.data.storeCode);
            if (this.data.checkoutKey) clearCheckoutFlow(this.data.checkoutKey);
            wx.switchTab({ url: "/pages/menu/index" });
          },
        });
      } else {
        wx.showModal({
          title: "下单未完成",
          content: orderNoLongerPayable ? "原订单已关闭，请重新提交订单" : customerSafeErrorMessage(error),
          showCancel: false,
        });
      }
    } finally { this.setData({ submitting: false }); }
  },
  clearAllCart() {
    wx.showModal({
      title: "清空购物车？",
      content: "已选择的规格、口味和加料都会被移除。",
      confirmText: "清空",
      confirmColor: "#c6564e",
      success: (result) => {
        if (!result.confirm) return;
        clearCart(this.data.storeCode);
        if (this.data.checkoutKey) clearCheckoutFlow(this.data.checkoutKey);
        wx.switchTab({ url: "/pages/menu/index" });
      },
    });
  },
});
