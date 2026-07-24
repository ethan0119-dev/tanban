export interface MiniappEnvironment {
  apiBaseUrl: string;
  defaultStoreCode: string;
  requestTimeoutMs: number;
}

// 一期直接使用生产 API；需要本地联调时只修改这里，不要把密钥放进小程序。
export const env: MiniappEnvironment = {
  apiBaseUrl: "https://tbapi.666qwe.cn/api/v1",
  defaultStoreCode: "manong-coffee-gulou",
  requestTimeoutMs: 10_000,
};
