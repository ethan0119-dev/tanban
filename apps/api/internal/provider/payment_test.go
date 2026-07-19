package provider

import (
	"context"
	"testing"
)

func TestMockPaymentLifecycle(t *testing.T) {
	t.Parallel()
	provider := NewMockPayment()
	created, err := provider.Create(context.Background(), CreatePaymentRequest{OrderNo: "TB001", Amount: 1200})
	if err != nil || created.Status != PaymentPending {
		t.Fatalf("create: result=%+v err=%v", created, err)
	}
	if !provider.Confirm(created.ProviderOrderNo) {
		t.Fatal("confirm should accept pending payment")
	}
	queried, err := provider.Query(context.Background(), created.ProviderOrderNo)
	if err != nil || queried.Status != PaymentSuccess || queried.PaidAt == nil {
		t.Fatalf("query: result=%+v err=%v", queried, err)
	}
	refunded, err := provider.Refund(context.Background(), RefundRequest{MerchantNo: "M001", ProviderOrderNo: created.ProviderOrderNo, RefundNo: "RF001", Amount: 300})
	if err != nil || refunded.Status != PaymentSuccess {
		t.Fatalf("refund: result=%+v err=%v", refunded, err)
	}
	queriedRefund, err := provider.QueryRefund(context.Background(), "RF001")
	if err != nil || queriedRefund.Status != PaymentSuccess || queriedRefund.ProviderRefundNo == "" || queriedRefund.RefundNo != "RF001" || queriedRefund.ProviderOrderNo != created.ProviderOrderNo || queriedRefund.MerchantNo != "M001" || queriedRefund.Amount != 300 {
		t.Fatalf("query refund: result=%+v err=%v", queriedRefund, err)
	}
}

func TestMockPaymentCannotConfirmAfterClose(t *testing.T) {
	t.Parallel()
	mock := NewMockPayment()
	created, err := mock.Create(context.Background(), CreatePaymentRequest{OrderNo: "TB-CLOSE", Amount: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err = mock.Close(context.Background(), created.ProviderOrderNo); err != nil {
		t.Fatal(err)
	}
	if mock.Confirm(created.ProviderOrderNo) {
		t.Fatal("closed payment must not be confirmed")
	}
}

func TestMockPaymentCreateAndRefundAreProviderIdempotent(t *testing.T) {
	t.Parallel()
	mock := NewMockPayment()
	request := CreatePaymentRequest{MerchantNo: "M001", OrderNo: "TB-IDEMPOTENT", Amount: 880}
	created, err := mock.Create(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !mock.Confirm(created.ProviderOrderNo) {
		t.Fatal("confirm should succeed")
	}
	repeated, err := mock.Create(context.Background(), request)
	if err != nil || repeated.ProviderOrderNo != created.ProviderOrderNo || repeated.Status != PaymentSuccess {
		t.Fatalf("repeated create must preserve the provider result: %+v err=%v", repeated, err)
	}
	queried, err := mock.Query(context.Background(), created.ProviderOrderNo)
	if err != nil || queried.OrderNo != request.OrderNo || queried.MerchantNo != request.MerchantNo || queried.Amount != request.Amount {
		t.Fatalf("query must return identity fields: %+v err=%v", queried, err)
	}
	refundRequest := RefundRequest{MerchantNo: request.MerchantNo, ProviderOrderNo: created.ProviderOrderNo, RefundNo: "RF-IDEMPOTENT", Amount: 120}
	first, err := mock.Refund(context.Background(), refundRequest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := mock.Refund(context.Background(), refundRequest)
	if err != nil || second != first {
		t.Fatalf("repeated refund must preserve the provider result: first=%+v second=%+v err=%v", first, second, err)
	}
}
