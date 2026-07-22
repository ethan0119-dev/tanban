package app

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/cache"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func TestMerchantOwnerWithMultipleTenantsMustSelectWorkspace(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash, _ := bcrypt.GenerateFromPassword([]byte("owner-password"), bcrypt.MinCost)
	mock.ExpectQuery("SELECT id,username,display_name,password_hash,status").
		WithArgs("manong").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "password_hash", "status", "platform_role"}).AddRow(2, "manong", "码农咖啡店主", string(hash), "ACTIVE", ""))
	mock.ExpectQuery("SELECT m.id,m.tenant_id,t.name,s.id,s.name,s.logo_url,m.role").
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"membership_id", "tenant_id", "tenant_name", "store_id", "store_name", "store_logo_url", "role"}).
			AddRow(11, 1, "码农咖啡鼓楼店", 1, "码农咖啡鼓楼店", "", RoleMerchantOwner).
			AddRow(12, 3, "码农咖啡大悦城店", 3, "码农咖啡大悦城店", "", RoleMerchantOwner).
			AddRow(13, 4, "码农咖啡", 4, "码农咖啡", "", RoleMerchantOwner))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	server := &Server{DB: db, Config: config.Config{JWTSecret: "12345678901234567890123456789012", JWTTTL: time.Hour}, Logger: slog.Default(), Cache: cache.NewMemory()}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"manong","password":"owner-password","portal":"merchant"}`))
	response := httptest.NewRecorder()
	server.login(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			SelectionRequired bool                `json:"selection_required"`
			SelectionToken    string              `json:"selection_token"`
			Workspaces        []merchantWorkspace `json:"workspaces"`
		} `json:"data"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Data.SelectionRequired || payload.Data.SelectionToken == "" || len(payload.Data.Workspaces) != 3 {
		t.Fatalf("unexpected selection response: %s", response.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantStaffWithSingleTenantEntersDirectly(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash, _ := bcrypt.GenerateFromPassword([]byte("staff-password"), bcrypt.MinCost)
	mock.ExpectQuery("SELECT id,username,display_name,password_hash,status").
		WithArgs("barista").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "password_hash", "status", "platform_role"}).AddRow(20, "barista", "咖啡师", string(hash), "ACTIVE", ""))
	mock.ExpectQuery("SELECT m.id,m.tenant_id,t.name,s.id,s.name,s.logo_url,m.role").
		WithArgs(int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{"membership_id", "tenant_id", "tenant_name", "store_id", "store_name", "store_logo_url", "role"}).
			AddRow(21, 3, "码农咖啡大悦城店", 3, "码农咖啡大悦城店", "", RoleMerchantStaff))
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnResult(sqlmock.NewResult(1, 1))

	server := &Server{DB: db, Config: config.Config{JWTSecret: "12345678901234567890123456789012", JWTTTL: time.Hour}, Logger: slog.Default(), Cache: cache.NewMemory()}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"barista","password":"staff-password","portal":"merchant"}`))
	response := httptest.NewRecorder()
	server.login(response, request)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"access_token"`)) || bytes.Contains(response.Body.Bytes(), []byte(`"selection_required":true`)) {
		t.Fatalf("unexpected direct login response: status=%d body=%s", response.Code, response.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
