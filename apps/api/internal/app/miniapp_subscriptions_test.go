package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/cache"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
)

func TestWechatSubscriptionTemplateFormatting(t *testing.T) {
	if got := wechatTemplateAmount(12345); got != "123.45元" {
		t.Fatalf("unexpected amount: %q", got)
	}
	if got := wechatTemplateAmount(-50); got != "0.50元" {
		t.Fatalf("unexpected negative amount normalization: %q", got)
	}
	if got := wechatTemplateText("  餐品已制作完成\n请及时取餐  ", 8); got != "餐品已制作完成" {
		t.Fatalf("unexpected template text: %q", got)
	}
	if miniAppNotificationPermanentError(wechatSubscribeSendError{Code: 50002, Msg: "busy"}) {
		t.Fatal("transient provider errors must remain retryable")
	}
	if !miniAppNotificationPermanentError(wechatSubscribeSendError{Code: 47003, Msg: "invalid data"}) {
		t.Fatal("invalid template payload must be permanent")
	}
}

func TestSendMiniAppSubscriptionMessageUsesStableTokenAndExactPayload(t *testing.T) {
	var tokenCalls, sendCalls int
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			tokenCalls++
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"appid":"wx087d633542ae8d0b"`) {
				t.Fatalf("stable token request missing appid: %s", body)
			}
			_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":7200}`))
		case "/cgi-bin/message/subscribe/send":
			sendCalls++
			if r.URL.Query().Get("access_token") != "token-1" {
				t.Fatalf("unexpected token: %q", r.URL.Query().Get("access_token"))
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["template_id"] != "pickup-template" || body["page"] != "pages/order-detail/index?orderNo=TB1" {
				t.Fatalf("unexpected send payload: %#v", body)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	server := &Server{
		Config: config.Config{WeChatMiniApp: config.WeChatMiniApp{APIBaseURL: provider.URL}},
		Cache:  cache.NewMemory(), HTTPClient: provider.Client(),
	}
	item := miniAppNotificationOutboxItem{
		AppID: "wx087d633542ae8d0b", OpenID: "openid", TemplateID: "pickup-template",
		PagePath:    "pages/order-detail/index?orderNo=TB1",
		PayloadJSON: `{"phrase19":{"value":"请取餐"}}`,
	}
	credentials := miniAppCredentials{AppID: "wx087d633542ae8d0b", AppSecret: "secret"}
	if _, err := server.sendMiniAppSubscriptionMessage(context.Background(), credentials, item); err != nil {
		t.Fatal(err)
	}
	if _, err := server.sendMiniAppSubscriptionMessage(context.Background(), credentials, item); err != nil {
		t.Fatal(err)
	}
	if tokenCalls != 1 || sendCalls != 2 {
		t.Fatalf("expected cached token and two sends, token=%d send=%d", tokenCalls, sendCalls)
	}
}

func TestMiniAppNotificationBackoffIsBounded(t *testing.T) {
	if got := miniAppNotificationBackoff(1); got != 2*time.Second {
		t.Fatalf("unexpected first backoff: %v", got)
	}
	if got := miniAppNotificationBackoff(50); got != 256*time.Second {
		t.Fatalf("unexpected capped backoff: %v", got)
	}
}
