export interface BalancePaymentBreakdown {
  balanceDeductionAmount: number;
  remainingPaymentAmount: number;
}

function nonNegativeCents(value: number): number {
  return Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0;
}

/**
 * Builds the customer-facing payment split. Once a balance deduction has
 * already been applied by the server, that immutable amount wins over the
 * current wallet balance so a resumed checkout never promises to deduct twice.
 */
export function balancePaymentBreakdown(
  orderAmount: number,
  balanceCents: number,
  useBalance: boolean,
  appliedBalanceAmount = 0,
): BalancePaymentBreakdown {
  const amount = nonNegativeCents(orderAmount);
  const applied = Math.min(amount, nonNegativeCents(appliedBalanceAmount));
  const deduction = applied > 0
    ? applied
    : useBalance
      ? Math.min(amount, nonNegativeCents(balanceCents))
      : 0;

  return {
    balanceDeductionAmount: deduction,
    remainingPaymentAmount: amount - deduction,
  };
}
