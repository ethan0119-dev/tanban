import { env } from "../config/env";
import { customerGuestKey } from "./customer";

interface SessionEnvelope {
  data?: { accessToken?: string };
  error?: { message?: string } | string;
}

function loginCode(): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.login({
      success(result) {
        if (result.code) resolve(result.code);
        else reject(new Error("微信登录凭证为空"));
      },
      fail(error) {
        reject(new Error(error.errMsg || "微信登录失败"));
      },
    });
  });
}

export async function createCustomerSession(storeCode: string): Promise<string> {
  const code = await loginCode();
  return new Promise((resolve, reject) => {
    wx.request<SessionEnvelope>({
      url: `${env.apiBaseUrl}/public/customer/session`,
      method: "POST",
      timeout: env.requestTimeoutMs,
      header: { "content-type": "application/json" },
      data: { code, guestKey: customerGuestKey(), storeCode, channelKey: env.channelKey },
      success(response) {
        const token = response.data?.data?.accessToken;
        if (response.statusCode >= 200 && response.statusCode < 300 && token) {
          resolve(token);
          return;
        }
        const apiError = response.data?.error;
        reject(new Error(typeof apiError === "string" ? apiError : apiError?.message || "微信登录失败"));
      },
      fail(error) {
        reject(new Error(error.errMsg || "微信登录失败"));
      },
    });
  });
}
