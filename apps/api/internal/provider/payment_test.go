package provider

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestMockPaymentLifecycle(t *testing.T) {
	t.Parallel()
	provider := NewMockPayment()
	created, err := provider.Create(context.Background(), CreatePaymentRequest{MerchantNo: "M001", OrderNo: "TB001", Amount: 1200})
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

func TestMockPaymentRejectsCumulativeRefundOverflow(t *testing.T) {
	t.Parallel()
	mock := NewMockPayment()
	request := CreatePaymentRequest{MerchantNo: "M-LIMIT", OrderNo: "TB-LIMIT", Amount: 100}
	created, err := mock.Create(context.Background(), request)
	if err != nil || !mock.Confirm(created.ProviderOrderNo) {
		t.Fatalf("prepare payment: result=%+v err=%v", created, err)
	}
	if _, err = mock.Refund(context.Background(), RefundRequest{MerchantNo: request.MerchantNo, ProviderOrderNo: created.ProviderOrderNo, RefundNo: "RF-1", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = mock.Refund(context.Background(), RefundRequest{MerchantNo: request.MerchantNo, ProviderOrderNo: created.ProviderOrderNo, RefundNo: "RF-MAX", Amount: math.MaxInt64}); err == nil {
		t.Fatal("refund larger than the remaining paid amount must be rejected")
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

func TestMockPaymentClosedAttemptCanBeReplacedByFreshAttempt(t *testing.T) {
	t.Parallel()
	mock := NewMockPayment()
	first, err := mock.Create(context.Background(), CreatePaymentRequest{MerchantNo: "M-RETRY", OrderNo: "PY-ATTEMPT-1", Amount: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err = mock.Close(context.Background(), first.ProviderOrderNo); err != nil {
		t.Fatal(err)
	}
	second, err := mock.Create(context.Background(), CreatePaymentRequest{MerchantNo: "M-RETRY", OrderNo: "PY-ATTEMPT-2", Amount: 100})
	if err != nil {
		t.Fatal(err)
	}
	if first.ProviderOrderNo == second.ProviderOrderNo {
		t.Fatalf("fresh attempts must use distinct provider keys: %s", first.ProviderOrderNo)
	}
	if !mock.Confirm(second.ProviderOrderNo) {
		t.Fatal("the fresh attempt should remain payable")
	}
	if mock.Confirm(first.ProviderOrderNo) {
		t.Fatal("the closed prior attempt must remain closed")
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

func TestWeChatPayPartnerStaysDisabledUntilAPIv3TransportIsImplemented(t *testing.T) {
	t.Parallel()
	adapter := WeChatPayPartner{Config: WeChatPayPartnerConfig{ServiceProviderMchID: "1900000001"}}
	if adapter.Name() != "wechat_partner" {
		t.Fatalf("unexpected provider name %q", adapter.Name())
	}
	if _, err := adapter.Create(context.Background(), CreatePaymentRequest{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}
