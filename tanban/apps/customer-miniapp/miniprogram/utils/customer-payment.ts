import { request } from "./request";

export interface CustomerAccountPayment {
  id: number;
  provider: "free" | "mock" | "tianque" | "wechat_partner";
  status: string;
  fulfilled?: boolean;
  wxPayParams?: WechatMiniprogram.RequestPaymentOption;
}

function validWechatPayParams(value?: WechatMiniprogram.RequestPaymentOption): value is WechatMiniprogram.RequestPaymentOption {
  return Boolean(value?.timeStamp && value.nonceStr && value.package && value.signType && value.paySign);
}

export async function completeCustomerAccountPayment(payment: CustomerAccountPayment): Promise<void> {
  if (payment.provider === "free") return;
  if (payment.provider === "mock") {
    await request({
      url: `/public/account-payments/${payment.id}/mock-confirm`,
      method: "POST",
    });
    return;
  }
  if (payment.provider === "wechat_partner" && validWechatPayParams(payment.wxPayParams)) {
    await new Promise<void>((resolve, reject) => wx.requestPayment({ ...payment.wxPayParams!, success: () => resolve(), fail: reject }));
    return;
  }
  if (payment.provider === "tianque") throw new Error("会生活收银台尚未完成小程序接入");
  throw new Error("支付参数缺失，请稍后重试");
}
