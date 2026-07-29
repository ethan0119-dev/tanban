package app

import (
	"net/http/httptest"
	"testing"
)

func TestCashierActiveOrderQueryAliases(t *testing.T) {
	t.Parallel()
	for _, rawURL := range []string{
		"/api/v1/merchant/orders?cashier_active=1",
		"/api/v1/merchant/orders?cashier_active=true",
		"/api/v1/merchant/orders?cashierActive=true",
	} {
		request := httptest.NewRequest("GET", rawURL, nil)
		if !cashierActiveOrdersRequested(request) {
			t.Fatalf("cashier active alias was not recognized for %s", rawURL)
		}
	}
	if cashierActiveOrdersRequested(httptest.NewRequest("GET", "/api/v1/merchant/orders?cashier_active=false", nil)) {
		t.Fatal("cashier active filter accepted false")
	}
}
