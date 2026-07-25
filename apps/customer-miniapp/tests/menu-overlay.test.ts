import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const menuTemplatePath = fileURLToPath(new URL("../miniprogram/pages/menu/index.wxml", import.meta.url));
const menuTemplate = readFileSync(menuTemplatePath, "utf8");

describe("menu overlays", () => {
  it("keeps the ordering page and its overlays in the same conditional render root", () => {
    const stageStart = menuTemplate.indexOf('<view class="menu-stage"');
    const menuStart = menuTemplate.indexOf('<view class="page menu-page');
    const skuMaskStart = menuTemplate.indexOf('<view class="sku-mask"');
    const stageEnd = menuTemplate.lastIndexOf("</view>\n<view class=\"empty\" wx:elif");

    expect(stageStart).toBe(0);
    expect(menuStart).toBeGreaterThan(stageStart);
    expect(skuMaskStart).toBeGreaterThan(menuStart);
    expect(stageEnd).toBeGreaterThan(skuMaskStart);
    expect(menuTemplate.slice(stageStart, stageEnd)).toContain('<view class="cart-sheet-mask"');
    expect(menuTemplate.slice(stageStart, stageEnd)).toContain('<view class="sku-mask"');
    expect(menuTemplate).not.toContain("<block wx:if");
  });
});
