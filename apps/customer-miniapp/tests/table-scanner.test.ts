import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TableOrderingContext } from "../miniprogram/types/domain";
import { scanAndBindTableCode } from "../miniprogram/utils/table-scanner";

const context: TableOrderingContext = {
  publicScene: "0123456789abcdef0123456789ab",
  storeCode: "coffee",
  storeName: "测试门店",
  tablePublicId: "table-1",
  tableName: "A01",
  resolvedAt: Date.now(),
  validUntil: Date.now() + 60_000,
};

describe("table scanner", () => {
  let storedContext: TableOrderingContext | null;
  let showToast: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    storedContext = null;
    showToast = vi.fn();
    vi.stubGlobal("wx", {
      getStorageSync: vi.fn(() => storedContext),
      removeStorageSync: vi.fn(),
      showToast,
      scanCode: vi.fn(),
    });
  });

  it("opens the scanner and binds a valid table code", async () => {
    const prepareOrderingEntry = vi.fn(async () => {
      storedContext = context;
      app.globalData.storeCode = context.storeCode;
      app.globalData.tableContext = context;
    });
    const app = {
      globalData: { storeCode: "old-store", tableContext: null as TableOrderingContext | null, routeError: "" },
      prepareOrderingEntry,
    };
    vi.stubGlobal("getApp", () => app);
    vi.mocked(wx.scanCode).mockImplementation((options) => {
      options.success?.({ path: `pages/home/index?scene=${context.publicScene}` } as WechatMiniprogram.ScanCodeSuccessCallbackResult);
      return undefined as never;
    });

    await expect(scanAndBindTableCode()).resolves.toEqual({
      context,
      previousStoreCode: "old-store",
      storeCode: "coffee",
    });
    expect(prepareOrderingEntry).toHaveBeenCalledOnce();
    expect(wx.scanCode).toHaveBeenCalledWith(expect.objectContaining({ scanType: ["wxCode", "qrCode"] }));
  });

  it("rejects a non-table QR code with guidance", async () => {
    vi.mocked(wx.scanCode).mockImplementation((options) => {
      options.success?.({ result: "https://example.com/not-a-table" } as WechatMiniprogram.ScanCodeSuccessCallbackResult);
      return undefined as never;
    });

    await expect(scanAndBindTableCode()).resolves.toBeNull();
    expect(showToast).toHaveBeenCalledWith(expect.objectContaining({ title: "这不是本店桌码，请扫描桌面上的点餐码" }));
  });

  it("keeps user cancellation silent", async () => {
    vi.mocked(wx.scanCode).mockImplementation((options) => {
      options.fail?.({ errMsg: "scanCode:fail cancel" });
      return undefined as never;
    });

    await expect(scanAndBindTableCode()).resolves.toBeNull();
    expect(showToast).not.toHaveBeenCalled();
  });
});
