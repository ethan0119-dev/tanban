import { describe, expect, it } from "vitest";
import { balancePaymentBreakdown } from "../miniprogram/utils/payment-breakdown";

describe("balancePaymentBreakdown", () => {
  it("uses balance by default and reports no extra payment when it covers the order", () => {
    expect(balancePaymentBreakdown(1500, 2200, true)).toEqual({
      balanceDeductionAmount: 1500,
      remainingPaymentAmount: 0,
    });
  });

  it("uses all available balance and leaves only the shortfall for online payment", () => {
    expect(balancePaymentBreakdown(1500, 600, true)).toEqual({
      balanceDeductionAmount: 600,
      remainingPaymentAmount: 900,
    });
  });

  it("does not apply available balance after the customer turns it off", () => {
    expect(balancePaymentBreakdown(1500, 600, false)).toEqual({
      balanceDeductionAmount: 0,
      remainingPaymentAmount: 1500,
    });
  });

  it("keeps a server-applied deduction when checkout is resumed", () => {
    expect(balancePaymentBreakdown(1500, 0, false, 600)).toEqual({
      balanceDeductionAmount: 600,
      remainingPaymentAmount: 900,
    });
  });
});
