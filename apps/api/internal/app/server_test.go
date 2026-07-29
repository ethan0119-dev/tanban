package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
)

func TestHealthEnvelope(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{JWTSecret: "12345678901234567890123456789012"}, slog.Default())
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() == "" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTrustedProxyRealIPOnlyAcceptsLocalNginx(t *testing.T) {
	t.Parallel()
	server := &Server{}
	handler := server.trustedProxyRealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.RemoteAddr))
	}))

	direct := httptest.NewRequest(http.MethodGet, "/", nil)
	direct.RemoteAddr = "198.51.100.7:45678"
	direct.Header.Set("True-Client-IP", "203.0.113.99")
	direct.Header.Set("X-Real-IP", "203.0.113.98")
	direct.Header.Set("X-Forwarded-For", "203.0.113.97")
	directResponse := httptest.NewRecorder()
	handler.ServeHTTP(directResponse, direct)
	if got := directResponse.Body.String(); got != "198.51.100.7:45678" {
		t.Fatalf("direct client spoofed forwarding headers: %q", got)
	}

	proxied := httptest.NewRequest(http.MethodGet, "/", nil)
	proxied.RemoteAddr = "127.0.0.1:45678"
	proxied.Header.Set("True-Client-IP", "203.0.113.99")
	proxied.Header.Set("X-Real-IP", "198.51.100.8")
	proxied.Header.Set("X-Forwarded-For", "203.0.113.97")
	proxiedResponse := httptest.NewRecorder()
	handler.ServeHTTP(proxiedResponse, proxied)
	if got := strings.TrimSpace(proxiedResponse.Body.String()); got != "198.51.100.8" {
		t.Fatalf("local proxy client address was not accepted: %q", got)
	}
}

func TestReserveStockIsAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := `UPDATE inventory SET stock=stock-\? WHERE sku_id=\? AND tenant_id=\? AND stock>=\?`
	mock.ExpectExec(query).WithArgs(2, int64(10), int64(3), 2).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := reserveStock(context.Background(), db, 3, 10, 2); err != nil {
		t.Fatalf("reserve stock: %v", err)
	}
	mock.ExpectExec(query).WithArgs(2, int64(10), int64(3), 2).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := reserveStock(context.Background(), db, 3, 10, 2); !errors.Is(err, errInsufficientStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInventoryEditRejectsStaleExpectedStock(t *testing.T) {
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
	mock.ExpectQuery(`SELECT stock FROM inventory`).WithArgs(int64(10), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"stock"}).AddRow(9))
	mock.ExpectRollback()
	if err = updateInventoryOptimistic(context.Background(), tx, 3, 10, 10, 10, true, false, 0); !errors.Is(err, errStockConflict) {
		t.Fatalf("expected stock conflict, got %v", err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
