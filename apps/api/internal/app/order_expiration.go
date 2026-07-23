package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var errOrderNotPayable = errors.New("order is not pending payment")

// StartOrderExpirationWorker releases inventory held by abandoned unpaid
// orders. Stores that allow late payment keep the order open after release;
// the next payment attempt must reserve the inventory again before a provider
// payment is created. Stores that disallow late payment are closed.
func (s *Server) StartOrderExpirationWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.reconcileExpiredOrderReservations(ctx)
			}
		}
	}()
}

func (s *Server) reconcileExpiredOrderReservations(ctx context.Context) {
	rows, err := s.DB.QueryContext(ctx, `SELECT o.tenant_id,o.id
		FROM orders o
		JOIN stores st ON st.id=o.store_id AND st.tenant_id=o.tenant_id
		WHERE o.status='PENDING_PAYMENT' AND o.payment_status='UNPAID'
		  AND o.inventory_reserved=1 AND o.stock_reserved_at IS NOT NULL
		  AND DATE_ADD(o.stock_reserved_at, INTERVAL st.payment_timeout_minutes MINUTE)<=NOW(3)
		ORDER BY o.stock_reserved_at,o.id LIMIT 100`)
	if err != nil {
		s.Logger.Error("list expired order reservations", "error", err)
		return
	}
	type candidate struct{ tenantID, orderID int64 }
	candidates := make([]candidate, 0, 100)
	for rows.Next() {
		var item candidate
		if scanErr := rows.Scan(&item.tenantID, &item.orderID); scanErr != nil {
			rows.Close()
			s.Logger.Error("scan expired order reservation", "error", scanErr)
			return
		}
		candidates = append(candidates, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		s.Logger.Error("iterate expired order reservations", "error", err)
		return
	}
	rows.Close()

	for _, item := range candidates {
		conn, release, lockErr := s.acquirePaymentOrderLock(ctx, item.tenantID, item.orderID)
		if lockErr != nil {
			s.Logger.Warn("lock expired order reservation", "order_id", item.orderID, "error", lockErr)
			continue
		}
		expired, allowLate, expireErr := s.expireOrderReservationLocked(ctx, conn, item.tenantID, item.orderID)
		release()
		if errors.Is(expireErr, errPaymentAlreadyPaid) {
			continue
		}
		if expireErr != nil {
			s.Logger.Error("expire order reservation", "order_id", item.orderID, "error", expireErr)
			continue
		}
		if expired {
			action := "closed"
			if allowLate {
				action = "inventory_released_late_payment_allowed"
			}
			s.Logger.Info("expired unpaid order reservation", "order_id", item.orderID, "action", action)
		}
	}
}

// expireOrderReservationLocked must run while the per-order payment named lock
// is held. It closes an in-flight provider payment before releasing stock, so a
// payable provider session can never outlive its local inventory reservation.
func (s *Server) expireOrderReservationLocked(ctx context.Context, conn *sql.Conn, tenantID, orderID int64) (bool, bool, error) {
	var allowLate int
	err := conn.QueryRowContext(ctx, `SELECT st.allow_late_payment
		FROM orders o JOIN stores st ON st.id=o.store_id AND st.tenant_id=o.tenant_id
		WHERE o.id=? AND o.tenant_id=? AND o.status='PENDING_PAYMENT' AND o.payment_status='UNPAID'
		  AND o.inventory_reserved=1 AND o.stock_reserved_at IS NOT NULL
		  AND DATE_ADD(o.stock_reserved_at, INTERVAL st.payment_timeout_minutes MINUTE)<=NOW(3)`, orderID, tenantID).Scan(&allowLate)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if err = s.closePendingPaymentLocked(ctx, conn, tenantID, orderID); err != nil {
		return false, allowLate == 1, err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return false, allowLate == 1, err
	}
	defer tx.Rollback()
	var status, paymentStatus string
	var inventoryReserved, stillExpired int
	err = tx.QueryRowContext(ctx, `SELECT o.status,o.payment_status,o.inventory_reserved,st.allow_late_payment,
		(o.stock_reserved_at IS NOT NULL AND DATE_ADD(o.stock_reserved_at, INTERVAL st.payment_timeout_minutes MINUTE)<=NOW(3))
		FROM orders o JOIN stores st ON st.id=o.store_id AND st.tenant_id=o.tenant_id
		WHERE o.id=? AND o.tenant_id=? FOR UPDATE`, orderID, tenantID).
		Scan(&status, &paymentStatus, &inventoryReserved, &allowLate, &stillExpired)
	if err != nil {
		return false, allowLate == 1, err
	}
	if status != "PENDING_PAYMENT" || paymentStatus != "UNPAID" || inventoryReserved != 1 || stillExpired != 1 {
		if err = tx.Commit(); err != nil {
			return false, allowLate == 1, err
		}
		return false, allowLate == 1, nil
	}
	if err = restoreOrderInventory(ctx, tx, tenantID, orderID); err != nil {
		return false, allowLate == 1, err
	}
	if allowLate == 1 {
		_, err = tx.ExecContext(ctx, `UPDATE orders SET inventory_reserved=0,stock_reserved_at=NULL
			WHERE id=? AND tenant_id=? AND status='PENDING_PAYMENT' AND payment_status='UNPAID' AND inventory_reserved=1`, orderID, tenantID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE orders SET status='CLOSED',inventory_reserved=0,stock_reserved_at=NULL,closed_at=NOW(3)
			WHERE id=? AND tenant_id=? AND status='PENDING_PAYMENT' AND payment_status='UNPAID' AND inventory_reserved=1`, orderID, tenantID)
		if err == nil {
			err = releaseOrderCoupon(ctx, tx, tenantID, orderID)
		}
	}
	if err != nil {
		return false, allowLate == 1, err
	}
	if err = tx.Commit(); err != nil {
		return false, allowLate == 1, err
	}
	return true, allowLate == 1, nil
}

// ensureOrderStockReservationLocked reserves every order item atomically. It is
// used by late-payment attempts after the expiration worker has returned the
// original reservation to inventory.
func ensureOrderStockReservationLocked(ctx context.Context, conn *sql.Conn, tenantID, orderID int64) (bool, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var status, paymentStatus string
	var storeID int64
	var inventoryReserved int
	if err = tx.QueryRowContext(ctx, `SELECT status,payment_status,inventory_reserved,store_id FROM orders
		WHERE id=? AND tenant_id=? FOR UPDATE`, orderID, tenantID).Scan(&status, &paymentStatus, &inventoryReserved, &storeID); err != nil {
		return false, err
	}
	if status != "PENDING_PAYMENT" || paymentStatus != "UNPAID" {
		return false, errOrderNotPayable
	}
	if inventoryReserved == 1 {
		if err = tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}
	type stockItem struct {
		skuID     int64
		quantity  int
		available int
	}
	rows, err := tx.QueryContext(ctx, `SELECT oi.sku_id,oi.quantity,
		CASE WHEN s.id IS NOT NULL AND s.status='ACTIVE' AND s.deleted_at IS NULL
		  AND p.id IS NOT NULL AND p.status='ACTIVE' AND p.deleted_at IS NULL
		  AND c.id IS NOT NULL AND c.status='ACTIVE' AND c.deleted_at IS NULL
		  AND i.sku_id IS NOT NULL THEN 1 ELSE 0 END AS available
		FROM order_items oi
		LEFT JOIN skus s ON s.id=oi.sku_id AND s.tenant_id=oi.tenant_id AND s.store_id=?
		LEFT JOIN products p ON p.id=oi.product_id AND p.id=s.product_id AND p.tenant_id=oi.tenant_id AND p.store_id=?
		LEFT JOIN categories c ON c.id=p.category_id AND c.tenant_id=p.tenant_id AND c.store_id=p.store_id
		LEFT JOIN inventory i ON i.sku_id=s.id AND i.tenant_id=s.tenant_id AND i.store_id=s.store_id
		WHERE oi.tenant_id=? AND oi.order_id=? ORDER BY oi.sku_id,oi.id FOR UPDATE`, storeID, storeID, tenantID, orderID)
	if err != nil {
		return false, err
	}
	items := make([]stockItem, 0)
	for rows.Next() {
		var item stockItem
		if err = rows.Scan(&item.skuID, &item.quantity, &item.available); err != nil {
			rows.Close()
			return false, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	if len(items) == 0 {
		return false, fmt.Errorf("order %d has no items", orderID)
	}
	for _, item := range items {
		if item.available != 1 {
			return false, errInsufficientStock
		}
		if err = reserveStock(ctx, tx, tenantID, item.skuID, item.quantity); err != nil {
			return false, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE orders SET inventory_reserved=1,stock_reserved_at=NOW(3)
		WHERE id=? AND tenant_id=? AND status='PENDING_PAYMENT' AND payment_status='UNPAID' AND inventory_reserved=0`, orderID, tenantID)
	if err != nil {
		return false, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return false, errors.New("order reservation changed concurrently")
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func restoreOrderInventory(ctx context.Context, tx *sql.Tx, tenantID, orderID int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT sku_id,quantity FROM order_items
		WHERE tenant_id=? AND order_id=? ORDER BY sku_id,id`, tenantID, orderID)
	if err != nil {
		return err
	}
	type stockItem struct {
		skuID    int64
		quantity int
	}
	items := make([]stockItem, 0)
	for rows.Next() {
		var item stockItem
		if err = rows.Scan(&item.skuID, &item.quantity); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		if _, err = tx.ExecContext(ctx, "UPDATE inventory SET stock=stock+? WHERE sku_id=? AND tenant_id=?", item.quantity, item.skuID, tenantID); err != nil {
			return err
		}
	}
	return nil
}
