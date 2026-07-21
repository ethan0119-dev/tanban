import type { TanbanAppOption } from "../../app";
import type { MarketingCoupon } from "../../types/domain";
import { customerGuestKey } from "../../utils/customer";
import { idempotencyKey, request } from "../../utils/request";
import { tableContextForStore } from "../../utils/table-context";
import { rememberClaimedCoupon } from "../../utils/coupon-wallet";
import { loadPageAppearance } from "../../utils/page-appearance";

interface CouponView extends MarketingCoupon {
  amountText: string;
  thresholdText: string;
  stockText: string;
  validityText: string;
  scopeText: string;
  remainingStock: number;
  claiming?: boolean;
}

function localDate(value?: string): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}

function viewOf(coupon: MarketingCoupon): CouponView {
  const remaining = Math.max(0, Number(coupon.remaining_stock ?? (Number(coupon.total_stock || 0) - Number(coupon.issued_count || 0))));
  const scope = new Set(coupon.order_types || []);
  const scopeText = scope.has("DINE_IN") && scope.has("TAKEOUT")
    ? "堂食 / 快餐自取可用"
    : scope.has("DINE_IN") ? "仅堂食可用" : scope.has("TAKEOUT") ? "仅快餐自取可用" : "按活动规则使用";
  return {
    ...coupon,
    amountText: (Number(coupon.discount_cents || 0) / 100).toFixed(Number(coupon.discount_cents || 0) % 100 ? 2 : 0),
    thresholdText: coupon.coupon_type === "CASH" || !coupon.threshold_cents ? "无门槛" : `满 ¥${(coupon.threshold_cents / 100).toFixed(2)} 可用`,
    stockText: remaining > 0 ? `剩余 ${remaining} 张` : "已领完",
    validityText: coupon.valid_days ? `领取后 ${coupon.valid_days} 天内有效` : [localDate(coupon.valid_from), localDate(coupon.valid_to)].filter(Boolean).join(" 至 ") || "按活动规则有效",
    scopeText,
    remainingStock: remaining,
  };
}

Page({
  data: { loading: true, coupons: [] as CouponView[], error: "", appearanceStyle: "" },
  onLoad() { void this.loadCoupons(); },
  onPullDownRefresh() { this.loadCoupons().finally(() => wx.stopPullDownRefresh()); },
  async loadCoupons() {
    const appearance = await loadPageAppearance();
    this.setData({ appearanceStyle: appearance.appearanceStyle });
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    const channel = tableContextForStore(storeCode) ? "DINE_IN" : "TAKEOUT";
    this.setData({ loading: true, error: "" });
    try {
      const response = await request<MarketingCoupon[] | { items?: MarketingCoupon[] }>({ url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/coupons?channel_scope=${channel}`, method: "GET" });
      const coupons = Array.isArray(response) ? response : response.items || [];
      this.setData({ coupons: (coupons || []).map(viewOf), loading: false });
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : "优惠券加载失败" });
    }
  },
  async claim(event: WechatMiniprogram.BaseEvent) {
    const id = Number(event.currentTarget.dataset.id);
    const coupon = this.data.coupons.find((item) => item.id === id);
    if (!coupon || coupon.claiming || coupon.remainingStock <= 0) return;
    this.setData({ coupons: this.data.coupons.map((item) => item.id === id ? { ...item, claiming: true } : item) });
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      const result = await request<{ warning?: string; asset_status?: string }>({
        url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/coupons/${id}/claim`, method: "POST",
        header: { "Idempotency-Key": idempotencyKey("coupon") },
        data: { subject_key: customerGuestKey() },
      });
      rememberClaimedCoupon(storeCode, coupon);
      wx.showModal({ title: "领取已记录", content: result.warning || "当前仅生成联调领取记录，暂不能抵扣真实订单。", showCancel: false });
      await this.loadCoupons();
    } catch (error) {
      wx.showToast({ title: error instanceof Error ? error.message : "领取失败", icon: "none" });
      this.setData({ coupons: this.data.coupons.map((item) => item.id === id ? { ...item, claiming: false } : item) });
    }
  },
  goMenu() { wx.switchTab({ url: "/pages/menu/index" }); },
});
