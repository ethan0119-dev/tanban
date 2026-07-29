import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { canPresentMembershipLevel } from "../miniprogram/utils/membership";
import { customRechargeAllowed, yuanInputToCents, yuanText } from "../miniprogram/utils/recharge";

function read(relativePath: string): string {
  return readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), "utf8");
}

const membershipScript = read("../miniprogram/pages/membership/index.ts");
const membershipTemplate = read("../miniprogram/pages/membership/index.wxml");
const rechargeScript = read("../miniprogram/pages/recharge/index.ts");
const rechargeTemplate = read("../miniprogram/pages/recharge/index.wxml");

describe("membership and stored-value customer flows", () => {
  it("only presents levels above the customer's current rank", () => {
    expect(canPresentMembershipLevel(false, -1, 1)).toBe(true);
    expect(canPresentMembershipLevel(true, 2, 3)).toBe(true);
    expect(canPresentMembershipLevel(true, 2, 2)).toBe(false);
    expect(canPresentMembershipLevel(true, 2, 1)).toBe(false);
    expect(membershipScript).toContain("canPresentMembershipLevel(hasCurrentLevel, currentRank, level.rank)");
    expect(membershipTemplate).toContain("只支持向更高等级升级");
    expect(membershipTemplate).toContain("已达最高等级");
    expect(membershipTemplate).toContain("低于或等于当前等级的卡片已自动隐藏");
  });

  it("distinguishes membership upgrade payments from repeatable stored-value promotions", () => {
    expect(membershipTemplate).toContain("等级开通金额全部进入本金余额");
    expect(membershipScript).toContain("不参与储值中心的赠送活动");
    expect(rechargeTemplate).toContain("优惠档位按页面规则赠送金额");
    expect(rechargeTemplate).toContain("自定义充值只增加本金");
  });

  it("supports a guarded custom recharge amount without a gift", () => {
    expect(yuanInputToCents("200")).toBe(20000);
    expect(yuanInputToCents("200.08")).toBe(20008);
    expect(yuanInputToCents("1.234")).toBe(0);
    expect(yuanText(20008)).toBe("200.08");
    expect(customRechargeAllowed(20000, 1000, 50000)).toBe(true);
    expect(customRechargeAllowed(999, 1000, 50000)).toBe(false);
    expect(customRechargeAllowed(50001, 1000, 50000)).toBe(false);
    expect(rechargeTemplate).toContain('bindinput="setCustomAmount"');
    expect(rechargeScript).toContain("customRechargeAllowed(customAmountCents");
    expect(rechargeScript).toContain("data: { ruleId: rule?.id || 0, amountCents }");
    expect(rechargeScript).toContain("自定义充值不享赠送金额");
  });
});
