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
