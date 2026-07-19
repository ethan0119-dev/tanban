export type PaymentMode = "mock" | "tianque";

export interface MiniappEnvironment {
  apiBaseUrl: string;
  defaultStoreCode: string;
  paymentMode: PaymentMode;
  requestTimeoutMs: number;
}

// 一期直接使用生产 API；需要本地联调时只修改这里，不要把密钥放进小程序。
export const env: MiniappEnvironment = {
  apiBaseUrl: "https://tbapi.666qwe.cn/api/v1",
  defaultStoreCode: "manong-coffee",
  paymentMode: "mock",
  requestTimeoutMs: 10_000,
};
