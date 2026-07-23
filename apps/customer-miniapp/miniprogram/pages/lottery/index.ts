import type { TanbanAppOption } from "../../app";
import type { MarketingCoupon, MarketingLottery, MarketingLotteryPrize } from "../../types/domain";
import { customerGuestKey } from "../../utils/customer";
import { idempotencyKey, request } from "../../utils/request";
import { tableContextForStore } from "../../utils/table-context";
import { loadPageAppearance } from "../../utils/page-appearance";
import { customerSafeErrorMessage } from "../../utils/availability";
import { rememberClaimedCoupon } from "../../utils/coupon-wallet";

interface LotteryPrizeView extends MarketingLotteryPrize {
  stockText: string;
}

interface LotteryView extends MarketingLottery {
  prizes: LotteryPrizeView[];
}

interface DrawResult {
  draw_id?: number;
  prize?: MarketingLotteryPrize;
  prize_name?: string;
  prize_type?: "THANKS" | "COUPON";
  warning?: string;
  coupon?: { campaign?: MarketingCoupon };
}

Page({
  data: { loading: true, campaigns: [] as LotteryView[], selected: null as LotteryView | null, drawing: false, error: "", appearanceStyle: "" },
  onLoad(options: Record<string, string>) { void this.loadCampaigns(Number(options.id || 0)); },
  async loadCampaigns(preferredID = 0) {
    const appearance = await loadPageAppearance();
    this.setData({ appearanceStyle: appearance.appearanceStyle });
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    const channelScope = tableContextForStore(storeCode) ? "DINE_IN" : "TAKEOUT";
    this.setData({ loading: true, error: "" });
    try {
      const response = await request<MarketingLottery[] | { items?: MarketingLottery[] }>({ url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/lotteries?channelScope=${channelScope}`, method: "GET" });
      const rawCampaigns = Array.isArray(response) ? response : response.items || [];
      const campaigns: LotteryView[] = rawCampaigns.map((campaign) => ({
        ...campaign,
        prizes: (campaign.prizes || []).map((prize) => ({
          ...prize,
          stockText: prize.prize_type === "THANKS"
            ? "参与奖"
            : `剩余 ${Math.max(0, Number(prize.total_stock || 0) - Number(prize.awarded_count || 0))} 份`,
        })),
      }));
      const selected = campaigns.find((item) => item.id === preferredID) || campaigns[0] || null;
      this.setData({ campaigns, selected, loading: false });
    } catch (error) {
      this.setData({ loading: false, error: customerSafeErrorMessage(error, "活动暂时无法加载，请稍后重试。") });
    }
  },
  onPullDownRefresh() { this.loadCampaigns(this.data.selected?.id || 0).finally(() => wx.stopPullDownRefresh()); },
  chooseCampaign(event: WechatMiniprogram.BaseEvent) {
    const selected = this.data.campaigns.find((item) => item.id === Number(event.currentTarget.dataset.id)) || null;
    this.setData({ selected });
  },
  async draw() {
    const campaign = this.data.selected;
    if (!campaign || this.data.drawing) return;
    this.setData({ drawing: true });
    const storeCode = getApp<TanbanAppOption>().globalData.storeCode;
    try {
      const result = await request<DrawResult>({
        url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/lotteries/${campaign.id}/draw`, method: "POST",
        header: { "Idempotency-Key": idempotencyKey("draw") },
        data: { subject_key: customerGuestKey() },
      });
      const prizeType = result.prize_type || result.prize?.prize_type;
      const prizeName = result.prize_name || result.prize?.name || "谢谢参与";
      if (result.coupon?.campaign) rememberClaimedCoupon(storeCode, result.coupon.campaign);
      wx.showModal({
        title: prizeType === "COUPON" ? "恭喜中奖" : "本次结果",
        content: prizeName,
        showCancel: false,
      });
      await this.loadCampaigns(campaign.id);
    } catch (error) {
      wx.showToast({ title: customerSafeErrorMessage(error, "暂时无法参与活动，请稍后重试。"), icon: "none" });
    } finally {
      this.setData({ drawing: false });
    }
  },
  goCoupons() { wx.navigateTo({ url: "/pages/coupons/index" }); },
});
