import { env } from "../../config/env";
import type { TanbanAppOption } from "../../app";
import type { CartItem, Order } from "../../types/domain";
import { cartLineKey, clearCart, readCart } from "../../utils/cart";
import { checkoutFlowFor, checkoutNeedsFreshOrder, checkoutOrderIsClosed, clearCheckoutFlow, markCheckoutSubmitted, rememberCheckoutDetails, rememberCheckoutOrder } from "../../utils/checkout";
import { customerGuestKey } from "../../utils/customer";
import { rememberOrder } from "../../utils/orders";
import { ApiError, request } from "../../utils/request";

interface PaymentResult { id: number; provider: string; status: string; wxPayParams?: WechatMiniprogram.RequestPaymentOption; }
interface TextInputEvent extends WechatMiniprogram.BaseEvent { detail: { value: string } }

Page({
  data: { storeCode: "", cart: [] as CartItem[], amount: 0, remark: "", fulfillmentType: "PICKUP" as "PICKUP" | "DINE_IN", detailsLocked: false, submitting: false, checkoutKey: "", orderNo: "", paymentMode: env.paymentMode },
  onLoad() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    const cart = readCart(storeCode);
    if (!cart.length) {
      void wx.navigateBack();
      return;
    }
    const flow = checkoutFlowFor(storeCode, cart);
    this.setData({
      storeCode,
      cart: cart.map((item) => ({ ...item, lineKey: cartLineKey(item) })),
      amount: cart.reduce((sum, item) => sum + item.price * item.quantity, 0),
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
        const fresh = checkoutFlowFor(this.data.storeCode, this.data.cart);
        this.setData({ checkoutKey: fresh.idempotencyKey, orderNo: "", fulfillmentType: fresh.fulfillmentType, remark: fresh.remark, detailsLocked: false });
        return;
      }
      this.setData({
        amount: order.amount,
        fulfillmentType: order.fulfillmentType === "DINE_IN" ? "DINE_IN" : "PICKUP",
        remark: order.remark || "",
        detailsLocked: true,
      });
    } catch (error) {
      wx.showToast({ title: error instanceof Error ? error.message : "订单恢复失败", icon: "none" });
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
  chooseFulfillment(event: WechatMiniprogram.BaseEvent) {
    if (this.data.detailsLocked) {
      wx.showToast({ title: "订单已生成，取餐信息不能修改", icon: "none" });
      return;
    }
    const fulfillmentType = String(event.currentTarget.dataset.type) === "DINE_IN" ? "DINE_IN" : "PICKUP";
    this.setData({ fulfillmentType });
    rememberCheckoutDetails(this.data.checkoutKey, fulfillmentType, this.data.remark);
  },
  async submitOrder() {
    if (this.data.submitting) return;
    this.setData({ submitting: true });
    try {
      const storeCode = this.data.storeCode;
      // 每次提交都重新经过持久化 flow 校验，这样页面长时间停留时 TTL 也会生效。
      let flow = checkoutFlowFor(storeCode, this.data.cart);
      if (flow.idempotencyKey !== this.data.checkoutKey || flow.orderNo !== this.data.orderNo) {
        this.setData({ checkoutKey: flow.idempotencyKey, orderNo: flow.orderNo });
      }
      if (!flow.submitted) {
        rememberCheckoutDetails(flow.idempotencyKey, this.data.fulfillmentType, this.data.remark);
        markCheckoutSubmitted(flow.idempotencyKey);
        flow = checkoutFlowFor(storeCode, this.data.cart);
        this.setData({ detailsLocked: true });
      }
      let order: Order;
      if (flow.orderNo) {
        order = await request<Order>({ url: `/public/orders/${encodeURIComponent(flow.orderNo)}`, method: "GET" });
        if (checkoutOrderIsClosed(order.status)) {
          clearCheckoutFlow(flow.idempotencyKey);
          flow = checkoutFlowFor(storeCode, this.data.cart);
          rememberCheckoutDetails(flow.idempotencyKey, this.data.fulfillmentType, this.data.remark);
          markCheckoutSubmitted(flow.idempotencyKey);
          flow = checkoutFlowFor(storeCode, this.data.cart);
          this.setData({ checkoutKey: flow.idempotencyKey, orderNo: "", detailsLocked: true });
          order = await request<Order>({
            url: `/public/stores/${encodeURIComponent(storeCode)}/orders`, method: "POST",
            header: { "Idempotency-Key": flow.idempotencyKey },
            data: { customerKey: customerGuestKey(), fulfillmentType: this.data.fulfillmentType, remark: this.data.remark, items: this.data.cart.map((item) => ({ productId: item.productId, skuId: item.skuId, quantity: item.quantity, optionValueIds: item.optionValueIds || [], modifiers: item.modifiers || [], itemRemark: item.itemRemark || '' })) },
          });
          rememberCheckoutOrder(flow.idempotencyKey, order.orderNo);
          this.setData({ checkoutKey: flow.idempotencyKey, orderNo: order.orderNo });
        }
      } else {
        order = await request<Order>({
          url: `/public/stores/${encodeURIComponent(storeCode)}/orders`, method: "POST",
          header: { "Idempotency-Key": flow.idempotencyKey },
          data: { customerKey: customerGuestKey(), fulfillmentType: this.data.fulfillmentType, remark: this.data.remark, items: this.data.cart.map((item) => ({ productId: item.productId, skuId: item.skuId, quantity: item.quantity, optionValueIds: item.optionValueIds || [], modifiers: item.modifiers || [], itemRemark: item.itemRemark || '' })) },
        });
        rememberCheckoutOrder(flow.idempotencyKey, order.orderNo);
        this.setData({ checkoutKey: flow.idempotencyKey, orderNo: order.orderNo });
      }
      this.setData({
        fulfillmentType: order.fulfillmentType === "DINE_IN" ? "DINE_IN" : "PICKUP",
        remark: order.remark || "",
      });
      rememberOrder(storeCode, order.orderNo);
      if (Number(order.amount) !== Number(this.data.amount)) {
        const previousAmount = this.data.amount;
        this.setData({ amount: order.amount });
        const confirmed = await new Promise<boolean>((resolve) => wx.showModal({
          title: "商品价格已更新",
          content: `订单最新金额为 ¥${(order.amount / 100).toFixed(2)}（原预计 ¥${(previousAmount / 100).toFixed(2)}），请确认后继续支付。`,
          confirmText: "确认支付",
          cancelText: "暂不支付",
          success: (result) => resolve(result.confirm),
          fail: () => resolve(false),
        }));
        if (!confirmed) return;
      }
      if (order.paymentStatus === "SUCCEEDED") {
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
      clearCheckoutFlow(flow.idempotencyKey);
      wx.redirectTo({ url: `/pages/order-detail/index?orderNo=${encodeURIComponent(order.orderNo)}` });
    } catch (error) {
      const orderNoLongerPayable = checkoutNeedsFreshOrder(error);
      if (orderNoLongerPayable && this.data.checkoutKey) {
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
          content: orderNoLongerPayable ? "原订单已关闭，请重新提交订单" : error instanceof Error ? error.message : "请稍后重试",
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
