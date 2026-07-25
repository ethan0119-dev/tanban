package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

func TestExchangeWechatLoginCode(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/jscode2session" ||
			r.URL.Query().Get("appid") != "mini-app" ||
			r.URL.Query().Get("secret") != "mini-secret" ||
			r.URL.Query().Get("js_code") != "temporary-code" ||
			r.URL.Query().Get("grant_type") != "authorization_code" {
			t.Fatalf("unexpected code2session request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openid":"wx-open-id","session_key":"secret-session"}`))
	}))
	defer upstream.Close()

	server := &Server{
		Config: config.Config{WeChatMiniApp: config.WeChatMiniApp{
			APIBaseURL: upstream.URL, AppID: "mini-app", AppSecret: "mini-secret",
		}},
		HTTPClient: upstream.Client(),
	}
	result, err := server.exchangeWechatLoginCode(context.Background(), miniAppCredentials{
		AppID: "mini-app", AppSecret: "mini-secret",
	}, "temporary-code")
	if err != nil {
		t.Fatal(err)
	}
	if result.OpenID != "wx-open-id" || result.SessionKey != "secret-session" {
		t.Fatalf("unexpected code2session result: %#v", result)
	}
}

func TestOptionalPublicCustomerSessionRequiresCustomerToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const signingKey = "12345678901234567890123456789012"
	server := &Server{DB: db, Config: config.Config{JWTSecret: signingKey}}
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		TenantID:  9,
		Role:      "CUSTOMER",
		TokenKind: "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "21", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)), Issuer: "tanban-api",
		},
	}).SignedString([]byte(signingKey))
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT COALESCE\\(source_store_id,0\\),COALESCE\\(wechat_openid,''\\)").
		WithArgs(int64(9), int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{"source_store_id", "wechat_openid"}).AddRow(7, "wx-open-id"))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	session, ok := server.optionalPublicCustomerSession(request.Context(), request, 9)
	if !ok || session.CustomerID != 21 || session.StoreID != 7 || session.OpenID != "wx-open-id" {
		t.Fatalf("unexpected customer session: ok=%v session=%#v", ok, session)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
