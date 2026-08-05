package provider

import "testing"

func TestPaymentRouterResolvesRegisteredProviders(t *testing.T) {
	mock := NewMockPayment()
	router := NewPaymentRouter(mock, TianQue{})

	resolved, err := router.Resolve(" MOCK ")
	if err != nil {
		t.Fatalf("resolve mock: %v", err)
	}
	if resolved != mock {
		t.Fatal("router did not return the registered mock instance")
	}
	if _, err = router.Resolve("wechat_partner"); err == nil {
		t.Fatal("expected an unregistered provider to fail")
	}
	names := router.Names()
	if len(names) != 2 || names[0] != "mock" || names[1] != "tianque" {
		t.Fatalf("unexpected provider names: %#v", names)
	}
}
