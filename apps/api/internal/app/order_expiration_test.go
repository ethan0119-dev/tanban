package app

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
)

func TestExpireOrderReservationReleasesInventoryAndKeepsLatePayableOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT st.allow_late_payment")).
		WithArgs(int64(8), int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"allow_late_payment"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,provider,provider_order_no,status FROM payment_transactions")).
		WithArgs(int64(4), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "provider", "provider_order_no", "status"}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT o.status,o.payment_status,o.inventory_reserved,st.allow_late_payment")).
		WithArgs(int64(8), int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "payment_status", "inventory_reserved", "allow_late_payment", "expired"}).AddRow("PENDING_PAYMENT", "UNPAID", 1, 1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT sku_id,quantity FROM order_items")).
		WithArgs(int64(4), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"sku_id", "quantity"}).AddRow(19, 2).AddRow(20, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE inventory SET stock=stock+? WHERE sku_id=? AND tenant_id=?")).
		WithArgs(2, int64(19), int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE inventory SET stock=stock+? WHERE sku_id=? AND tenant_id=?")).
		WithArgs(1, int64(20), int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE orders SET inventory_reserved=0,stock_reserved_at=NULL")).
		WithArgs(int64(8), int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	expired, allowLate, err := server.expireOrderReservationLocked(context.Background(), conn, 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !expired || !allowLate {
		t.Fatalf("expired=%v allowLate=%v", expired, allowLate)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureLatePaymentReservationIsAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status,payment_status,inventory_reserved,store_id FROM orders")).
		WithArgs(int64(8), int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "payment_status", "inventory_reserved", "store_id"}).AddRow("PENDING_PAYMENT", "UNPAID", 0, 6))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT oi.sku_id,oi.quantity")).
		WithArgs(int64(6), int64(6), int64(4), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"sku_id", "quantity", "available"}).AddRow(19, 2, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE inventory SET stock=stock-? WHERE sku_id=? AND tenant_id=? AND stock>=?")).
		WithArgs(2, int64(19), int64(4), 2).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE orders SET inventory_reserved=1,stock_reserved_at=NOW(3)")).
		WithArgs(int64(8), int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	reserved, err := ensureOrderStockReservationLocked(context.Background(), conn, 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !reserved {
		t.Fatal("expected a fresh inventory reservation")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureLatePaymentReservationRejectsDisabledCatalog(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status,payment_status,inventory_reserved,store_id FROM orders")).
		WithArgs(int64(8), int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "payment_status", "inventory_reserved", "store_id"}).AddRow("PENDING_PAYMENT", "UNPAID", 0, 6))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT oi.sku_id,oi.quantity")).
		WithArgs(int64(6), int64(6), int64(4), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"sku_id", "quantity", "available"}).AddRow(19, 2, 0))
	mock.ExpectRollback()

	if _, err = ensureOrderStockReservationLocked(context.Background(), conn, 4, 8); !errors.Is(err, errInsufficientStock) {
		t.Fatalf("expected unavailable catalog rejection, got %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPaidCallbackWithoutReservationBecomesPaymentException(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	paidAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT p.id,p.tenant_id,p.store_id,p.order_id,p.status,o.status,o.payment_status,o.inventory_reserved")).
		WithArgs("mock", "MOCK-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "store_id", "order_id", "payment_status", "order_status", "order_payment_status", "inventory_reserved"}).
			AddRow(2, 4, 5, 8, "PENDING", "PENDING_PAYMENT", "UNPAID", 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,provider,provider_order_no,status FROM payment_transactions")).
		WithArgs(int64(4), int64(8), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "provider", "provider_order_no", "status"}))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE payment_transactions SET status='SUCCESS',paid_at=? WHERE id=?")).
		WithArgs(paidAt, int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE orders SET status=?,payment_status='PAID',inventory_reserved=0,stock_reserved_at=NULL,paid_cents=total_cents,paid_at=? WHERE id=?")).
		WithArgs("PAYMENT_EXCEPTION", paidAt, int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE customer_coupons SET status='USED',used_at=NOW(3)")).
		WithArgs(int64(4), int64(8)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err = server.markPaymentPaidLocked(context.Background(), conn, "mock", "MOCK-1", paidAt); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSecondSuccessfulPaymentAttemptBecomesPaymentException(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	paidAt := time.Date(2026, 7, 20, 12, 5, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT p.id,p.tenant_id,p.store_id,p.order_id,p.status,o.status,o.payment_status,o.inventory_reserved")).
		WithArgs("mock", "MOCK-SECOND").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "store_id", "order_id", "payment_status", "order_status", "order_payment_status", "inventory_reserved"}).
			AddRow(3, 4, 5, 8, "PENDING", "PAID", "PAID", 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,provider,provider_order_no,status FROM payment_transactions")).
		WithArgs(int64(4), int64(8), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "provider", "provider_order_no", "status"}))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE payment_transactions SET status='SUCCESS',paid_at=? WHERE id=?")).
		WithArgs(paidAt, int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE orders SET status='PAYMENT_EXCEPTION' WHERE id=?")).
		WithArgs(int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err = server.markPaymentPaidLocked(context.Background(), conn, "mock", "MOCK-SECOND", paidAt); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLateSuccessClosesNewerPendingAttemptAndFlagsException(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	paidAt := time.Date(2026, 7, 20, 12, 10, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT p.id,p.tenant_id,p.store_id,p.order_id,p.status,o.status,o.payment_status,o.inventory_reserved")).
		WithArgs("mock", "MOCK-OLD").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "store_id", "order_id", "payment_status", "order_status", "order_payment_status", "inventory_reserved"}).
			AddRow(2, 4, 5, 8, "CLOSED", "PENDING_PAYMENT", "UNPAID", 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,provider,provider_order_no,status FROM payment_transactions")).
		WithArgs(int64(4), int64(8), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "provider", "provider_order_no", "status"}).AddRow(3, "mock", "MOCK-NEW", "PENDING"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE payment_transactions SET status='CLOSED' WHERE id=? AND tenant_id=? AND status='PENDING'")).
		WithArgs(int64(3), int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE payment_transactions SET status='SUCCESS',paid_at=? WHERE id=?")).
		WithArgs(paidAt, int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE orders SET status=?,payment_status='PAID',inventory_reserved=0,stock_reserved_at=NULL,paid_cents=total_cents,paid_at=? WHERE id=?")).
		WithArgs("PAYMENT_EXCEPTION", paidAt, int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE customer_coupons SET status='USED',used_at=NOW(3)")).
		WithArgs(int64(4), int64(8)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err = server.markPaymentPaidLocked(context.Background(), conn, "mock", "MOCK-OLD", paidAt); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
