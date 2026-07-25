package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type orderBalancePaymentResult struct {
	ID, AmountCents, RemainingCents int64
	Reference                       string
	FullyPaid                       bool
}

func (s *Server) applyOrderBalancePaymentLocked(ctx context.Context, conn *sql.Conn, tenantID, orderID, sessionCustomerID int64) (orderBalancePaymentResult, error) {
	if sessionCustomerID <= 0 {
		return orderBalancePaymentResult{}, nil
	}
	var activeExternal int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_transactions
		WHERE tenant_id=? AND order_id=? AND status IN ('CREATING','PENDING','SUCCESS')`, tenantID, orderID).Scan(&activeExternal); err != nil {
		return orderBalancePaymentResult{}, err
	}
	if activeExternal > 0 {
		return orderBalancePaymentResult{}, nil
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return orderBalancePaymentResult{}, err
	}
	defer tx.Rollback()
	var storeID, customerID, totalCents, paidCents int64
	var orderNo, orderStatus, paymentStatus, settlementMode string
	var inventoryReserved int
	err = tx.QueryRowContext(ctx, `SELECT store_id,COALESCE(customer_id,0),order_no,total_cents,paid_cents,status,payment_status,
		settlement_mode_snapshot,inventory_reserved FROM orders WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, orderID).
		Scan(&storeID, &customerID, &orderNo, &totalCents, &paidCents, &orderStatus, &paymentStatus, &settlementMode, &inventoryReserved)
	if err != nil {
		return orderBalancePaymentResult{}, err
	}
	remaining := totalCents - paidCents
	if remaining <= 0 || paymentStatus != "UNPAID" || customerID <= 0 {
		return orderBalancePaymentResult{}, nil
	}
	payable := orderStatus == "PENDING_PAYMENT"
	if settlementMode == "PAY_AFTER" {
		payable = validStatus(orderStatus, "PAID", "ACCEPTED", "PREPARING", "READY")
	}
	if !payable || inventoryReserved != 1 {
		return orderBalancePaymentResult{}, nil
	}
	var matched int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM customers WHERE tenant_id=? AND id=?
		AND status='ACTIVE' AND deleted_at IS NULL`, tenantID, sessionCustomerID).Scan(&matched); err != nil {
		return orderBalancePaymentResult{}, err
	}
	if matched != 1 || customerID != sessionCustomerID {
		return orderBalancePaymentResult{}, nil
	}
	deductionOrder := "BONUS_FIRST"
	err = tx.QueryRowContext(ctx, "SELECT deduction_order FROM stored_value_settings WHERE tenant_id=? FOR UPDATE", tenantID).
		Scan(&deductionOrder)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return orderBalancePaymentResult{}, err
	}
	var existing orderBalancePaymentResult
	err = tx.QueryRowContext(ctx, `SELECT id,amount_cents FROM order_balance_payments WHERE tenant_id=? AND order_id=? FOR UPDATE`,
		tenantID, orderID).Scan(&existing.ID, &existing.AmountCents)
	if err == nil {
		existing.Reference = fmt.Sprintf("BALANCE-%d", existing.ID)
		existing.RemainingCents = remaining
		existing.FullyPaid = paymentStatus == "PAID" || remaining == 0
		if commitErr := tx.Commit(); commitErr != nil {
			return orderBalancePaymentResult{}, commitErr
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return orderBalancePaymentResult{}, err
	}
	var principal, bonus int64
	err = tx.QueryRowContext(ctx, `SELECT principal_cents,bonus_cents FROM balance_accounts
		WHERE tenant_id=? AND customer_id=? FOR UPDATE`, tenantID, customerID).Scan(&principal, &bonus)
	if errors.Is(err, sql.ErrNoRows) {
		return orderBalancePaymentResult{}, nil
	}
	if err != nil {
		return orderBalancePaymentResult{}, err
	}
	available := principal + bonus
	if available < remaining {
		return orderBalancePaymentResult{}, nil
	}
	useAmount := remaining
	principalUse, bonusUse := allocateOrderBalance(principal, bonus, useAmount, deductionOrder)
	reference := "ORDERBAL:" + int64String(orderID)
	if bonusUse > 0 {
		if _, _, err = applyBalanceDeltaTx(ctx, tx, tenantID, customerID, "BONUS", -bonusUse, "PAYMENT", "ORDER", orderNo,
			reference+":bonus", 0, "订单余额支付"); err != nil {
			return orderBalancePaymentResult{}, err
		}
	}
	if principalUse > 0 {
		if _, _, err = applyBalanceDeltaTx(ctx, tx, tenantID, customerID, "PRINCIPAL", -principalUse, "PAYMENT", "ORDER", orderNo,
			reference+":principal", 0, "订单余额支付"); err != nil {
			return orderBalancePaymentResult{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO order_balance_payments(tenant_id,store_id,order_id,customer_id,
		principal_cents,bonus_cents,amount_cents,status) VALUES(?,?,?,?,?,?,?,'APPLIED')`,
		tenantID, storeID, orderID, customerID, principalUse, bonusUse, useAmount)
	if err != nil {
		return orderBalancePaymentResult{}, err
	}
	balancePaymentID, _ := result.LastInsertId()
	remaining -= useAmount
	targetStatus := "PAID"
	if settlementMode == "PAY_AFTER" {
		targetStatus = "COMPLETED"
	}
	now := time.Now()
	if _, err = tx.ExecContext(ctx, `UPDATE orders SET status=?,payment_status='PAID',inventory_reserved=0,
		stock_reserved_at=NULL,paid_cents=total_cents,paid_at=?,completed_at=IF(?='COMPLETED',?,completed_at),updated_at=NOW(3)
		WHERE id=? AND tenant_id=? AND payment_status='UNPAID'`, targetStatus, now, targetStatus, now, orderID, tenantID); err != nil {
		return orderBalancePaymentResult{}, err
	}
	if err = useOrderCoupon(ctx, tx, tenantID, orderID); err != nil {
		return orderBalancePaymentResult{}, err
	}
	if err = enqueuePrintOutboxWith(ctx, tx, tenantID, storeID, orderID, "PAYMENT_SUCCESS",
		fmt.Sprintf("order-balance-payment:%d", balancePaymentID), 0, ""); err != nil {
		return orderBalancePaymentResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return orderBalancePaymentResult{}, err
	}
	return orderBalancePaymentResult{
		ID: balancePaymentID, AmountCents: useAmount, RemainingCents: remaining,
		Reference: fmt.Sprintf("BALANCE-%d", balancePaymentID), FullyPaid: true,
	}, nil
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func allocateOrderBalance(principal, bonus, amount int64, deductionOrder string) (principalUse, bonusUse int64) {
	if amount <= 0 {
		return 0, 0
	}
	if deductionOrder == "PRINCIPAL_FIRST" {
		principalUse = minInt64(principal, amount)
		bonusUse = minInt64(bonus, amount-principalUse)
		return principalUse, bonusUse
	}
	bonusUse = minInt64(bonus, amount)
	principalUse = minInt64(principal, amount-bonusUse)
	return principalUse, bonusUse
}
