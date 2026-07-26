package app

import "testing"

func TestCashierTokenPathAllowed(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"/api/v1/auth/me",
		"/api/v1/auth/workspaces",
		"/api/v1/merchant/dashboard",
		"/api/v1/merchant/table-board",
		"/api/v1/merchant/pickup-display",
		"/api/v1/merchant/cashier/session",
		"/api/v1/merchant/cashier/context",
		"/api/v1/merchant/cashier/handover",
		"/api/v1/merchant/orders",
		"/api/v1/merchant/orders/42",
		"/api/v1/merchant/orders/42/cashier-settle",
		"/api/v1/merchant/print-jobs/99/retry",
	} {
		if !cashierTokenPathAllowed(path) {
			t.Fatalf("cashier path %q was rejected", path)
		}
	}
	for _, path := range []string{
		"/api/v1/platform/tenants",
		"/api/v1/merchant/products",
		"/api/v1/merchant/settings",
		"/api/v1/merchant/staff",
	} {
		if cashierTokenPathAllowed(path) {
			t.Fatalf("administration path %q was allowed", path)
		}
	}
}

func TestValidWechatPaymentCode(t *testing.T) {
	t.Parallel()
	for _, code := range []string{
		"101234567890123456",
		"121234567890123456",
		"151234567890123456",
	} {
		if !validWechatPaymentCode(code) {
			t.Fatalf("valid WeChat payment code %q was rejected", code)
		}
	}
	for _, code := range []string{
		"", "091234567890123456", "161234567890123456",
		"10123456789012345", "1012345678901234567", "10123456789012345x",
	} {
		if validWechatPaymentCode(code) {
			t.Fatalf("invalid WeChat payment code %q was accepted", code)
		}
	}
}

func TestValidateWechatCodePayableOrder(t *testing.T) {
	t.Parallel()
	ready := wechatCodePayableOrder{
		Status: "READY", PaymentStatus: "UNPAID", SettlementMode: "PAY_AFTER",
		TenantPaymentProvider: "wechat_partner", MerchantNo: "1900000109",
		OnboardingStatus: "ACTIVE", ProductAuthorizationStatus: "AUTHORIZED",
	}
	if code, message := validateWechatCodePayableOrder(ready, "wechat_partner"); code != "" {
		t.Fatalf("ready order rejected: %s %s", code, message)
	}
	paid := ready
	paid.PaymentStatus = "PAID"
	if code, _ := validateWechatCodePayableOrder(paid, "wechat_partner"); code != "ORDER_ALREADY_PAID" {
		t.Fatalf("paid order returned code %q", code)
	}
	notAuthorized := ready
	notAuthorized.ProductAuthorizationStatus = "NOT_AUTHORIZED"
	if code, _ := validateWechatCodePayableOrder(notAuthorized, "wechat_partner"); code != "WECHAT_PAY_MERCHANT_NOT_READY" {
		t.Fatalf("unauthorized merchant returned code %q", code)
	}
}
