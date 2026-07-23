package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/go-chi/chi/v5"
)

func TestApplyBalanceDeltaTxAppendsLedgerAndProjection(t *testing.T) {
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
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,customer_id,account_bucket,delta_cents,balance_before_cents,balance_after_cents,entry_type,business_type,business_no,remark FROM balance_ledger WHERE tenant_id=? AND idempotency_key=?")).
		WithArgs(int64(9), "adjust-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE")).
		WithArgs(int64(21), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))
	mock.ExpectExec("INSERT INTO balance_accounts").WithArgs(int64(9), int64(21), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT principal_cents,bonus_cents FROM balance_accounts WHERE tenant_id=? AND customer_id=? FOR UPDATE")).
		WithArgs(int64(9), int64(21)).WillReturnRows(sqlmock.NewRows([]string{"principal_cents", "bonus_cents"}).AddRow(500, 100))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT max_balance_cents FROM stored_value_settings WHERE tenant_id=?")).
		WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"max_balance_cents"}).AddRow(1000000))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO balance_ledger(tenant_id,customer_id,account_bucket,delta_cents,balance_before_cents,balance_after_cents,entry_type,business_type,business_no,idempotency_key,operator_user_id,remark) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)")).
		WithArgs(int64(9), int64(21), "PRINCIPAL", int64(250), int64(500), int64(750), "ADJUSTMENT", "MANUAL_ADJUSTMENT", "BA-1", "adjust-1", int64(7), "cash correction").
		WillReturnResult(sqlmock.NewResult(88, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE balance_accounts SET principal_cents=?,version=version+1 WHERE tenant_id=? AND customer_id=?")).
		WithArgs(int64(750), int64(9), int64(21)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	entry, replayed, err := applyBalanceDeltaTx(context.Background(), tx, 9, 21, "PRINCIPAL", 250, "ADJUSTMENT", "MANUAL_ADJUSTMENT", "BA-1", "adjust-1", 7, "cash correction")
	if err != nil {
		t.Fatalf("apply balance: %v", err)
	}
	if replayed || entry.ID != 88 || entry.BeforeCents != 500 || entry.AfterCents != 750 {
		t.Fatalf("unexpected entry: %#v replayed=%v", entry, replayed)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyBalanceDeltaTxRejectsNegativeBalance(t *testing.T) {
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
	mock.ExpectQuery("SELECT id,customer_id").WithArgs(int64(3), "debit-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE")).
		WithArgs(int64(4), int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(4))
	mock.ExpectExec("INSERT INTO balance_accounts").WithArgs(int64(3), int64(4), int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT principal_cents,bonus_cents").WithArgs(int64(3), int64(4)).WillReturnRows(sqlmock.NewRows([]string{"principal_cents", "bonus_cents"}).AddRow(20, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT max_balance_cents FROM stored_value_settings WHERE tenant_id=?")).
		WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"max_balance_cents"}).AddRow(1000000))
	mock.ExpectRollback()

	_, _, err = applyBalanceDeltaTx(context.Background(), tx, 3, 4, "PRINCIPAL", -21, "ADJUSTMENT", "MANUAL_ADJUSTMENT", "BA-2", "debit-1", 1, "test")
	if !errors.Is(err, errInsufficientBalance) {
		t.Fatalf("expected insufficient balance, got %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyBalanceDeltaTxIdempotentReplayDoesNotWrite(t *testing.T) {
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
	mock.ExpectQuery("SELECT id,customer_id").WithArgs(int64(5), "same-key").WillReturnRows(sqlmock.NewRows([]string{"id", "customer_id", "account_bucket", "delta_cents", "balance_before_cents", "balance_after_cents", "entry_type", "business_type", "business_no", "remark"}).AddRow(17, 8, "BONUS", 100, 0, 100, "RECHARGE_GIFT", "STORED_VALUE", "SV-1", "gift"))
	mock.ExpectCommit()

	entry, replayed, err := applyBalanceDeltaTx(context.Background(), tx, 5, 8, "BONUS", 100, "RECHARGE_GIFT", "STORED_VALUE", "SV-1", "same-key", 1, "gift")
	if err != nil || !replayed || entry.ID != 17 {
		t.Fatalf("entry=%#v replayed=%v err=%v", entry, replayed, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyBalanceDeltaTxRejectsIdempotencyKeyReuse(t *testing.T) {
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
	mock.ExpectQuery("SELECT id,customer_id").WithArgs(int64(5), "reused-key").WillReturnRows(sqlmock.NewRows([]string{"id", "customer_id", "account_bucket", "delta_cents", "balance_before_cents", "balance_after_cents", "entry_type", "business_type", "business_no", "remark"}).AddRow(17, 8, "PRINCIPAL", 100, 0, 100, "ADJUSTMENT", "MANUAL_ADJUSTMENT", "BA-1", "first reason"))
	mock.ExpectRollback()

	_, _, err = applyBalanceDeltaTx(context.Background(), tx, 5, 8, "PRINCIPAL", 200, "ADJUSTMENT", "MANUAL_ADJUSTMENT", "BA-2", "reused-key", 1, "different reason")
	if !errors.Is(err, errIdempotencyKeyReused) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMaskPhone(t *testing.T) {
	if got := maskPhone("13800138000"); got != "138****8000" {
		t.Fatalf("got %q", got)
	}
	if got := maskPhone("123"); got != "123" {
		t.Fatalf("short phone changed: %q", got)
	}
}

func TestUpdateCustomerAllowsNoopUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT source_store_id,avatar_url FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL")).
		WithArgs(int64(3), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"source_store_id", "avatar_url"}).AddRow(nil, ""))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE customers SET source_store_id=?,name=?,avatar_url=?,phone=?,source=?,status=?,remark=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL")).
		WithArgs(nil, "张三", "", "13800138000", "MANUAL", "ACTIVE", "", int64(3), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT c.id,c.public_id,c.name").
		WithArgs(int64(3), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "name", "phone", "avatar_url", "source", "status", "remark", "source_store_id", "store_name",
			"member_id", "member_no", "member_status", "growth_value", "level_id", "level_name", "principal_cents", "bonus_cents",
			"order_count", "net_spent_cents", "refunded_cents", "registered_at", "last_seen_at", "joined_at", "expires_at",
		}).AddRow(3, "CU-3", "张三", "13800138000", "", "MANUAL", "ACTIVE", "", nil, "", 0, "", "", 0, 0, "", 0, 0, 0, 0, 0, "2026-07-23 20:00:00", nil, nil, nil))
	mock.ExpectQuery("SELECT t.id,t.name,t.color FROM customer_tag_assignments").
		WithArgs(int64(9), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "color"}))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	router.Put("/customers/{customerID}", server.updateCustomer)
	request := httptest.NewRequest(http.MethodPut, "/customers/3", bytes.NewBufferString(`{"name":"张三","phone":"13800138000","source":"MANUAL","status":"ACTIVE","remark":""}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 2, TenantID: 9, Role: RoleMerchantManager}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMemberLevelOrderAcceptsLegacyAmountField(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id,request_fingerprint FROM member_level_orders").
		WithArgs(int64(9), "level-order-legacy").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id FROM customers").
		WithArgs(int64(3), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery("SELECT name,price_cents FROM member_levels").
		WithArgs(int64(4), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"name", "price_cents"}).AddRow("金卡", 1234))
	mock.ExpectQuery("SELECT id FROM members").
		WithArgs(int64(9), int64(3)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO member_level_orders").
		WithArgs(int64(9), sqlmock.AnyArg(), int64(3), nil, int64(4), sqlmock.AnyArg(), int64(1234), "MANUAL", "RECORDED", "COMPLETED", "", "level-order-legacy", sqlmock.AnyArg(), int64(2), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(21, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	router.Post("/member-level-orders", server.createMemberLevelOrder)
	request := httptest.NewRequest(http.MethodPost, "/member-level-orders", bytes.NewBufferString(`{"customer_id":3,"level_id":4,"amount":12.34,"amount_cents":1234,"payment_method":"MANUAL","status":"COMPLETED"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "level-order-legacy")
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 2, TenantID: 9, Role: RoleMerchantManager}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMemberSummaryUsesTenantWideAggregates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(memberSummaryQuery)).
		WithArgs(int64(9), int64(9), int64(9), int64(9), int64(9), int64(9), int64(9), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"customer_count", "member_count", "balance_cents", "blocked_customer_count",
			"stored_value_principal_cents", "stored_value_gift_cents", "stored_value_customer_count", "active_stored_value_rule_count",
		}).AddRow(205, 88, 32100, 4, 130000, 12000, 51, 6))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.memberRoutes(router)
	request := httptest.NewRequest(http.MethodGet, "/member-summary", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 2, TenantID: 9, Role: RoleMerchantManager}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data memberSummary `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.CustomerCount != 205 || payload.Data.MemberCount != 88 || payload.Data.BalanceCents != 32100 || payload.Data.BlockedCustomerCount != 4 || payload.Data.StoredValuePrincipalCents != 130000 || payload.Data.StoredValueGiftCents != 12000 || payload.Data.StoredValueCustomerCount != 51 || payload.Data.ActiveStoredValueRuleCount != 6 {
		t.Fatalf("unexpected summary: %#v", payload.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveCustomerLocksBalanceAndCommitsAuditAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE")).
		WithArgs(int64(3), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO balance_accounts(tenant_id,customer_id) VALUES(?,?) ON DUPLICATE KEY UPDATE customer_id=VALUES(customer_id)")).
		WithArgs(int64(9), int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT principal_cents,bonus_cents FROM balance_accounts WHERE tenant_id=? AND customer_id=? FOR UPDATE")).
		WithArgs(int64(9), int64(3)).WillReturnRows(sqlmock.NewRows([]string{"principal_cents", "bonus_cents"}).AddRow(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE customers SET status='ARCHIVED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL")).
		WithArgs(int64(3), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.memberRoutes(router)
	request := httptest.NewRequest(http.MethodDelete, "/customers/3", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 1, TenantID: 9, Role: RoleMerchantOwner}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveCustomerRejectsNonzeroBalanceInsideTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM customers").WithArgs(int64(3), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectExec("INSERT INTO balance_accounts").WithArgs(int64(9), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT principal_cents,bonus_cents").WithArgs(int64(9), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"principal_cents", "bonus_cents"}).AddRow(100, 0))
	mock.ExpectRollback()

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.memberRoutes(router)
	request := httptest.NewRequest(http.MethodDelete, "/customers/3", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 1, TenantID: 9, Role: RoleMerchantOwner}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "CUSTOMER_HAS_BALANCE") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMemberRoutesFailClosedForStaffAndManagerMoneyWrites(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.memberRoutes(router)

	staffRequest := httptest.NewRequest(http.MethodGet, "/customers", nil)
	staffRequest = staffRequest.WithContext(context.WithValue(staffRequest.Context(), identityKey{}, identity{UserID: 1, TenantID: 9, Role: RoleMerchantStaff}))
	staffResponse := httptest.NewRecorder()
	router.ServeHTTP(staffResponse, staffRequest)
	if staffResponse.Code != http.StatusForbidden {
		t.Fatalf("staff status=%d body=%s", staffResponse.Code, staffResponse.Body.String())
	}

	managerRequest := httptest.NewRequest(http.MethodPost, "/customers/3/balance-adjustments", bytes.NewBufferString(`{"bucket":"PRINCIPAL","direction":"CREDIT","amount_cents":100,"remark":"test"}`))
	managerRequest.Header.Set("Content-Type", "application/json")
	managerRequest.Header.Set("Idempotency-Key", "adjust-route-1")
	managerRequest = managerRequest.WithContext(context.WithValue(managerRequest.Context(), identityKey{}, identity{UserID: 2, TenantID: 9, Role: RoleMerchantManager}))
	managerResponse := httptest.NewRecorder()
	router.ServeHTTP(managerResponse, managerRequest)
	if managerResponse.Code != http.StatusForbidden {
		t.Fatalf("manager adjustment status=%d body=%s", managerResponse.Code, managerResponse.Body.String())
	}
}

func TestOwnerBalanceAdjustmentRequiresIdempotencyKey(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.memberRoutes(router)
	request := httptest.NewRequest(http.MethodPost, "/customers/3/balance-adjustments", bytes.NewBufferString(`{"bucket":"PRINCIPAL","direction":"CREDIT","amount_cents":100,"remark":"test"}`))
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 1, TenantID: 9, Role: RoleMerchantOwner}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "IDEMPOTENCY_KEY_REQUIRED") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
