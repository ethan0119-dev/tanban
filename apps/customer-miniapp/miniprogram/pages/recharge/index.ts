import type { TanbanAppOption } from "../../app";
import { request } from "../../utils/request";
import { loadPageAppearance } from "../../utils/page-appearance";
import { customerFeatureCopy, customerSafeErrorMessage, showUnavailableFeature } from "../../utils/availability";

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
  data: { loading: true, available: false, message: "", rules: [] as StoredValueRule[], selectedRuleId: 0, appearanceStyle: "" },
  onShow() { void this.loadRules(); },
  onPullDownRefresh() { this.loadRules().finally(() => wx.stopPullDownRefresh()); },
  async loadRules() {
    const appearance = await loadPageAppearance();
    this.setData({ appearanceStyle: appearance.appearanceStyle });
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      const result = await request<StoredValueView>({ url: `/public/stores/${encodeURIComponent(storeCode)}/stored-value`, method: "GET" });
      const rules = (result.rules || []).map((item) => ({ ...item, rechargeText: (item.rechargeCents / 100).toFixed(0), giftText: (item.giftCents / 100).toFixed(0) }));
      this.setData({ loading: false, available: result.available, message: result.available ? "请选择充值金额" : customerFeatureCopy.STORED_VALUE.content, rules, selectedRuleId: rules[0]?.id || 0 });
    } catch (error) {
      this.setData({ loading: false, available: false, message: customerSafeErrorMessage(error, "储值服务暂时无法加载。"), rules: [] });
    }
  },
  chooseRule(event: WechatMiniprogram.BaseEvent) { this.setData({ selectedRuleId: Number(event.currentTarget.dataset.id) }); },
  recharge() {
    showUnavailableFeature("STORED_VALUE");
  },
});
