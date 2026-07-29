import type { TanbanAppOption } from "../../app";
import { request } from "../../utils/request";
import { idempotencyKey } from "../../utils/request";
import { loadPageAppearance } from "../../utils/page-appearance";
import { customerFeatureCopy, customerSafeErrorMessage } from "../../utils/availability";
import { completeCustomerAccountPayment, type CustomerAccountPayment } from "../../utils/customer-payment";
import { requestNotificationSubscriptions } from "../../utils/notification-subscriptions";
import { customRechargeAllowed, yuanInputToCents, yuanText } from "../../utils/recharge";

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

interface TextInputEvent extends WechatMiniprogram.BaseEvent { detail: { value: string } }

Page({
  data: {
    loading: true,
    recharging: false,
    available: false,
    message: "",
    rules: [] as StoredValueRule[],
    selectedMode: "RULE" as "RULE" | "CUSTOM",
    selectedRuleId: 0,
    customAmount: "",
    customAmountCents: 0,
    minRechargeCents: 0,
    maxRechargeCents: 0,
    minRechargeText: "",
    maxRechargeText: "",
    customAmountPlaceholder: "",
    canRecharge: false,
    rechargeButtonText: "立即充值",
    balanceCents: 0,
    principalCents: 0,
    bonusCents: 0,
    appearanceStyle: "",
  },
  onShow() { void this.loadRules(); },
  onPullDownRefresh() { this.loadRules().finally(() => wx.stopPullDownRefresh()); },
  async loadRules() {
    const app = getApp<TanbanAppOption>();
    await app.globalData.routeReady;
    const appearance = await loadPageAppearance();
    this.setData({ appearanceStyle: appearance.appearanceStyle });
    const storeCode = app.globalData.storeCode;
    try {
      const result = await request<StoredValueView>({ url: `/public/stores/${encodeURIComponent(storeCode)}/stored-value`, method: "GET" });
      const rules = (result.rules || []).map((item) => ({ ...item, rechargeText: (item.rechargeCents / 100).toFixed(0), giftText: (item.giftCents / 100).toFixed(0) }));
      const selectedRuleId = rules[0]?.id || 0;
      this.setData({
        loading: false,
        available: result.available,
        message: result.available ? "优惠档位可享赠送" : customerFeatureCopy.STORED_VALUE.content,
        rules,
        selectedMode: selectedRuleId ? "RULE" : "CUSTOM",
        selectedRuleId,
        customAmount: "",
        customAmountCents: 0,
        minRechargeCents: result.settings.minRechargeCents,
        maxRechargeCents: result.settings.maxRechargeCents,
        minRechargeText: yuanText(result.settings.minRechargeCents),
        maxRechargeText: yuanText(result.settings.maxRechargeCents),
        customAmountPlaceholder: `${yuanText(result.settings.minRechargeCents)}–${yuanText(result.settings.maxRechargeCents)}`,
        canRecharge: Boolean(selectedRuleId),
        rechargeButtonText: selectedRuleId ? `充值 ¥${yuanText(rules[0].rechargeCents)}` : "请输入充值金额",
        balanceCents: result.balance?.balanceCents || 0,
        principalCents: result.balance?.principalCents || 0,
        bonusCents: result.balance?.bonusCents || 0,
      });
    } catch (error) {
      this.setData({ loading: false, available: false, message: customerSafeErrorMessage(error, "储值服务暂时无法加载。"), rules: [] });
    }
  },
  chooseRule(event: WechatMiniprogram.BaseEvent) {
    const selectedRuleId = Number(event.currentTarget.dataset.id);
    const rule = this.data.rules.find((item) => item.id === selectedRuleId);
    if (!rule) return;
    this.setData({
      selectedMode: "RULE",
      selectedRuleId,
      canRecharge: true,
      rechargeButtonText: `充值 ¥${yuanText(rule.rechargeCents)}`,
    });
  },
  setCustomAmount(event: TextInputEvent) {
    const customAmount = event.detail.value;
    const customAmountCents = yuanInputToCents(customAmount);
    const canRecharge = customRechargeAllowed(customAmountCents, this.data.minRechargeCents, this.data.maxRechargeCents);
    this.setData({
      selectedMode: "CUSTOM",
      selectedRuleId: 0,
      customAmount,
      customAmountCents,
      canRecharge,
      rechargeButtonText: canRecharge ? `充值 ¥${yuanText(customAmountCents)}` : "请输入有效金额",
    });
  },
  async recharge() {
    if (this.data.recharging || !this.data.canRecharge) return;
    const rule = this.data.selectedMode === "RULE"
      ? this.data.rules.find((item) => item.id === this.data.selectedRuleId)
      : undefined;
    const amountCents = rule?.rechargeCents || this.data.customAmountCents;
    if (amountCents < this.data.minRechargeCents || amountCents > this.data.maxRechargeCents) return;
    const confirmed = await new Promise<boolean>((resolve) => wx.showModal({
      title: "确认充值",
      content: rule
        ? `支付 ¥${(rule.rechargeCents / 100).toFixed(2)}${rule.giftCents ? `，到账 ¥${((rule.rechargeCents + rule.giftCents) / 100).toFixed(2)}` : ""}`
        : `支付 ¥${(amountCents / 100).toFixed(2)}，自定义充值不享赠送金额`,
      confirmText: "确认支付",
      success: (result) => resolve(result.confirm),
      fail: () => resolve(false),
    }));
    if (!confirmed) return;
    this.setData({ recharging: true });
    try {
      const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
      const rechargeKey = idempotencyKey("recharge");
      await requestNotificationSubscriptions({
        storeCode,
        scenes: ["RECHARGE_SUCCESS"],
        requestContext: "RECHARGE",
        businessNo: rechargeKey,
      });
      const payment = await request<CustomerAccountPayment>({
        url: `/public/stores/${encodeURIComponent(storeCode)}/stored-value/orders`,
        method: "POST",
        header: {
          "Idempotency-Key": rechargeKey,
        },
        data: { ruleId: rule?.id || 0, amountCents },
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
