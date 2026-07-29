import { env } from "../config/env";
import type { TanbanAppOption } from "../app";

interface ApiEnvelope<T> {
  data: T;
  meta?: Record<string, unknown>;
  error?: { code?: string; message?: string } | string;
}

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly statusCode: number,
    public readonly code?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export function request<T>(options: WechatMiniprogram.RequestOption): Promise<T> {
  const app = getApp<TanbanAppOption>();
  const token = app.globalData.customerToken;

  return new Promise((resolve, reject) => {
    wx.request<ApiEnvelope<T>>({
      ...options,
      url: options.url.startsWith("http") ? options.url : `${env.apiBaseUrl}${options.url}`,
      timeout: env.requestTimeoutMs,
      header: {
        "content-type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(options.header || {}),
      },
      success(response) {
        if (response.statusCode >= 200 && response.statusCode < 300) {
          const body = response.data;
          resolve((body && Object.prototype.hasOwnProperty.call(body, "data") ? body.data : body) as T);
          return;
        }
        const error = response.data?.error;
        const message = typeof error === "string" ? error : error?.message;
        const code = typeof error === "string" ? undefined : error?.code;
        reject(new ApiError(message || "请求失败，请稍后重试", response.statusCode, code));
      },
      fail(error) {
        reject(new ApiError(error.errMsg || "网络连接失败", 0));
      },
    });
  });
}

export function idempotencyKey(prefix = "wx"): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}
