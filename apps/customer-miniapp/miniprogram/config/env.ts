export interface MiniappEnvironment {
  apiBaseUrl: string;
  defaultStoreCode: string;
  channelKey: string;
  requestTimeoutMs: number;
}

// 构建脚本会按发布目标替换渠道和默认门店。这里只能放公开配置，不能放服务端密钥。
export const env: MiniappEnvironment = {
  apiBaseUrl: "https://api.tanban.com.cn/api/v1",
  defaultStoreCode: "manong-coffee-gulou",
  channelKey: "tanban-public",
  requestTimeoutMs: 10_000,
};
