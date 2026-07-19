import { env } from "./config/env";
import { routedStoreCode } from "./utils/store-route";

export interface TanbanAppOption {
  globalData: {
    storeCode: string;
    customerToken: string;
  };
}

App<TanbanAppOption>({
  globalData: {
    storeCode: env.defaultStoreCode,
    customerToken: "",
  },
  onLaunch(options) {
    const storeCode = routedStoreCode(options.query as Record<string, string | undefined>);
    if (storeCode) this.globalData.storeCode = storeCode;
    const token = wx.getStorageSync<string>("tanban_customer_token");
    if (token) this.globalData.customerToken = token;
  },
  onShow(options) {
    const storeCode = routedStoreCode(options.query as Record<string, string | undefined>);
    if (storeCode) this.globalData.storeCode = storeCode;
  },
});
