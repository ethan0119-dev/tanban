import type { TanbanAppOption } from "../../app";
import { customerSafeErrorMessage } from "../../utils/availability";
import { completeCustomerAccountPayment, type CustomerAccountPayment } from "../../utils/customer-payment";
import { idempotencyKey, request } from "../../utils/request";
import { loadPageAppearance } from "../../utils/page-appearance";

interface MembershipLevel {
  id: number;
  name: string;
  acquireType: string;
  rechargeCents: number;
  validDays: number;
  discountPercent: number;
  isDefault?: boolean;
  rechargeText?: string;
  discountText?: string;
  validityText?: string;
  current?: boolean;
  purchasable?: boolean;
}

interface PublicMembership {
  available: boolean;
  card?: { name: string; color: string; imageUrl?: string; agreementUrl?: string; showBalance: boolean };
  member?: { memberId: number; memberNo: string; levelId: number; levelName: string; principalCents: number; bonusCents: number; balanceCents: number };
  levels: MembershipLevel[];
}

Page({
  data: {
    loading: true,
    purchasing: false,
    membership: null as PublicMembership | null,
    levels: [] as MembershipLevel[],
    appearanceStyle: "",
  },
  onShow() { void this.loadMembership(); },
  onPullDownRefresh() { this.loadMembership().finally(() => wx.stopPullDownRefresh()); },
  async loadMembership() {
    try {
      const appearance = await loadPageAppearance();
      const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
      const membership = await request<PublicMembership>({
        url: `/public/stores/${encodeURIComponent(storeCode)}/membership`,
        method: "GET",
      });
      const levels = (membership.levels || []).map((level) => ({
        ...level,
        rechargeText: level.acquireType === "GROWTH" ? "成长值升级" : (level.rechargeCents ? `充值 ¥${(level.rechargeCents / 100).toFixed(2)}` : "免费开通"),
        discountText: level.discountPercent < 100
          ? `${(level.discountPercent / 10).toFixed(level.discountPercent % 10 ? 1 : 0)} 折`
          : "会员身份",
        validityText: level.validDays > 0 ? `有效期 ${level.validDays} 天` : "长期有效",
        current: membership.member?.levelId === level.id,
        purchasable: level.acquireType !== "GROWTH",
      }));
      this.setData({ loading: false, membership, levels, appearanceStyle: appearance.appearanceStyle });
    } catch (error) {
      this.setData({ loading: false });
      wx.showToast({ title: customerSafeErrorMessage(error, "会员信息暂时无法加载。"), icon: "none" });
    }
  },
  chooseLevel(event: WechatMiniprogram.BaseEvent) {
    if (this.data.purchasing) return;
    const level = this.data.levels.find((item) => item.id === Number(event.currentTarget.dataset.id));
    if (!level || level.current || !level.purchasable) {
      if (level?.acquireType === "GROWTH") wx.showToast({ title: "该等级需达到成长值后自动升级", icon: "none" });
      return;
    }
    const priceText = level.rechargeCents ? `支付 ¥${(level.rechargeCents / 100).toFixed(2)}` : "免费开通";
    wx.showModal({
      title: `开通${level.name}`,
      content: `${priceText}，金额将全部进入账户余额；本等级享受${level.discountText}，${level.validityText}。`,
      confirmText: level.rechargeCents ? "确认支付" : "立即开通",
      success: (result) => {
        if (result.confirm) void this.purchaseLevel(level);
      },
    });
  },
  async purchaseLevel(level: MembershipLevel) {
    this.setData({ purchasing: true });
    try {
      const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
      const payment = await request<CustomerAccountPayment>({
        url: `/public/stores/${encodeURIComponent(storeCode)}/membership/orders`,
        method: "POST",
        header: {
          "Idempotency-Key": idempotencyKey("member"),
        },
        data: { levelId: level.id },
      });
      await completeCustomerAccountPayment(payment);
      await this.loadMembership();
      wx.showToast({ title: "会员等级已生效", icon: "success" });
    } catch (error) {
      wx.showModal({ title: "开通未完成", content: customerSafeErrorMessage(error), showCancel: false });
    } finally {
      this.setData({ purchasing: false });
    }
  },
  goRecharge() { wx.navigateTo({ url: "/pages/recharge/index" }); },
});
