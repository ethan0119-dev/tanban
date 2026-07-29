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
		{"wechat_partner", "微信支付（普通服务商）", "WECHAT_MINI_PROGRAM", false},
	}
	for _, test := range tests {
		got := describePaymentProvider(test.provider)
		if got.DisplayName != test.displayName || got.CheckoutMode != test.checkoutMode || got.AdapterImplemented != test.implemented {
			t.Fatalf("describePaymentProvider(%q)=%+v", test.provider, got)
		}
	}
}

func TestWeChatPayCallbacksFailClosedUntilVerificationExists(t *testing.T) {
	t.Parallel()
	server := &Server{Payment: provider.WeChatPayPartner{}}
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
		if response.Code != http.StatusNotImplemented {
			t.Fatalf("%s returned %d, want %d", test.path, response.Code, http.StatusNotImplemented)
		}
	}
}
