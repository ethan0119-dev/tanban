package app

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLoadLatestPaymentAttemptReusesHistoricalChannelAfterTenantSwitch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(latestPaymentAttemptQuery)).
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "provider", "provider_order_no", "status", "raw_response"}).
			AddRow(29, "wechat_partner", "wx-old-pending", "PENDING", `{"timeStamp":"1"}`))

	attempt, err := loadLatestPaymentAttempt(context.Background(), db, 7, 11)
	if err != nil {
		t.Fatalf("load latest payment attempt: %v", err)
	}
	if attempt.ID != 29 || attempt.Provider != "wechat_partner" || attempt.Status != "PENDING" {
		t.Fatalf("unexpected historical attempt: %+v", attempt)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
