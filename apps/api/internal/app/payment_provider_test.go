package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

func TestDescribePaymentProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		provider, displayName, checkoutMode string
		implemented                         bool
	}{
		{"mock", "模拟支付（开发环境）", "MOCK", true},
		{"tianque", "会生活 · 随行付", "HALF_SCREEN_CASHIER", false},
		{"wechat_partner", "微信支付（普通服务商）", "WECHAT_MINI_PROGRAM", true},
	}
	for _, test := range tests {
		got := describePaymentProvider(test.provider)
		if got.DisplayName != test.displayName || got.CheckoutMode != test.checkoutMode || got.AdapterImplemented != test.implemented {
			t.Fatalf("describePaymentProvider(%q)=%+v", test.provider, got)
		}
	}
}

func TestPaymentClientActionUsesCapabilityInsteadOfProviderName(t *testing.T) {
	t.Parallel()
	params := map[string]string{
		"timeStamp": "1710000000", "nonceStr": "nonce", "package": "prepay_id=123", "signType": "RSA", "paySign": "signature",
	}
	if got := paymentClientAction("wechat_partner", params); got != "WECHAT_REQUEST_PAYMENT" {
		t.Fatalf("wechat action=%q", got)
	}
	if got := paymentClientAction("lichu", params); got != "WECHAT_REQUEST_PAYMENT" {
		t.Fatalf("lichu action=%q", got)
	}
	if got := paymentClientAction("mock", nil); got != "MOCK_CONFIRM" {
		t.Fatalf("mock action=%q", got)
	}
	if got := paymentClientAction("lichu", map[string]string{"timeStamp": "1710000000"}); got != "NONE" {
		t.Fatalf("incomplete action=%q", got)
	}
}

func TestNormalizePaymentWorkflowStatusesAllowsNonWechatConfiguration(t *testing.T) {
	t.Parallel()
	settings := tenantPaymentSettings{Provider: "mock"}
	normalizePaymentWorkflowStatuses(&settings)
	if settings.OnboardingStatus != "NOT_APPLIED" || settings.ProductAuthorizationStatus != "NOT_AUTHORIZED" {
		t.Fatalf("unexpected defaults: %+v", settings)
	}
}

func TestWeChatPayCallbacksFailClosedWithoutValidSignature(t *testing.T) {
	t.Parallel()
	wechatPay := &provider.WeChatPayPartner{}
	server := &Server{Payment: wechatPay, WeChatPay: wechatPay}
	tests := []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"/api/v1/payments/wechat-partner/callback", server.wechatPayCallback},
		{"/api/v1/payments/wechat-partner/refund-callback", server.wechatPayRefundCallback},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		test.handler(response, httptest.NewRequest(http.MethodPost, test.path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s returned %d, want %d", test.path, response.Code, http.StatusUnauthorized)
		}
	}
}
