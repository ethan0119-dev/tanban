package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEnqueuePrintOutboxRecordsOnlyAnIdempotentBusinessFact(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	insert := regexp.QuoteMeta("INSERT INTO print_outbox(tenant_id,store_id,order_id,event_type,dedupe_key,actor_id,extra_text,status,available_at)")
	mock.ExpectExec(insert).
		WithArgs(int64(2), int64(5), int64(11), "PAYMENT_SUCCESS", "PAYMENT:41", int64(0), "").
		WillReturnResult(sqlmock.NewResult(71, 1))
	if err = enqueuePrintOutboxWith(context.Background(), db, 2, 5, 11, "payment_success", paymentPrintDedupeKey(41), 0, ""); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProcessPrintOutboxCommitsGeneratedJobsAndDoneTogether(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := &Server{DB: db, Logger: discardLogger()}
	past := time.Now().Add(-time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,store_id,order_id,event_type,actor_id,extra_text,status,attempts,available_at")).
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "store_id", "order_id", "event_type", "actor_id", "extra_text", "status", "attempts", "available_at"}).
			AddRow(17, 2, 5, 11, "PAYMENT_SUCCESS", 9, "fact", "PENDING", 0, past))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE print_outbox SET status='PROCESSING',attempts=?")).
		WithArgs(1, int64(17)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("SAVEPOINT print_outbox_enqueue")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO print_jobs(test_marker) VALUES(?)")).WithArgs("atomic").WillReturnResult(sqlmock.NewResult(99, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE print_outbox") + `\s+` + regexp.QuoteMeta("SET status='DONE',last_error='',processed_at=NOW(3)")).
		WithArgs(int64(17)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	enqueue := func(ctx context.Context, executor sqlQueryExecer, tenantID, storeID, orderID int64, event string, reprint bool, actorID int64, extra string) error {
		if tenantID != 2 || storeID != 5 || orderID != 11 || event != "PAYMENT_SUCCESS" || reprint || actorID != 9 || extra != "fact" {
			t.Fatalf("unexpected event payload tenant=%d store=%d order=%d event=%s reprint=%v actor=%d extra=%q", tenantID, storeID, orderID, event, reprint, actorID, extra)
		}
		_, execErr := executor.ExecContext(ctx, "INSERT INTO print_jobs(test_marker) VALUES(?)", "atomic")
		return execErr
	}
	if err = server.processPrintOutboxEventWith(context.Background(), 17, enqueue); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProcessPrintOutboxRollsBackPartialJobsAndSchedulesRetry(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := &Server{DB: db, Logger: discardLogger()}
	past := time.Now().Add(-time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,store_id,order_id,event_type,actor_id,extra_text,status,attempts,available_at")).
		WithArgs(int64(18)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "store_id", "order_id", "event_type", "actor_id", "extra_text", "status", "attempts", "available_at"}).
			AddRow(18, 2, 5, 12, "REFUND", 9, "refund", "PENDING", 2, past))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE print_outbox SET status='PROCESSING',attempts=?")).
		WithArgs(3, int64(18)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("SAVEPOINT print_outbox_enqueue")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO print_jobs(test_marker) VALUES(?)")).WithArgs("partial").WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec(regexp.QuoteMeta("ROLLBACK TO SAVEPOINT print_outbox_enqueue")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE print_outbox")+`\s+`+regexp.QuoteMeta("SET status='PENDING',available_at=?,last_error=?,processed_at=NULL")).
		WithArgs(sqlmock.AnyArg(), "malformed print template", int64(18)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	enqueue := func(ctx context.Context, executor sqlQueryExecer, _, _, _ int64, _ string, _ bool, _ int64, _ string) error {
		if _, execErr := executor.ExecContext(ctx, "INSERT INTO print_jobs(test_marker) VALUES(?)", "partial"); execErr != nil {
			return execErr
		}
		return errors.New("malformed print template")
	}
	if err = server.processPrintOutboxEventWith(context.Background(), 18, enqueue); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProcessPrintOutboxMarksDeadAtRetryLimit(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := &Server{DB: db, Logger: discardLogger()}
	past := time.Now().Add(-time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,store_id,order_id,event_type,actor_id,extra_text,status,attempts,available_at")).
		WithArgs(int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "store_id", "order_id", "event_type", "actor_id", "extra_text", "status", "attempts", "available_at"}).
			AddRow(19, 2, 5, 13, "ORDER_CREATED", 0, "", "PENDING", printOutboxMaxAttempts-1, past))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE print_outbox SET status='PROCESSING',attempts=?")).
		WithArgs(printOutboxMaxAttempts, int64(19)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("SAVEPOINT print_outbox_enqueue")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("ROLLBACK TO SAVEPOINT print_outbox_enqueue")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE print_outbox")+`\s+`+regexp.QuoteMeta("SET status='DEAD',last_error=?,processed_at=NOW(3)")).
		WithArgs("printer configuration is invalid", int64(19)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	enqueue := func(context.Context, sqlQueryExecer, int64, int64, int64, string, bool, int64, string) error {
		return errors.New("printer configuration is invalid")
	}
	if err = server.processPrintOutboxEventWith(context.Background(), 19, enqueue); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPaymentSuccessCommitsFactWithOutboxInsteadOfRenderingTemplates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := &Server{DB: db, Logger: discardLogger()}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	paidAt := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT p.id,p.tenant_id,p.store_id,p.order_id,p.status,o.status,o.payment_status,o.inventory_reserved")).
		WithArgs("mock", "MOCK-PAID").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "store_id", "order_id", "payment_status", "order_status", "order_payment_status", "inventory_reserved"}).
			AddRow(41, 2, 5, 11, "PENDING", "PENDING_PAYMENT", "UNPAID", 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,provider,provider_order_no,status FROM payment_transactions")).
		WithArgs(int64(2), int64(11), int64(41)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "provider", "provider_order_no", "status"}))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE payment_transactions SET status='SUCCESS',paid_at=? WHERE id=?")).
		WithArgs(paidAt, int64(41)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE orders SET status=?,payment_status='PAID',inventory_reserved=0,stock_reserved_at=NULL,paid_cents=total_cents,paid_at=? WHERE id=?")).
		WithArgs("PAID", paidAt, int64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE customer_coupons SET status='USED',used_at=NOW(3)")).
		WithArgs(int64(2), int64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO print_outbox(tenant_id,store_id,order_id,event_type,dedupe_key,actor_id,extra_text,status,available_at)")).
		WithArgs(int64(2), int64(5), int64(11), "PAYMENT_SUCCESS", "PAYMENT:41", int64(0), "").
		WillReturnResult(sqlmock.NewResult(17, 1))
	mock.ExpectCommit()

	if err = server.markPaymentPaidLocked(context.Background(), conn, "mock", "MOCK-PAID", paidAt); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRefundSuccessCommitsFactWithOutboxInsteadOfRenderingTemplates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := &Server{DB: db, Logger: discardLogger()}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT r.tenant_id,r.store_id,r.order_id,r.amount_cents,r.reason,r.status,r.created_by,o.paid_cents,o.refunded_cents")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "store_id", "order_id", "amount_cents", "reason", "status", "created_by", "paid_cents", "refunded_cents"}).
			AddRow(2, 5, 11, 300, "少做一杯", "PENDING", 9, 1200, 0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE refunds SET status='SUCCESS',provider_refund_no=?,last_error='' WHERE id=? AND status='PENDING'")).
		WithArgs("RF-7", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE orders SET refunded_cents=?,payment_status=?,status=IF(?='REFUNDED','REFUNDED',status) WHERE id=? AND tenant_id=?")).
		WithArgs(int64(300), "PARTIALLY_REFUNDED", "PARTIALLY_REFUNDED", int64(11), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO print_outbox(tenant_id,store_id,order_id,event_type,dedupe_key,actor_id,extra_text,status,available_at)")).
		WithArgs(int64(2), int64(5), int64(11), "REFUND", "REFUND:7", int64(9), "退款 300 分：少做一杯").
		WillReturnResult(sqlmock.NewResult(18, 1))
	mock.ExpectCommit()

	if err = server.finalizeRefund(context.Background(), 7, "RF-7"); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

var _ sqlQueryExecer = (*sql.Tx)(nil)
