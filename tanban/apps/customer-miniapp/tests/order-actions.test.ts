import { describe, expect, it } from "vitest";
import type { Order } from "../miniprogram/types/domain";
import { customerOrderActions } from "../miniprogram/utils/order-actions";

function order(overrides: Partial<Order> = {}): Order {
  return {
    id: 1,
    orderNo: "TB001",
    status: "PENDING_PAYMENT",
    paymentStatus: "UNPAID",
    settlementMode: "PAY_BEFORE",
    orderScene: "DINE_IN",
    tablePublicId: "table-1",
    amount: 1500,
    paidAmount: 0,
    remainingAmount: 1500,
    canAddItems: true,
    createdAt: "2026-07-26 12:00:00",
    ...overrides,
  };
}

describe("customerOrderActions", () => {
  it("offers payment and additions before a pay-before dine-in order is paid", () => {
    expect(customerOrderActions(order(), true)).toMatchObject({ canPay: true, canAddItems: true });
  });

  it("locks additions once a pay-before order is paid", () => {
    expect(customerOrderActions(order({
      status: "PAID",
      paymentStatus: "SUCCEEDED",
      paidAmount: 1500,
      remainingAmount: 0,
      canAddItems: false,
    }), true)).toMatchObject({ canPay: false, canAddItems: false });
  });

  it.each(["PAID", "ACCEPTED", "PREPARING", "READY"])(
    "allows payment and additions for an untouched pay-after order in %s",
    (status) => {
      expect(customerOrderActions(order({
        status,
        settlementMode: "PAY_AFTER",
      }), true)).toMatchObject({ canPay: true, canAddItems: true });
    },
  );

  it("keeps only payment after a pay-after bill is partially collected", () => {
    expect(customerOrderActions(order({
      status: "PREPARING",
      settlementMode: "PAY_AFTER",
      paidAmount: 600,
      remainingAmount: 900,
      canAddItems: false,
    }), true)).toMatchObject({ canPay: true, canAddItems: false });
  });

  it("never treats a takeout order as an add-item order", () => {
    expect(customerOrderActions(order({
      orderScene: "TAKEOUT",
      fulfillmentType: "PICKUP",
      tablePublicId: undefined,
      canAddItems: true,
    }), true)).toMatchObject({ canPay: true, canAddItems: false });
  });

  it("allows additions but not online settlement when pay-after online payment is disabled", () => {
    expect(customerOrderActions(order({
      status: "READY",
      settlementMode: "PAY_AFTER",
    }), false)).toMatchObject({ canPay: false, canAddItems: true });
  });
});
