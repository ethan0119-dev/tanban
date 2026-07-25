import type { TanbanAppOption } from "../../app";
import type { Order } from "../../types/domain";
import { request } from "../../utils/request";
import { loadPageAppearance } from "../../utils/page-appearance";
import { customerSafeErrorMessage } from "../../utils/availability";
import { formatBeijingDateTime } from "../../utils/datetime";

interface PaymentResult {
  id: number;
  provider: "balance" | "mock" | "tianque" | "wechat_partner";
  status: string;
  wxPayParams?: WechatMiniprogram.RequestPaymentOption;
}

interface OrderView extends Order {
  isDineIn: boolean;
  payAfterMeal: boolean;
  canPay: boolean;
  paymentSucceeded: boolean;
  paymentPending: boolean;
  statusTitle: string;
  statusMessage: string;
  orderStatusText: string;
  paymentStatusText: string;
  displayTableName: string;
  displayTableCode: string;
  displayTableArea: string;
}

function validWechatPayParams(value?: WechatMiniprogram.RequestPaymentOption): value is WechatMiniprogram.RequestPaymentOption {
  return Boolean(value?.timeStamp && value.nonceStr && value.package && value.signType && value.paySign);
}

function decorateOrder(order: Order): OrderView {
  const paymentStatus = String(order.paymentStatus || "").toUpperCase();
  const orderStatus = String(order.status || "").toUpperCase();
  const paymentSucceeded = paymentStatus === "SUCCEEDED" || paymentStatus === "PAID";
  const payAfterMeal = order.settlementMode === "PAY_AFTER";
  const paidAmount = Math.max(Number(order.paidAmount || 0), 0);
  const remainingAmount = Math.max(Number(order.remainingAmount ?? (order.amount - paidAmount)), 0);
  const canPay = payAfterMeal && !paymentSucceeded && !["CLOSED", "CANCELED", "CANCELLED", "REFUNDED"].includes(orderStatus);
  const paymentPending = !payAfterMeal && !paymentSucceeded
    && ["", "UNPAID", "PENDING", "CREATED", "PROCESSING"].includes(paymentStatus)
    && order.status === "PENDING_PAYMENT";
  return {
    ...order,
    paidAmount,
    remainingAmount,
    createdAt: formatBeijingDateTime(order.createdAt),
    isDineIn: order.orderScene === "DINE_IN" || order.order_scene === "DINE_IN" || Boolean(order.tablePublicId || order.table?.publicId),
    payAfterMeal,
    canPay,
    paymentSucceeded,
    paymentPending,
    statusTitle: paymentSucceeded ? "支付成功" : canPay ? "用餐中 · 待结账" : paymentPending ? "正在确认支付结果" : order.status === "CLOSED" ? "订单已关闭" : "支付尚未成功",
    statusMessage: paymentSucceeded
      ? "商家已收到订单，请留意制作进度"
      : canPay ? (paidAmount > 0 ? "订单已部分结账，请支付剩余金额；部分结账后不能继续加菜" : "订单已提交，可继续加菜；用餐结束后在此完成支付") : paymentPending ? "请勿重复付款，页面会自动刷新支付结果" : "商家尚未确认收款，请返回订单后重试或联系商家",
    orderStatusText: payAfterMeal && !paymentSucceeded && orderStatus === "PAID" ? "已下单" : ({ PENDING_PAYMENT: "待付款", PAID: "已付款", ACCEPTED: "商家已接单", PREPARING: "制作中", READY: "待取餐", COMPLETED: "已完成", CLOSED: "已关闭", CANCELED: "已取消", CANCELLED: "已取消", REFUNDED: "已退款", PARTIALLY_REFUNDED: "部分退款" } as Record<string, string>)[orderStatus] || "状态更新中",
    paymentStatusText: ({ UNPAID: "待付款", PENDING: "确认中", CREATED: "待付款", PROCESSING: "确认中", SUCCEEDED: "支付成功", PAID: "支付成功", FAILED: "支付未完成", CLOSED: "已关闭", REFUNDED: "已退款", PARTIALLY_REFUNDED: "部分退款" } as Record<string, string>)[paymentStatus] || "状态更新中",
    displayTableName: order.tableName || order.table?.name || order.tableCode || order.table?.tableCode || "当前桌台",
    displayTableCode: order.tableCode || order.table?.tableCode || "",
    displayTableArea: order.tableAreaName || order.table?.areaName || "",
  };
}

let confirmationTimer: ReturnType<typeof setTimeout> | undefined;

Page({
  data: { order: null as OrderView | null, loading: true, paying: false, orderNo: "", storeCode: "", confirmationAttempts: 0, appearanceStyle: "" },
  onLoad(options: Record<string, string>) { this.setData({ orderNo: options.orderNo || "" }); },
  async onShow() {
    const appearance = await loadPageAppearance();
    this.setData({ storeCode: getApp<TanbanAppOption>().globalData.storeCode });
    this.setData({ appearanceStyle: appearance.appearanceStyle });
    if (!this.data.orderNo) return;
    this.setData({ confirmationAttempts: 0 });
    void this.loadOrder();
  },
  async loadOrder() {
    if (!this.data.orderNo) return;
    if (confirmationTimer) clearTimeout(confirmationTimer);
    try {
      const order = await request<Order>({ url: `/public/orders/${encodeURIComponent(this.data.orderNo)}`, method: "GET" });
      const decorated = decorateOrder(order);
      this.setData({ order: decorated, loading: false });
      if (decorated.paymentPending && this.data.confirmationAttempts < 10) {
        this.setData({ confirmationAttempts: this.data.confirmationAttempts + 1 });
        confirmationTimer = setTimeout(() => void this.loadOrder(), 1500);
      }
    }
    catch (error) { this.setData({ loading: false }); wx.showToast({ title: customerSafeErrorMessage(error, "订单暂时无法加载，请稍后重试。"), icon: "none" }); }
  },
  onUnload() {
    if (confirmationTimer) clearTimeout(confirmationTimer);
    confirmationTimer = undefined;
  },
  async payOrder() {
    const order = this.data.order;
    if (!order?.canPay || this.data.paying) return;
    this.setData({ paying: true });
    try {
      const payment = await request<PaymentResult>({
        url: `/public/orders/${encodeURIComponent(order.orderNo)}/payments`,
        method: "POST",
        data: {},
      });
      if (payment.provider === "balance") {
        // 服务端已在同一事务中完成余额扣减和订单结账。
      } else if (payment.provider === "mock") {
        await request({ url: `/public/payments/${payment.id}/mock-confirm`, method: "POST" });
      } else if (payment.provider === "wechat_partner" && validWechatPayParams(payment.wxPayParams)) {
        await new Promise<void>((resolve, reject) => wx.requestPayment({ ...payment.wxPayParams!, success: () => resolve(), fail: reject }));
      } else if (payment.provider === "tianque") {
        throw new Error("会生活收银台尚未完成小程序接入");
      } else {
        throw new Error("支付参数缺失，请稍后重试");
      }
      wx.showToast({ title: "支付成功", icon: "success" });
      await this.loadOrder();
    } catch (error) {
      wx.showModal({ title: "支付未完成", content: customerSafeErrorMessage(error), showCancel: false });
    } finally {
      this.setData({ paying: false });
    }
  },
  backToMenu() { wx.switchTab({ url: "/pages/menu/index" }); },
});
