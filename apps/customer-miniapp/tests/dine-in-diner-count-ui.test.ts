import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

function read(relativePath: string): string {
  return readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), "utf8");
}

const checkoutSource = read("../miniprogram/pages/checkout/index.ts");
const checkoutTemplate = read("../miniprogram/pages/checkout/index.wxml");
const orderDetailTemplate = read("../miniprogram/pages/order-detail/index.wxml");

describe("dine-in diner count", () => {
  it("shows a 1-20 diner selector only for table-code orders", () => {
    expect(checkoutTemplate).toContain('class="card diner-count-card" wx:if="{{tableContext}}"');
    expect(checkoutTemplate).toContain("就餐人数");
    expect(checkoutTemplate).toContain('range="{{dinerOptions}}"');
    expect(checkoutTemplate).toContain('bindchange="selectDinerCount"');
    expect(checkoutSource).toContain("dinerOptions: Array.from({ length: 20 }, (_, index) => index + 1)");
    expect(checkoutSource).toContain("dinerCount: tableContext ? 2 : 1");
  });

  it("submits and displays the diner count for dine-in orders", () => {
    expect(checkoutSource).toContain('fulfillmentType: tableContext ? "DINE_IN" : "PICKUP", dinerCount: tableContext ? this.data.dinerCount : 1');
    expect(orderDetailTemplate).toContain('wx:if="{{order.isDineIn && order.dinerCount}}"');
    expect(orderDetailTemplate).toContain("{{order.dinerCount}} 人");
  });
});
