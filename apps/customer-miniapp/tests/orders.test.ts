import { describe, expect, it } from "vitest";
import { sortCustomerOrders } from "../miniprogram/utils/orders";

describe("sortCustomerOrders", () => {
  it("shows unfinished orders first and sorts each group newest first", () => {
    const orders = [
      { id: 1, current: false, createdAt: "2026-07-25 12:00:00" },
      { id: 2, current: true, createdAt: "2026-07-24 12:00:00" },
      { id: 3, current: true, createdAt: "2026-07-25 11:00:00" },
      { id: 4, current: false, createdAt: "2026-07-26 12:00:00" },
    ];

    expect(sortCustomerOrders(orders).map((item) => item.id)).toEqual([3, 2, 4, 1]);
    expect(orders.map((item) => item.id)).toEqual([1, 2, 3, 4]);
  });
});
