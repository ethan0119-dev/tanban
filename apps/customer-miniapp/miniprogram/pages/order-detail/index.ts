import type { TanbanAppOption } from "../../app";
import type { Order } from "../../types/domain";
import { request } from "../../utils/request";
import { loadPageAppearance } from "../../utils/page-appearance";

interface OrderView extends Order {
  isDineIn: boolean;
  paymentSucceeded: boolean;
  paymentPending: boolean;
  statusTitle: string;
  statusMessage: string;
  displayTableName: string;
  displayTableCode: string;
  displayTableArea: string;
}

function decorateOrder(order: Order): OrderView {
  const paymentStatus = String(order.paymentStatus || "").toUpperCase();
  const paymentSucceeded = paymentStatus === "SUCCEEDED" || paymentStatus === "PAID";
  const paymentPending = !paymentSucceeded
    && ["", "UNPAID", "PENDING", "CREATED", "PROCESSING"].includes(paymentStatus)
    && order.status === "PENDING_PAYMENT";
  return {
    ...order,
    isDineIn: order.orderScene === "DINE_IN" || order.order_scene === "DINE_IN" || Boolean(order.tablePublicId || order.table?.publicId),
    paymentSucceeded,
    paymentPending,
    statusTitle: paymentSucceeded ? "支付成功" : paymentPending ? "正在确认支付结果" : order.status === "CLOSED" ? "订单已关闭" : "支付尚未成功",
    statusMessage: paymentSucceeded
      ? "商家已收到订单，请留意制作进度"
      : paymentPending ? "请勿重复付款，页面会自动刷新支付结果" : "商家尚未确认收款，请返回订单后重试或联系商家",
    displayTableName: order.tableName || order.table?.name || order.tableCode || order.table?.tableCode || "当前桌台",
    displayTableCode: order.tableCode || order.table?.tableCode || "",
    displayTableArea: order.tableAreaName || order.table?.areaName || "",
  };
}

let confirmationTimer: ReturnType<typeof setTimeout> | undefined;

Page({
  data: { order: null as OrderView | null, loading: true, orderNo: "", storeCode: "", confirmationAttempts: 0, appearanceStyle: "" },
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
    catch (error) { this.setData({ loading: false }); wx.showToast({ title: error instanceof Error ? error.message : "加载失败", icon: "none" }); }
  },
  onUnload() {
    if (confirmationTimer) clearTimeout(confirmationTimer);
    confirmationTimer = undefined;
  },
  backToMenu() { wx.switchTab({ url: "/pages/menu/index" }); },
});
