import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const menuTemplatePath = fileURLToPath(new URL("../miniprogram/pages/menu/index.wxml", import.meta.url));
const menuTemplate = readFileSync(menuTemplatePath, "utf8");
const menuStylesPath = fileURLToPath(new URL("../miniprogram/pages/menu/index.wxss", import.meta.url));
const menuStyles = readFileSync(menuStylesPath, "utf8");

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

  it("keeps configuration choices compact and hides only the scroll indicator", () => {
    expect(menuTemplate).toContain('class="configuration-scroll" scroll-y enhanced show-scrollbar="{{false}}"');
    expect(menuTemplate).not.toContain('class="configuration-option-cell"');
    expect(menuTemplate).toContain('<view role="button" class="configuration-option');
    expect(menuTemplate).not.toContain('<button class="configuration-option');
    expect(menuStyles).toMatch(/\.configuration-options \{[^}]*display: block;[^}]*font-size: 0;/);
    expect(menuStyles).toMatch(/\.configuration-option \{[^}]*display: inline-flex;[^}]*min-width: 144rpx;[^}]*margin: 0 10rpx 12rpx 0;/);
  });

  it("shows member prices consistently without crowding the recommended action", () => {
    expect(menuTemplate).toContain('class="sku-option-price" wx:if="{{item.memberPrice !== undefined && item.memberPrice < item.price}}"');
    expect(menuTemplate).toContain("{{pickerHasMemberPrice ? '本份会员价' : '本份预计'}}");
    expect(menuTemplate).toContain('class="configuration-original-price" wx:if="{{pickerHasMemberPrice}}"');
    expect(menuStyles).toMatch(/\.recommended-card-footer > \.member-price-line \{[^}]*flex-direction: column;[^}]*overflow: hidden;/);
    expect(menuStyles).toMatch(/\.sku-option-price text:last-child \{[^}]*text-decoration: line-through;/);
  });
});
