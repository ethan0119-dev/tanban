import type { TanbanAppOption } from "../app";
import type { TableOrderingContext } from "../types/domain";
import { orderingEntryOptionsFromScan, parseOrderingEntry } from "./store-route";
import { tableContextForStore } from "./table-context";

export interface TableScanBinding {
  context: TableOrderingContext;
  previousStoreCode: string;
  storeCode: string;
}

/**
 * Opens WeChat's scanner, validates a table code through the normal app-entry
 * pipeline, and returns the bound table context. User cancellation is silent.
 */
export function scanAndBindTableCode(): Promise<TableScanBinding | null> {
  return new Promise((resolve) => {
    wx.scanCode({
      onlyFromCamera: false,
      scanType: ["wxCode", "qrCode"],
      success: (result) => {
        void bindScannedTableCode(result.path || result.result)
          .then(resolve)
          .catch(() => {
            wx.showToast({ title: "桌码暂时无法使用，请稍后重试", icon: "none" });
            resolve(null);
          });
      },
      fail: (error) => {
        if (!String(error.errMsg || "").includes("cancel")) {
          wx.showToast({ title: "未能打开扫一扫，请稍后重试", icon: "none" });
        }
        resolve(null);
      },
    });
  });
}

async function bindScannedTableCode(pathOrResult: string): Promise<TableScanBinding | null> {
  const options = orderingEntryOptionsFromScan(pathOrResult);
  if (!options || parseOrderingEntry(options).kind !== "TABLE") {
    wx.showToast({ title: "这不是本店桌码，请扫描桌面上的点餐码", icon: "none" });
    return null;
  }
  const app = getApp<TanbanAppOption>();
  const previousStoreCode = app.globalData.storeCode;
  try {
    await app.prepareOrderingEntry(options, false);
  } catch {
    wx.showToast({ title: "桌码暂时无法使用，请稍后重试", icon: "none" });
    return null;
  }
  // prepareOrderingEntry already shows a specific modal for an invalid,
  // disabled or expired table code.
  if (app.globalData.routeError) return null;
  const storeCode = app.globalData.storeCode;
  const context = tableContextForStore(storeCode);
  if (!context) {
    wx.showToast({ title: "桌码暂时无法使用，请联系店员", icon: "none" });
    return null;
  }
  return { context, previousStoreCode, storeCode };
}
