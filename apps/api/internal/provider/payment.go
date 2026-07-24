package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrNotConfigured = errors.New("provider not configured")

type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "PENDING"
	PaymentSuccess  PaymentStatus = "SUCCESS"
	PaymentFailed   PaymentStatus = "FAILED"
	PaymentClosed   PaymentStatus = "CLOSED"
	PaymentRefunded PaymentStatus = "REFUNDED"
)

type CreatePaymentRequest struct {
	MerchantNo string
	OrderNo    string
	Amount     int64
	OpenID     string
	SubAppID   string
	NotifyURL  string
}

type CreatePaymentResult struct {
	ProviderOrderNo string            `json:"provider_order_no"`
	Status          PaymentStatus     `json:"status"`
	PayParams       map[string]string `json:"pay_params"`
}

type QueryPaymentResult struct {
	ProviderOrderNo string        `json:"provider_order_no"`
	Status          PaymentStatus `json:"status"`
	MerchantNo      string        `json:"merchant_no"`
	OrderNo         string        `json:"order_no"`
	Amount          int64         `json:"amount"`
	PaidAt          *time.Time    `json:"paid_at,omitempty"`
}

type RefundRequest struct {
	MerchantNo      string
	ProviderOrderNo string
	RefundNo        string
	Amount          int64
}

type RefundResult struct {
	ProviderRefundNo string        `json:"provider_refund_no"`
	Status           PaymentStatus `json:"status"`
}

type QueryRefundResult struct {
	RefundNo         string        `json:"refund_no"`
	ProviderRefundNo string        `json:"provider_refund_no"`
	ProviderOrderNo  string        `json:"provider_order_no"`
	MerchantNo       string        `json:"merchant_no"`
	Amount           int64         `json:"amount"`
	Status           PaymentStatus `json:"status"`
}

type PaymentProvider interface {
	Name() string
	Create(context.Context, CreatePaymentRequest) (CreatePaymentResult, error)
	Query(context.Context, string) (QueryPaymentResult, error)
	Close(context.Context, string) error
	Refund(context.Context, RefundRequest) (RefundResult, error)
	QueryRefund(context.Context, string) (QueryRefundResult, error)
}

type MockPayment struct {
	mu             sync.RWMutex
	statuses       map[string]PaymentStatus
	requests       map[string]CreatePaymentRequest
	refundStatuses map[string]QueryRefundResult
}

func NewMockPayment() *MockPayment {
	return &MockPayment{
		statuses:       make(map[string]PaymentStatus),
		requests:       make(map[string]CreatePaymentRequest),
		refundStatuses: make(map[string]QueryRefundResult),
	}
}
func (m *MockPayment) Name() string { return "mock" }

func (m *MockPayment) Create(_ context.Context, req CreatePaymentRequest) (CreatePaymentResult, error) {
	providerNo := "MOCKPAY-" + req.OrderNo
	m.mu.Lock()
	status, exists := m.statuses[providerNo]
	if !exists {
		status = PaymentPending
		m.statuses[providerNo] = status
		m.requests[providerNo] = req
	}
	m.mu.Unlock()
	return CreatePaymentResult{
		ProviderOrderNo: providerNo,
		Status:          status,
		PayParams: map[string]string{
			"mode":              "mock",
			"provider_order_no": providerNo,
		},
	}, nil
}

func (m *MockPayment) Confirm(providerNo string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.statuses[providerNo]; !ok {
		m.statuses[providerNo] = PaymentSuccess
		return true
	}
	if m.statuses[providerNo] == PaymentSuccess {
		return true
	}
	if m.statuses[providerNo] != PaymentPending {
		return false
	}
	m.statuses[providerNo] = PaymentSuccess
	return true
}

func (m *MockPayment) Query(_ context.Context, providerNo string) (QueryPaymentResult, error) {
	m.mu.RLock()
	status, ok := m.statuses[providerNo]
	request := m.requests[providerNo]
	m.mu.RUnlock()
	if !ok {
		return QueryPaymentResult{}, fmt.Errorf("mock payment %s not found", providerNo)
	}
	result := QueryPaymentResult{ProviderOrderNo: providerNo, Status: status, MerchantNo: request.MerchantNo, OrderNo: request.OrderNo, Amount: request.Amount}
	if status == PaymentSuccess {
		now := time.Now()
		result.PaidAt = &now
	}
	return result, nil
}

func (m *MockPayment) Close(_ context.Context, providerNo string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.statuses[providerNo]
	if !ok {
		m.statuses[providerNo] = PaymentClosed
		return nil
	}
	if status == PaymentSuccess || status == PaymentRefunded {
		return errors.New("paid payment cannot be closed")
	}
	m.statuses[providerNo] = PaymentClosed
	return nil
}

func (m *MockPayment) Refund(_ context.Context, req RefundRequest) (RefundResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.refundStatuses[req.RefundNo]; ok {
		return RefundResult{ProviderRefundNo: existing.ProviderRefundNo, Status: existing.Status}, nil
	}
	status := m.statuses[req.ProviderOrderNo]
	if status != PaymentSuccess && status != PaymentRefunded {
		return RefundResult{}, errors.New("payment is not refundable")
	}
	original, ok := m.requests[req.ProviderOrderNo]
	if !ok || original.Amount <= 0 || req.Amount <= 0 || req.MerchantNo != original.MerchantNo {
		return RefundResult{}, errors.New("invalid refund identity or amount")
	}
	refunded := int64(0)
	for _, prior := range m.refundStatuses {
		if prior.ProviderOrderNo != req.ProviderOrderNo || prior.Status != PaymentSuccess {
			continue
		}
		if prior.Amount < 0 || prior.Amount > original.Amount-refunded {
			return RefundResult{}, errors.New("mock refund history exceeds payment amount")
		}
		refunded += prior.Amount
	}
	if refunded > original.Amount || req.Amount > original.Amount-refunded {
		return RefundResult{}, errors.New("refund exceeds payment amount")
	}
	result := QueryRefundResult{
		RefundNo: req.RefundNo, ProviderRefundNo: "MOCKREF-" + req.RefundNo,
		ProviderOrderNo: req.ProviderOrderNo, MerchantNo: req.MerchantNo,
		Amount: req.Amount, Status: PaymentSuccess,
	}
	m.refundStatuses[req.RefundNo] = result
	return RefundResult{ProviderRefundNo: result.ProviderRefundNo, Status: result.Status}, nil
}

func (m *MockPayment) QueryRefund(_ context.Context, refundNo string) (QueryRefundResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, ok := m.refundStatuses[refundNo]
	if !ok {
		return QueryRefundResult{}, fmt.Errorf("mock refund %s not found", refundNo)
	}
	return result, nil
}

type TianQueConfig struct {
	BaseURL, OrgID, PrivateKey, PublicKey, NotifyURL string
}

// TianQue is a deliberate production adapter boundary. Signing and transport
// will be implemented after partner orgId, keys and sandbox access are issued.
type TianQue struct{ Config TianQueConfig }

func (t TianQue) Name() string { return "tianque" }
func (t TianQue) Create(context.Context, CreatePaymentRequest) (CreatePaymentResult, error) {
	return CreatePaymentResult{}, ErrNotConfigured
}
func (t TianQue) Query(context.Context, string) (QueryPaymentResult, error) {
	return QueryPaymentResult{}, ErrNotConfigured
}
func (t TianQue) Close(context.Context, string) error { return ErrNotConfigured }
func (t TianQue) Refund(context.Context, RefundRequest) (RefundResult, error) {
	return RefundResult{}, ErrNotConfigured
}
func (t TianQue) QueryRefund(context.Context, string) (QueryRefundResult, error) {
	return QueryRefundResult{}, ErrNotConfigured
}

type WeChatPayPartnerConfig struct {
	BaseURL, ServiceProviderMchID, ServiceProviderAppID, APICertSerialNo   string
	MerchantPrivateKey, APIV3Key, WeChatPayPublicKeyID, WeChatPayPublicKey string
	NotifyURL, RefundNotifyURL                                             string
}

// WeChatPayPartner is the API v3 ordinary service-provider boundary. Real
// signing, notification decryption and transport remain disabled until the
// service-provider account and a test sub-merchant are approved.
type WeChatPayPartner struct{ Config WeChatPayPartnerConfig }

func (w WeChatPayPartner) Name() string { return "wechat_partner" }
func (w WeChatPayPartner) Create(context.Context, CreatePaymentRequest) (CreatePaymentResult, error) {
	return CreatePaymentResult{}, ErrNotConfigured
}
func (w WeChatPayPartner) Query(context.Context, string) (QueryPaymentResult, error) {
	return QueryPaymentResult{}, ErrNotConfigured
}
func (w WeChatPayPartner) Close(context.Context, string) error { return ErrNotConfigured }
func (w WeChatPayPartner) Refund(context.Context, RefundRequest) (RefundResult, error) {
	return RefundResult{}, ErrNotConfigured
}
func (w WeChatPayPartner) QueryRefund(context.Context, string) (QueryRefundResult, error) {
	return QueryRefundResult{}, ErrNotConfigured
}
