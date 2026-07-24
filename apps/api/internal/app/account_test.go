package app

import (
	"bytes"
	"context"
	"database/sql/driver"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type bcryptHashOf string

func (password bcryptHashOf) Match(value driver.Value) bool {
	hash, ok := value.(string)
	return ok && bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func TestCreateTenantProvisionsFirstStoreAndOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO tenants").
		WithArgs("WDP001", "王大鹏", "王大鹏", "13800138000", "ACTIVE", "mock", "", "").
		WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO stores").
		WithArgs(int64(31), "wangdapeng", "王大鹏主门店").
		WillReturnResult(sqlmock.NewResult(41, 1))
	mock.ExpectExec("INSERT IGNORE INTO store_business_periods").
		WithArgs(int64(31), int64(41), 0, 0).
		WillReturnResult(sqlmock.NewResult(0, 7))
	mock.ExpectExec("INSERT INTO accounts").
		WithArgs("13800138000", bcryptHashOf("Safe-pass-2026!"), "王大鹏").
		WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectExec("INSERT INTO tenant_memberships").
		WithArgs(int64(31), int64(51), RoleMerchantOwner).
		WillReturnResult(sqlmock.NewResult(61, 1))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT t.id,t.code,t.name").
		WithArgs(RoleMerchantOwner, RoleMerchantOwner, RoleMerchantOwner, RoleMerchantOwner, int64(31)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "code", "name", "contact_name", "contact_phone", "status", "payment_provider", "payment_merchant_no", "payment_sub_appid",
			"business_license_url", "food_business_license_url", "store_id", "store_code", "store_name", "order_count", "owner_username", "owner_display_name", "owner_status", "has_owner", "created_at",
		}).AddRow(31, "WDP001", "王大鹏", "王大鹏", "13800138000", "ACTIVE", "mock", "", "", "", "", 41, "wangdapeng", "王大鹏主门店", 0, "13800138000", "王大鹏", "ACTIVE", true, "2026-07-20T10:00:00Z"))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	request := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(`{
		"code":"WDP001","name":"王大鹏","contact_name":"王大鹏","contact_phone":"13800138000","status":"ACTIVE","payment_provider":"mock",
		"owner_username":"13800138000","owner_password":"Safe-pass-2026!","owner_display_name":"王大鹏","initial_store_code":"wangdapeng","initial_store_name":"王大鹏主门店"
	}`))
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 1, Role: RolePlatformAdmin}))
	response := httptest.NewRecorder()
	server.createTenant(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "Safe-pass-2026!") {
		t.Fatalf("API must not echo the plaintext password: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateFirstOwnerForExistingTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM tenants").
		WithArgs(int64(31)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM tenant_memberships").
		WithArgs(int64(31), RoleMerchantOwner).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("INSERT INTO accounts").
		WithArgs("wangdapeng", bcryptHashOf("Owner-pass-2026!"), "王大鹏").
		WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectExec("INSERT INTO tenant_memberships").
		WithArgs(int64(31), int64(51), RoleMerchantOwner).
		WillReturnResult(sqlmock.NewResult(61, 1))
	mock.ExpectCommit()
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	router.Post("/tenants/{tenantID}/owner", server.createTenantOwner)
	request := httptest.NewRequest(http.MethodPost, "/tenants/31/owner", bytes.NewBufferString(`{"username":"wangdapeng","password":"Owner-pass-2026!","display_name":"王大鹏"}`))
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 1, Role: RolePlatformAdmin}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "Owner-pass-2026!") {
		t.Fatalf("API must not echo the plaintext password: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantCanChangeOwnPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	currentHash, err := bcrypt.GenerateFromPassword([]byte("current-pass"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a.password_hash FROM accounts a").
		WithArgs(int64(7), int64(21)).WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(string(currentHash)))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET password_hash=?,updated_at=NOW(3) WHERE id=? AND status='ACTIVE' AND deleted_at IS NULL")).
		WithArgs(sqlmock.AnyArg(), int64(21)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs(tenant_id,actor_user_id,action,resource_type,resource_id,request_id,ip,details_text) VALUES(?,?,?,?,?,?,?,?)")).
		WithArgs(int64(7), int64(21), "account.password.change", "user", "21", "", sqlmock.AnyArg(), "null").
		WillReturnResult(sqlmock.NewResult(1, 1))

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.merchantRoutes(router)
	request := httptest.NewRequest(http.MethodPut, "/account/password", bytes.NewBufferString(`{"current_password":"current-pass","new_password":"next-pass-2026"}`))
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 21, TenantID: 7, Role: RoleMerchantOwner}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantPasswordChangeRejectsWrongCurrentPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	currentHash, _ := bcrypt.GenerateFromPassword([]byte("actual-pass"), bcrypt.MinCost)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a.password_hash FROM accounts a").WithArgs(int64(7), int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(string(currentHash)))
	mock.ExpectRollback()

	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.merchantRoutes(router)
	request := httptest.NewRequest(http.MethodPut, "/account/password", bytes.NewBufferString(`{"current_password":"wrong-pass","new_password":"next-pass-2026"}`))
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 21, TenantID: 7, Role: RoleMerchantOwner}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantPasswordChangeRejectsPasswordOverBcryptLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	router := chi.NewRouter()
	server.merchantRoutes(router)
	tooLong := strings.Repeat("密", 25) // 75 UTF-8 bytes; bcrypt accepts at most 72.
	request := httptest.NewRequest(http.MethodPut, "/account/password", bytes.NewBufferString(`{"current_password":"current-pass","new_password":"`+tooLong+`"}`))
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{UserID: 21, TenantID: 7, Role: RoleMerchantOwner}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
