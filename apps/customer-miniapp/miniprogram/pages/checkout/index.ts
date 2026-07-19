import { env } from "../../config/env";
import type { TanbanAppOption } from "../../app";
import type { CartItem, Order } from "../../types/domain";
import { clearCart, readCart } from "../../utils/cart";
import { checkoutFlowFor, checkoutNeedsFreshOrder, checkoutOrderIsClosed, clearCheckoutFlow, rememberCheckoutOrder } from "../../utils/checkout";
import { rememberOrder } from "../../utils/orders";
import { request } from "../../utils/request";

interface PaymentResult { id: number; provider: string; status: string; wxPayParams?: WechatMiniprogram.RequestPaymentOption; }
interface TextInputEvent extends WechatMiniprogram.BaseEvent { detail: { value: string } }

Page({
  data: { storeCode: "", cart: [] as CartItem[], amount: 0, remark: "", fulfillmentType: "PICKUP", submitting: false, checkoutKey: "", orderNo: "", paymentMode: env.paymentMode },
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
      cart: cart.map((item) => ({ ...item, lineKey: `${item.productId}:${item.skuId ?? 0}` })),
      amount: cart.reduce((sum, item) => sum + item.price * item.quantity, 0),
      checkoutKey: flow.idempotencyKey,
      orderNo: flow.orderNo,
    });
  },
  setRemark(event: TextInputEvent) { this.setData({ remark: event.detail.value }); },
  chooseFulfillment(event: WechatMiniprogram.BaseEvent) { this.setData({ fulfillmentType: String(event.currentTarget.dataset.type) }); },
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
      let order: Order;
      if (flow.orderNo) {
        order = await request<Order>({ url: `/public/orders/${encodeURIComponent(flow.orderNo)}`, method: "GET" });
        if (checkoutOrderIsClosed(order.status)) {
          clearCheckoutFlow(flow.idempotencyKey);
          flow = checkoutFlowFor(storeCode, this.data.cart);
          this.setData({ checkoutKey: flow.idempotencyKey, orderNo: "" });
          order = await request<Order>({
            url: `/public/stores/${encodeURIComponent(storeCode)}/orders`, method: "POST",
            header: { "Idempotency-Key": flow.idempotencyKey },
            data: { fulfillmentType: this.data.fulfillmentType, remark: this.data.remark, items: this.data.cart.map((item) => ({ productId: item.productId, skuId: item.skuId, quantity: item.quantity })) },
          });
          rememberCheckoutOrder(flow.idempotencyKey, order.orderNo);
          this.setData({ checkoutKey: flow.idempotencyKey, orderNo: order.orderNo });
        }
      } else {
        order = await request<Order>({
          url: `/public/stores/${encodeURIComponent(storeCode)}/orders`, method: "POST",
          header: { "Idempotency-Key": flow.idempotencyKey },
          data: { fulfillmentType: this.data.fulfillmentType, remark: this.data.remark, items: this.data.cart.map((item) => ({ productId: item.productId, skuId: item.skuId, quantity: item.quantity })) },
        });
        rememberCheckoutOrder(flow.idempotencyKey, order.orderNo);
        this.setData({ checkoutKey: flow.idempotencyKey, orderNo: order.orderNo });
      }
      rememberOrder(storeCode, order.orderNo);
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
        this.setData({ checkoutKey: "", orderNo: "" });
      }
      wx.showModal({
        title: "下单未完成",
        content: orderNoLongerPayable ? "原订单已关闭，请重新提交订单" : error instanceof Error ? error.message : "请稍后重试",
        showCancel: false,
      });
    } finally { this.setData({ submitting: false }); }
  },
});
