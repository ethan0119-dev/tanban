import type { TanbanAppOption } from "../../app";
import { request } from "../../utils/request";
import { idempotencyKey } from "../../utils/request";
import { loadPageAppearance } from "../../utils/page-appearance";
import { customerFeatureCopy, customerSafeErrorMessage } from "../../utils/availability";
import { completeCustomerAccountPayment, type CustomerAccountPayment } from "../../utils/customer-payment";

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
  balance?: { balanceCents: number; principalCents: number; bonusCents: number };
  settings: { minRechargeCents: number; maxRechargeCents: number; maxBalanceCents: number; agreementUrl?: string };
}

Page({
  data: { loading: true, recharging: false, available: false, message: "", rules: [] as StoredValueRule[], selectedRuleId: 0, balanceCents: 0, principalCents: 0, bonusCents: 0, appearanceStyle: "" },
  onShow() { void this.loadRules(); },
  onPullDownRefresh() { this.loadRules().finally(() => wx.stopPullDownRefresh()); },
  async loadRules() {
    const appearance = await loadPageAppearance();
    this.setData({ appearanceStyle: appearance.appearanceStyle });
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      const result = await request<StoredValueView>({ url: `/public/stores/${encodeURIComponent(storeCode)}/stored-value`, method: "GET" });
      const rules = (result.rules || []).map((item) => ({ ...item, rechargeText: (item.rechargeCents / 100).toFixed(0), giftText: (item.giftCents / 100).toFixed(0) }));
      this.setData({
        loading: false,
        available: result.available,
        message: result.available ? "请选择充值金额" : customerFeatureCopy.STORED_VALUE.content,
        rules,
        selectedRuleId: rules[0]?.id || 0,
        balanceCents: result.balance?.balanceCents || 0,
        principalCents: result.balance?.principalCents || 0,
        bonusCents: result.balance?.bonusCents || 0,
      });
    } catch (error) {
      this.setData({ loading: false, available: false, message: customerSafeErrorMessage(error, "储值服务暂时无法加载。"), rules: [] });
    }
  },
  chooseRule(event: WechatMiniprogram.BaseEvent) { this.setData({ selectedRuleId: Number(event.currentTarget.dataset.id) }); },
  async recharge() {
    if (this.data.recharging || !this.data.selectedRuleId) return;
    const rule = this.data.rules.find((item) => item.id === this.data.selectedRuleId);
    if (!rule) return;
    const confirmed = await new Promise<boolean>((resolve) => wx.showModal({
      title: "确认充值",
      content: `支付 ¥${(rule.rechargeCents / 100).toFixed(2)}${rule.giftCents ? `，到账 ¥${((rule.rechargeCents + rule.giftCents) / 100).toFixed(2)}` : ""}`,
      confirmText: "确认支付",
      success: (result) => resolve(result.confirm),
      fail: () => resolve(false),
    }));
    if (!confirmed) return;
    this.setData({ recharging: true });
    try {
      const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
      const payment = await request<CustomerAccountPayment>({
        url: `/public/stores/${encodeURIComponent(storeCode)}/stored-value/orders`,
        method: "POST",
        header: {
          "Idempotency-Key": idempotencyKey("recharge"),
        },
        data: { ruleId: rule.id },
      });
      await completeCustomerAccountPayment(payment);
      await this.loadRules();
      wx.showToast({ title: "充值成功", icon: "success" });
    } catch (error) {
      wx.showModal({ title: "充值未完成", content: customerSafeErrorMessage(error), showCancel: false });
    } finally {
      this.setData({ recharging: false });
    }
  },
});
