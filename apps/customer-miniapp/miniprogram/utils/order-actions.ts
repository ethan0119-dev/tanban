import type { Order } from "../types/domain";

export interface CustomerOrderActions {
  isDineIn: boolean;
  payAfterMeal: boolean;
  canPay: boolean;
  canAddItems: boolean;
  paymentSucceeded: boolean;
  paymentPending: boolean;
  paidAmount: number;
  remainingAmount: number;
}

const closedStatuses = new Set(["CLOSED", "CANCELED", "CANCELLED", "REFUNDED", "COMPLETED"]);
const payAfterStatuses = new Set(["PAID", "ACCEPTED", "PREPARING", "READY"]);

/**
 * The API remains authoritative for add-item eligibility because it also
 * checks active payment transactions. This helper only adds client-side scene
 * guards and keeps the payment action aligned with the same order lifecycle.
 */
export function customerOrderActions(order: Order, payAfterOnlinePaymentEnabled: boolean): CustomerOrderActions {
  const paymentStatus = String(order.paymentStatus || "").toUpperCase();
  const orderStatus = String(order.status || "").toUpperCase();
  const paymentSucceeded = paymentStatus === "SUCCEEDED" || paymentStatus === "PAID";
  const payAfterMeal = order.settlementMode === "PAY_AFTER";
  const paidAmount = Math.max(Number(order.paidAmount || 0), 0);
  const remainingAmount = Math.max(Number(order.remainingAmount ?? (order.amount - paidAmount)), 0);
  const isDineIn = order.orderScene === "DINE_IN"
    || order.order_scene === "DINE_IN"
    || Boolean(order.tablePublicId || order.table?.publicId);
  const canPayBeforeMeal = !payAfterMeal && orderStatus === "PENDING_PAYMENT";
  const canPayAfterMeal = payAfterMeal && payAfterOnlinePaymentEnabled && payAfterStatuses.has(orderStatus);
  const canPay = !paymentSucceeded
    && remainingAmount > 0
    && !closedStatuses.has(orderStatus)
    && (canPayBeforeMeal || canPayAfterMeal);
  const paymentPending = !payAfterMeal && !paymentSucceeded
    && ["", "UNPAID", "PENDING", "CREATED", "PROCESSING"].includes(paymentStatus)
    && orderStatus === "PENDING_PAYMENT";

  return {
    isDineIn,
    payAfterMeal,
    canPay,
    canAddItems: Boolean(isDineIn && order.canAddItems && !paymentSucceeded),
    paymentSucceeded,
    paymentPending,
    paidAmount,
    remainingAmount,
  };
}
