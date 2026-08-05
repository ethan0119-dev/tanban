package app

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
)

func TestPublicMiniAppBootstrapReturnsConfiguredDefaultStore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := &Server{DB: db}
	mock.ExpectQuery("SELECT s.code").
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("manong-coffee-gulou"))

	request := httptest.NewRequest("GET", "/api/v1/public/miniapp/bootstrap?channelKey=tanban-public", nil)
	recorder := httptest.NewRecorder()
	server.publicMiniAppBootstrap(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			StoreCode  string `json:"storeCode"`
			ChannelKey string `json:"channelKey"`
		} `json:"data"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.StoreCode != "manong-coffee-gulou" || response.Data.ChannelKey != publicMiniAppChannelKey {
		t.Fatalf("unexpected bootstrap response: %#v", response.Data)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMiniAppSecretEncryptionRoundTrip(t *testing.T) {
	t.Parallel()
	server := &Server{Config: config.Config{JWTSecret: "12345678901234567890123456789012"}}
	ciphertext, err := server.encryptMiniAppSecret("dedicated-secret")
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == "" || ciphertext == "dedicated-secret" {
		t.Fatal("miniapp secret was not encrypted")
	}
	plain, err := server.decryptMiniAppSecret(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "dedicated-secret" {
		t.Fatalf("decrypted secret=%q", plain)
	}
}

func TestResolveDedicatedMiniAppCredentialsIsTenantScoped(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := New(db, config.Config{
		JWTSecret:     "12345678901234567890123456789012",
		WeChatMiniApp: config.WeChatMiniApp{APIBaseURL: "https://api.weixin.qq.com"},
	}, slog.Default())
	secret, err := server.encryptMiniAppSecret("dedicated-secret")
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT tenant_id,COALESCE\\(dedicated_channel_key").
		WithArgs("wx-channel-tenant-9").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "channel_key", "appid", "secret"}).
			AddRow(9, "wx-channel-tenant-9", "wx1234567890abcdef", secret))
	request := httptest.NewRequest("POST", "/public/customer/session", nil)
	credentials, err := server.resolveMiniAppCredentials(request, storeDTO{TenantID: 9}, "wx-channel-tenant-9")
	if err != nil {
		t.Fatal(err)
	}
	if credentials.TenantID != 9 || credentials.Mode != "DEDICATED" || credentials.AppID != "wx1234567890abcdef" || credentials.AppSecret != "dedicated-secret" {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveDedicatedMiniAppRejectsAnotherTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := &Server{DB: db, Config: config.Config{JWTSecret: "12345678901234567890123456789012"}}
	mock.ExpectQuery("SELECT tenant_id,COALESCE\\(dedicated_channel_key").
		WithArgs("wx-channel-tenant-9").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "channel_key", "appid", "secret"}).
			AddRow(9, "wx-channel-tenant-9", "wx1234567890abcdef", "not-needed"))
	request := httptest.NewRequest("POST", "/public/customer/session", nil)
	if _, err = server.resolveMiniAppCredentials(request, storeDTO{TenantID: 10}, "wx-channel-tenant-9"); err == nil {
		t.Fatal("dedicated channel must not access another tenant")
	}
}

func TestValidMiniAppID(t *testing.T) {
	t.Parallel()
	if !validMiniAppID("wx1234567890abcdef") {
		t.Fatal("valid miniapp AppID was rejected")
	}
	for _, value := range []string{"", "wx-short", "zz1234567890abcdef", "wx1234567890abcde!"} {
		if validMiniAppID(value) {
			t.Fatalf("invalid AppID %q was accepted", value)
		}
	}
}
