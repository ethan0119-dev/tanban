package app

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/go-chi/chi/v5"
)

func TestAllocateOrderBalanceHonorsConfiguredBucketOrder(t *testing.T) {
	tests := []struct {
		name, order              string
		principal, bonus, total  int64
		wantPrincipal, wantBonus int64
	}{
		{name: "bonus first", order: "BONUS_FIRST", principal: 500, bonus: 200, total: 600, wantPrincipal: 400, wantBonus: 200},
		{name: "principal first", order: "PRINCIPAL_FIRST", principal: 500, bonus: 200, total: 600, wantPrincipal: 500, wantBonus: 100},
		{name: "caps at available", order: "BONUS_FIRST", principal: 100, bonus: 50, total: 300, wantPrincipal: 100, wantBonus: 50},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			principal, bonus := allocateOrderBalance(test.principal, test.bonus, test.total, test.order)
			if principal != test.wantPrincipal || bonus != test.wantBonus {
				t.Fatalf("got principal=%d bonus=%d, want principal=%d bonus=%d", principal, bonus, test.wantPrincipal, test.wantBonus)
			}
		})
	}
}

func TestApplyPaidBalanceCreditTxCreditsCapturedMoney(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT id,customer_id").WithArgs(int64(9), "public-sv:principal").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO balance_accounts").WithArgs(int64(9), int64(21)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT principal_cents,bonus_cents").WithArgs(int64(9), int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{"principal_cents", "bonus_cents"}).AddRow(300, 40))
	mock.ExpectExec("INSERT INTO balance_ledger").
		WithArgs(int64(9), int64(21), "PRINCIPAL", int64(500), int64(300), int64(800), "RECHARGE", "STORED_VALUE", "SV-1", "public-sv:principal", "小程序储值充值").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE balance_accounts SET principal_cents=?,version=version+1 WHERE tenant_id=? AND customer_id=?")).
		WithArgs(int64(800), int64(9), int64(21)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err = applyPaidBalanceCreditTx(context.Background(), tx, 9, 21, "PRINCIPAL", 500, "STORED_VALUE", "SV-1", "public-sv:principal", "小程序储值充值"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateStoredValueSettingsAllowsMiniappRecharge(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("INSERT INTO stored_value_settings").
		WithArgs(int64(9), true, int64(100), int64(100000), int64(500000), "BONUS_FIRST", "MANUAL_REVIEW", "", true, int64(2)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	router.Put("/stored-value-settings", server.updateStoredValueSettings)
	request := httptest.NewRequest(http.MethodPut, "/stored-value-settings", bytes.NewBufferString(`{
		"enabled":true,"min_recharge_cents":100,"max_recharge_cents":100000,"max_balance_cents":500000,
		"deduction_order":"BONUS_FIRST","refund_policy":"MANUAL_REVIEW","agreement_url":"","show_in_miniapp":true
	}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 2, TenantID: 9, Role: RoleMerchantManager}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
