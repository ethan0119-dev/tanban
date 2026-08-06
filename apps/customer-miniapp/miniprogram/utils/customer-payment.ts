import { request } from "./request";

export type PaymentClientAction = "NONE" | "MOCK_CONFIRM" | "WECHAT_REQUEST_PAYMENT" | "OPEN_EMBEDDED_CASHIER" | "NAVIGATE_TO_MINIPROGRAM";

export interface ClientPaymentResult {
  id: number;
  provider: string;
  status: string;
  clientAction?: PaymentClientAction;
  wxPayParams?: WechatMiniprogram.RequestPaymentOption;
}

export interface CustomerAccountPayment extends ClientPaymentResult {
  fulfilled?: boolean;
}

function validWechatPayParams(value?: WechatMiniprogram.RequestPaymentOption): value is WechatMiniprogram.RequestPaymentOption {
  return Boolean(value?.timeStamp && value.nonceStr && value.package && value.signType && value.paySign);
}

export async function executeClientPayment(payment: ClientPaymentResult, confirmMock?: () => Promise<unknown>): Promise<void> {
  const action = payment.clientAction
    || (payment.provider === "mock" ? "MOCK_CONFIRM" : validWechatPayParams(payment.wxPayParams) ? "WECHAT_REQUEST_PAYMENT" : "NONE");
  if (action === "NONE") return;
  if (action === "MOCK_CONFIRM" && confirmMock) {
    await confirmMock();
    return;
  }
  if (action === "WECHAT_REQUEST_PAYMENT" && validWechatPayParams(payment.wxPayParams)) {
    await new Promise<void>((resolve, reject) => wx.requestPayment({ ...payment.wxPayParams!, success: () => resolve(), fail: reject }));
    return;
  }
  if (action === "OPEN_EMBEDDED_CASHIER") throw new Error("当前支付机构的收银台尚未完成小程序接入");
  if (action === "NAVIGATE_TO_MINIPROGRAM") throw new Error("当前支付机构的小程序跳转尚未完成接入");
  throw new Error("支付参数缺失，请稍后重试");
}

export async function completeCustomerAccountPayment(payment: CustomerAccountPayment): Promise<void> {
  await executeClientPayment(payment, () => request({
    url: `/public/account-payments/${payment.id}/mock-confirm`,
    method: "POST",
  }));
}
