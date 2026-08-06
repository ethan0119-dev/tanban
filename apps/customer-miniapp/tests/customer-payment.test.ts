import { afterEach, describe, expect, it, vi } from "vitest";
import { executeClientPayment } from "../miniprogram/utils/customer-payment";

describe("unified customer payment executor", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses wx.requestPayment for any provider that returns the shared action", async () => {
    const requestPayment = vi.fn((options: WechatMiniprogram.RequestPaymentOption) => options.success?.({ errMsg: "requestPayment:ok" }));
    vi.stubGlobal("wx", { requestPayment });

    await executeClientPayment({
      id: 1,
      provider: "lichu",
      status: "PENDING",
      clientAction: "WECHAT_REQUEST_PAYMENT",
      wxPayParams: { timeStamp: "1710000000", nonceStr: "nonce", package: "prepay_id=123", signType: "RSA", paySign: "signature" },
    });

    expect(requestPayment).toHaveBeenCalledOnce();
  });

  it("fails closed when a payment action is missing required parameters", async () => {
    vi.stubGlobal("wx", { requestPayment: vi.fn() });
    await expect(executeClientPayment({ id: 2, provider: "lichu", status: "PENDING", clientAction: "WECHAT_REQUEST_PAYMENT" }))
      .rejects.toThrow("支付参数缺失");
  });
});
