import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

function read(relativePath: string): string {
  return readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), "utf8");
}

const componentScript = read("../miniprogram/components/member-price/index.ts");
const componentTemplate = read("../miniprogram/components/member-price/index.wxml");
const componentStyles = read("../miniprogram/components/member-price/index.wxss");
const menuConfig = read("../miniprogram/pages/menu/index.json");
const menuTemplate = read("../miniprogram/pages/menu/index.wxml");

describe("member price component", () => {
  it("is registered on the menu page and used for all discounted menu price surfaces", () => {
    expect(JSON.parse(menuConfig).usingComponents).toEqual({
      "member-price": "/components/member-price/index",
    });
    expect(menuTemplate.match(/<member-price\b/g)).toHaveLength(4);
  });

  it("groups the membership label and discounted amount in one offer frame", () => {
    expect(componentTemplate).toContain('class="member-price__offer"');
    expect(componentTemplate).toContain('class="member-price__label">{{label}}</text>');
    expect(componentTemplate).toContain('class="member-price__amount">¥{{memberPrice / 100}}</text>');
    expect(componentTemplate).toContain('class="member-price__original" wx:if="{{showOriginal && originalPrice > memberPrice}}"');
    expect(componentScript).toContain('label: { type: String, value: "会员价" }');
  });

  it("supports compact, option, summary, and selected visual states", () => {
    expect(componentStyles).toMatch(/\.member-price__label \{[^}]*background: #3f302b;[^}]*color: #f5dfa5;/);
    expect(componentStyles).toMatch(/\.member-price__amount \{[^}]*background: #f0dda6;[^}]*color: #3f302b;/);
    expect(componentStyles).toContain(".member-price--stacked");
    expect(componentStyles).toContain(".member-price--option");
    expect(componentStyles).toContain(".member-price--summary");
    expect(componentStyles).toContain(".is-selected .member-price__original");
  });
});
