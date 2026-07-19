package app

import (
	"context"
	"log/slog"
	"math"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
)

func TestNonNegativeSumWithinRejectsOverflowAndLimitBreaches(t *testing.T) {
	tests := []struct {
		name   string
		limit  int64
		values []int64
		want   bool
	}{
		{name: "inside", limit: 100, values: []int64{20, 30, 50}, want: true},
		{name: "over limit", limit: 100, values: []int64{60, 41}},
		{name: "negative", limit: 100, values: []int64{10, -1}},
		{name: "max int cannot overflow", limit: 100, values: []int64{1, math.MaxInt64}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nonNegativeSumWithin(test.limit, test.values...); got != test.want {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
}

func TestFinalizeRefundRejectsOverflowBeforeMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT r.tenant_id,r.store_id,r.order_id,r.amount_cents,r.reason,r.status,r.created_by,o.paid_cents,o.refunded_cents")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "store_id", "order_id", "amount_cents", "reason", "status", "created_by", "paid_cents", "refunded_cents"}).
			AddRow(2, 3, 4, int64(math.MaxInt64), "overflow", "PENDING", 5, 100, 1))
	mock.ExpectRollback()

	if err = server.finalizeRefund(context.Background(), 7, "MOCKREF-7"); err == nil {
		t.Fatal("overflowing refund must be rejected")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
