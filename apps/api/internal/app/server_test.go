package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
