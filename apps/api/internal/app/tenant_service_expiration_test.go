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
	"github.com/go-chi/chi/v5"
)

func TestExpiredMerchantServiceIsPaused(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	router := chi.NewRouter()
	server.merchantRoutes(router)
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{
		UserID: 21, TenantID: 7, Role: RoleMerchantOwner, ServiceExpiresAt: "2026-07-22", ServiceExpired: true,
	}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusPaymentRequired || !bytes.Contains(response.Body.Bytes(), []byte("MERCHANT_SERVICE_EXPIRED")) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTenantServiceExpirationRejectsInvalidDate(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{}, slog.Default())
	router := chi.NewRouter()
	router.Put("/tenants/{tenantID}/service-expiration", server.updateTenantServiceExpiration)
	request := httptest.NewRequest(http.MethodPut, "/tenants/7/service-expiration", bytes.NewBufferString(`{"expires_at":"2026-02-30"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
