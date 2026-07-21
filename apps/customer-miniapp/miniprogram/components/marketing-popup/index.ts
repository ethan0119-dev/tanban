import type { MarketingPlacement } from "../../types/domain";
import { customerGuestKey } from "../../utils/customer";
import { marketingEventKey, rememberMarketingPopup, shouldDisplayMarketingPopup } from "../../utils/marketing";
import { idempotencyKey, request } from "../../utils/request";
import { customerExperienceCopy, customerSafeErrorMessage } from "../../utils/availability";

Component({
  properties: {
    storeCode: { type: String, value: "" },
    placementCode: { type: String, value: "HOME_POPUP" },
    channelScope: { type: String, value: "ALL" },
  },
  data: {
    placement: null as MarketingPlacement | null,
    visible: false,
    loadRequestID: 0,
  },
  observers: {
    "storeCode, placementCode, channelScope"() {
      void this.loadPlacement();
    },
  },
  methods: {
    async loadPlacement() {
      const loadRequestID = this.data.loadRequestID + 1;
      this.setData({ loadRequestID });
      const storeCode = String(this.properties.storeCode || "").trim();
      const placementCode = String(this.properties.placementCode || "HOME_POPUP").trim();
      const channelScope = String(this.properties.channelScope || "ALL").trim();
      if (!storeCode) {
        this.setData({ placement: null, visible: false });
        return;
      }
      try {
        const placement = await request<MarketingPlacement | null>({
          url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/popup?placementCode=${encodeURIComponent(placementCode)}&channelScope=${encodeURIComponent(channelScope)}`,
          method: "GET",
        });
        if (this.data.loadRequestID !== loadRequestID) return;
        if (!placement || !shouldDisplayMarketingPopup(storeCode, placement)) {
          this.setData({ placement: null, visible: false });
          return;
        }
        rememberMarketingPopup(storeCode, placement);
        this.setData({ placement, visible: true });
        void this.recordEvent("IMPRESSION");
      } catch {
        if (this.data.loadRequestID !== loadRequestID) return;
        this.setData({ placement: null, visible: false });
      }
    },
    async recordEvent(eventType: "IMPRESSION" | "CLICK" | "CLOSE") {
      const placement = this.data.placement;
      const storeCode = String(this.properties.storeCode || "").trim();
      if (!placement || !storeCode) return;
      try {
        await request({
          url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/events`,
          method: "POST",
          header: { "Idempotency-Key": marketingEventKey(placement.id, eventType) },
          data: { placement_id: placement.id, event_type: eventType, subject_key: customerGuestKey() },
        });
      } catch {
        // 统计失败不影响顾客操作。
      }
    },
    close() {
      const placement = this.data.placement;
      const storeCode = String(this.properties.storeCode || "").trim();
      if (!placement || !this.data.visible) return;
      rememberMarketingPopup(storeCode, placement);
      this.setData({ visible: false });
      void this.recordEvent("CLOSE");
    },
    async action() {
      const placement = this.data.placement;
      const storeCode = String(this.properties.storeCode || "").trim();
      if (!placement || !storeCode || !this.data.visible) return;
      rememberMarketingPopup(storeCode, placement);
      this.setData({ visible: false });
      void this.recordEvent("CLICK");
      if (placement.action_type === "OPEN_MENU") return void wx.switchTab({ url: "/pages/menu/index" });
      if (placement.action_type === "OPEN_COUPONS") return void wx.navigateTo({ url: "/pages/coupons/index" });
      if (placement.action_type === "OPEN_LOTTERY") return void wx.navigateTo({ url: `/pages/lottery/index?id=${placement.action_target_id || ""}` });
      if (placement.action_type === "CLAIM_COUPON" && placement.action_target_id) {
        try {
          const result = await request<{ warning?: string }>({
            url: `/public/stores/${encodeURIComponent(storeCode)}/marketing/coupons/${placement.action_target_id}/claim`,
            method: "POST",
            header: { "Idempotency-Key": idempotencyKey("popup_coupon") },
            data: { subject_key: customerGuestKey() },
          });
          void result;
          wx.showModal({ title: "领取结果", content: customerExperienceCopy.couponClaimed, showCancel: false });
        } catch (error) {
          wx.showToast({ title: customerSafeErrorMessage(error, "暂时无法领取，请稍后重试。"), icon: "none" });
        }
      }
    },
    noop() {},
  },
});
