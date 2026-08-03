package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
)

func TestRequireCashierEnabledRejectsDisabledTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT cashier_enabled FROM tenants").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"cashier_enabled"}).AddRow(false))

	server := New(db, config.Config{}, slog.Default())
	called := false
	handler := server.requireCashierEnabled(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	request := httptest.NewRequest(http.MethodPost, "/cashier/session", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{TenantID: 7, Role: RoleMerchantOwner}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if called || response.Code != http.StatusForbidden || !bytes.Contains(response.Body.Bytes(), []byte("CASHIER_NOT_ENABLED")) {
		t.Fatalf("status=%d called=%v body=%s", response.Code, called, response.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireCashierEnabledAllowsEnabledTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT cashier_enabled FROM tenants").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"cashier_enabled"}).AddRow(true))

	server := New(db, config.Config{}, slog.Default())
	handler := server.requireCashierEnabled(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodPost, "/cashier/session", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{TenantID: 7, Role: RoleMerchantOwner}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCashierDisabledPathAllowed(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/me",
		"/api/v1/auth/workspaces",
		"/api/v1/merchant/cashier/context",
		"/api/v1/merchant/pickup-display",
	} {
		if !cashierDisabledPathAllowed(path) {
			t.Fatalf("disabled cashier should retain preview-safe path %q", path)
		}
	}
	for _, path := range []string{
		"/api/v1/merchant/cashier/session",
		"/api/v1/merchant/orders",
		"/api/v1/merchant/orders/42/cashier-settle",
	} {
		if cashierDisabledPathAllowed(path) {
			t.Fatalf("disabled cashier unexpectedly retained operational path %q", path)
		}
	}
}
