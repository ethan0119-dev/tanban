import type { TanbanAppOption } from "../../app";
import type { Order } from "../../types/domain";
import { localOrderNumbers, sortCustomerOrders } from "../../utils/orders";
import { request } from "../../utils/request";
import { loadPageAppearance } from "../../utils/page-appearance";
import { formatBeijingDateTime } from "../../utils/datetime";
import { showUnavailableFeature } from "../../utils/availability";

type SceneTab = "ALL" | "DELIVERY" | "DINE_IN" | "TAKEOUT" | "COUNTER" | "QUEUE" | "RESERVATION";

interface OrderView extends Order {
  scene: SceneTab;
  sceneText: string;
  statusText: string;
  dateText: string;
  summaryText: string;
  current: boolean;
  amountText: string;
}

const currentStatuses = new Set(["PENDING_PAYMENT", "PAID", "ACCEPTED", "PREPARING", "READY"]);
const statusText: Record<string, string> = {
  PENDING_PAYMENT: "待付款", PAID: "已付款", ACCEPTED: "商家已接单", PREPARING: "制作中", READY: "请取餐",
  COMPLETED: "已完成", CLOSED: "已关闭", CANCELED: "已取消", CANCELLED: "已取消", REFUNDED: "已退款", PARTIALLY_REFUNDED: "部分退款",
};

function decorate(order: Order): OrderView {
  const rawScene = order.orderScene || order.order_scene || (order.fulfillmentType === "DINE_IN" ? "DINE_IN" : "TAKEOUT");
  const scene: SceneTab = rawScene === "DINE_IN" ? "DINE_IN" : "TAKEOUT";
  const summary = (order.items || []).slice(0, 3).map((item) => `${item.name} ×${item.quantity}`).join(" · ");
  return {
    ...order,
    scene,
    sceneText: scene === "DINE_IN" ? "堂食订单" : "门店自取",
    statusText: statusText[String(order.status).toUpperCase()] || "状态更新中",
    dateText: formatBeijingDateTime(order.createdAt),
    summaryText: summary || `取餐号 ${order.pickupCode || "--"}`,
    current: currentStatuses.has(String(order.status).toUpperCase()),
    amountText: (Number(order.amount || 0) / 100).toFixed(2),
  };
}

Page({
  data: {
    allOrders: [] as OrderView[],
    orders: [] as OrderView[],
    loading: true,
    sceneTab: "ALL" as SceneTab,
    sceneTabs: [
      { key: "ALL", text: "全部" }, { key: "DELIVERY", text: "外卖" }, { key: "DINE_IN", text: "堂食" },
      { key: "TAKEOUT", text: "快餐" }, { key: "COUNTER", text: "当面付" }, { key: "QUEUE", text: "排队" }, { key: "RESERVATION", text: "预约" },
    ],
    appearanceStyle: "",
  },
  async onShow() {
    const appearance = await loadPageAppearance();
    this.setData({ appearanceStyle: appearance.appearanceStyle });
    await this.loadOrders();
  },
  onPullDownRefresh() { this.loadOrders().finally(() => wx.stopPullDownRefresh()); },
  async loadOrders() {
    try {
      const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
      let sourceOrders: Order[] = [];
      try {
        sourceOrders = await request<Order[]>({
          url: `/public/customer/orders?storeCode=${encodeURIComponent(storeCode)}`,
          method: "GET",
        });
      } catch {
        const numbers = localOrderNumbers(storeCode);
        const results = await Promise.allSettled(numbers.map((orderNo) => request<Order>({ url: `/public/orders/${encodeURIComponent(orderNo)}`, method: "GET" })));
        sourceOrders = results.flatMap((result) => result.status === "fulfilled" ? [result.value] : []);
      }
      const allOrders = sortCustomerOrders(sourceOrders.map(decorate));
      this.setData({ allOrders, loading: false });
      this.applyFilters();
    } catch { this.setData({ allOrders: [], orders: [], loading: false }); }
  },
  chooseScene(event: WechatMiniprogram.BaseEvent) {
    const sceneTab = String(event.currentTarget.dataset.tab) as SceneTab;
    if (sceneTab === "DELIVERY") {
      showUnavailableFeature("DELIVERY");
      return;
    }
    if (["COUNTER", "QUEUE", "RESERVATION"].includes(sceneTab)) {
      const featureName = sceneTab === "COUNTER" ? "当面付" : sceneTab === "QUEUE" ? "排队取号" : "预约服务";
      showUnavailableFeature("PROFILE_SERVICE", featureName);
      return;
    }
    this.setData({ sceneTab });
    this.applyFilters();
  },
  applyFilters() {
    const orders = this.data.allOrders.filter((item) => this.data.sceneTab === "ALL" || item.scene === this.data.sceneTab);
    this.setData({ orders });
  },
  openOrder(event: WechatMiniprogram.BaseEvent) { wx.navigateTo({ url: `/pages/order-detail/index?orderNo=${encodeURIComponent(String(event.currentTarget.dataset.no))}` }); },
  goMenu() { wx.switchTab({ url: "/pages/menu/index" }); },
});
