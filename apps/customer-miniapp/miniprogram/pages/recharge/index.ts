import type { TanbanAppOption } from "../../app";
import { request } from "../../utils/request";

interface StoredValueRule {
  id: number;
  name: string;
  rechargeCents: number;
  giftCents: number;
  rechargeText?: string;
  giftText?: string;
}

interface StoredValueView {
  available: boolean;
  message: string;
  rules: StoredValueRule[];
  settings: { minRechargeCents: number; maxRechargeCents: number; maxBalanceCents: number; agreementUrl?: string };
}

Page({
  data: { loading: true, available: false, message: "", rules: [] as StoredValueRule[], selectedRuleId: 0 },
  onShow() { void this.loadRules(); },
  onPullDownRefresh() { this.loadRules().finally(() => wx.stopPullDownRefresh()); },
  async loadRules() {
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      const result = await request<StoredValueView>({ url: `/public/stores/${encodeURIComponent(storeCode)}/stored-value`, method: "GET" });
      const rules = (result.rules || []).map((item) => ({ ...item, rechargeText: (item.rechargeCents / 100).toFixed(0), giftText: (item.giftCents / 100).toFixed(0) }));
      this.setData({ loading: false, available: result.available, message: result.message, rules, selectedRuleId: rules[0]?.id || 0 });
    } catch (error) {
      this.setData({ loading: false, available: false, message: error instanceof Error ? error.message : "充值能力加载失败", rules: [] });
    }
  },
  chooseRule(event: WechatMiniprogram.BaseEvent) { this.setData({ selectedRuleId: Number(event.currentTarget.dataset.id) }); },
  recharge() {
    wx.showModal({ title: "储值支付暂未开放", content: "储值涉及真实资金。待顾客登录、支付通道、资金账本和退款对账闭环完成后再开放。", showCancel: false });
  },
});
