package provider

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	wechatcore "github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/fileuploader"
	"github.com/wechatpay-apiv3/wechatpay-go/services/partnerpayments"
	partnerjsapi "github.com/wechatpay-apiv3/wechatpay-go/services/partnerpayments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// wechatPayV3Runtime is initialized lazily so a disabled provider never reads
// production key material. The official SDK performs request signing, response
// verification, callback verification/decryption and requestPayment signing.
type wechatPayV3Runtime struct {
	client           *wechatcore.Client
	notify           *notify.Handler
	privateKeyLoaded bool
	publicKey        *rsa.PublicKey
}

type WeChatPaymentNotification struct {
	ProviderOrderNo string
	MerchantNo      string
	OrderNo         string
	Amount          int64
	Status          PaymentStatus
	PaidAt          time.Time
}

type WeChatRefundNotification struct {
	ProviderOrderNo  string
	ProviderRefundNo string
	MerchantNo       string
	RefundNo         string
	Amount           int64
	Status           PaymentStatus
}

func (w *WeChatPayPartner) v3Runtime(ctx context.Context) (*wechatPayV3Runtime, error) {
	w.v3Mu.Lock()
	defer w.v3Mu.Unlock()
	if w.v3 != nil {
		return w.v3, nil
	}
	cfg := w.Config
	if strings.TrimSpace(cfg.ServiceProviderMchID) == "" || strings.TrimSpace(cfg.ServiceProviderAppID) == "" ||
		strings.TrimSpace(cfg.APICertSerialNo) == "" || strings.TrimSpace(cfg.MerchantPrivateKey) == "" ||
		strings.TrimSpace(cfg.WeChatPayPublicKeyID) == "" || strings.TrimSpace(cfg.WeChatPayPublicKey) == "" ||
		len(strings.TrimSpace(cfg.APIV3Key)) != 32 {
		return nil, fmt.Errorf("%w: WeChat Pay APIv3 credentials are incomplete", ErrNotConfigured)
	}
	privatePEM, err := loadPEMMaterial(cfg.MerchantPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("load WeChat Pay merchant private key: %w", err)
	}
	privateKey, err := utils.LoadPrivateKey(string(privatePEM))
	if err != nil {
		return nil, fmt.Errorf("parse WeChat Pay merchant private key: %w", err)
	}
	publicPEM, err := loadPEMMaterial(cfg.WeChatPayPublicKey)
	if err != nil {
		return nil, fmt.Errorf("load WeChat Pay public key: %w", err)
	}
	publicKey, err := utils.LoadPublicKey(string(publicPEM))
	if err != nil {
		return nil, fmt.Errorf("parse WeChat Pay public key: %w", err)
	}
	opts := []wechatcore.ClientOption{option.WithWechatPayPublicKeyAuthCipher(
		cfg.ServiceProviderMchID, cfg.APICertSerialNo, privateKey,
		cfg.WeChatPayPublicKeyID, publicKey,
	)}
	if w.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(w.HTTPClient))
	}
	client, err := wechatcore.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("initialize WeChat Pay APIv3 client: %w", err)
	}
	runtime := &wechatPayV3Runtime{
		client: client,
		notify: notify.NewNotifyHandler(cfg.APIV3Key,
			verifiers.NewSHA256WithRSAPubkeyVerifier(cfg.WeChatPayPublicKeyID, *publicKey)),
		privateKeyLoaded: true,
		publicKey:        publicKey,
	}
	w.v3 = runtime
	return runtime, nil
}

type WeChatApplymentResult struct {
	ApplymentID string
}

type WeChatApplymentStatus struct {
	ApplymentID string
	State       string
	StateMsg    string
	SubMchID    string
	SignURL     string
}

type wechatApplymentAPIResponse struct {
	ApplymentID int64  `json:"applyment_id"`
	State       string `json:"applyment_state"`
	StateMsg    string `json:"applyment_state_msg"`
	SubMchID    string `json:"sub_mchid"`
	SignURL     string `json:"sign_url"`
}

func decodeWechatApplymentResult(reader io.Reader) (WeChatApplymentResult, error) {
	var response wechatApplymentAPIResponse
	if err := json.NewDecoder(io.LimitReader(reader, 1<<20)).Decode(&response); err != nil {
		return WeChatApplymentResult{}, err
	}
	if response.ApplymentID <= 0 {
		return WeChatApplymentResult{}, errors.New("WeChat Pay returned an empty applyment_id")
	}
	return WeChatApplymentResult{ApplymentID: strconv.FormatInt(response.ApplymentID, 10)}, nil
}

func decodeWechatApplymentStatus(reader io.Reader) (WeChatApplymentStatus, error) {
	var response wechatApplymentAPIResponse
	if err := json.NewDecoder(io.LimitReader(reader, 1<<20)).Decode(&response); err != nil {
		return WeChatApplymentStatus{}, err
	}
	return WeChatApplymentStatus{
		ApplymentID: strconv.FormatInt(response.ApplymentID, 10), State: response.State, StateMsg: response.StateMsg,
		SubMchID: response.SubMchID, SignURL: response.SignURL,
	}, nil
}

func (w *WeChatPayPartner) UploadApplymentImage(ctx context.Context, reader io.Reader, filename, contentType string) (string, error) {
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return "", err
	}
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/bmp" {
		return "", errors.New("WeChat Pay onboarding accepts JPEG, PNG or BMP images")
	}
	response, _, err := (&fileuploader.ImageUploader{Client: runtime.client}).Upload(ctx, reader, filename, contentType)
	if err != nil {
		return "", err
	}
	if response == nil || response.MediaId == nil || *response.MediaId == "" {
		return "", errors.New("WeChat Pay returned an empty media_id")
	}
	return *response.MediaId, nil
}

func (w *WeChatPayPartner) EncryptApplymentValue(ctx context.Context, plaintext string) (string, error) {
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(plaintext) == "" {
		return "", nil
	}
	return utils.EncryptOAEPWithPublicKey(plaintext, runtime.publicKey)
}

func (w *WeChatPayPartner) SubmitApplyment(ctx context.Context, payload map[string]any) (WeChatApplymentResult, error) {
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return WeChatApplymentResult{}, err
	}
	result, err := runtime.client.Request(ctx, http.MethodPost, w.Config.BaseURL+"/v3/applyment4sub/applyment/",
		http.Header{"Wechatpay-Serial": []string{w.Config.WeChatPayPublicKeyID}}, nil, payload, "application/json")
	if err != nil {
		return WeChatApplymentResult{}, err
	}
	defer result.Response.Body.Close()
	return decodeWechatApplymentResult(result.Response.Body)
}

func (w *WeChatPayPartner) QueryApplyment(ctx context.Context, applymentID string) (WeChatApplymentStatus, error) {
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return WeChatApplymentStatus{}, err
	}
	if strings.TrimSpace(applymentID) == "" {
		return WeChatApplymentStatus{}, errors.New("applyment_id is required")
	}
	result, err := runtime.client.Get(ctx, w.Config.BaseURL+"/v3/applyment4sub/applyment/applyment_id/"+url.PathEscape(applymentID))
	if err != nil {
		return WeChatApplymentStatus{}, err
	}
	defer result.Response.Body.Close()
	return decodeWechatApplymentStatus(result.Response.Body)
}

func (w *WeChatPayPartner) APIv3Ready(ctx context.Context) (bool, string) {
	if _, err := w.v3Runtime(ctx); err != nil {
		return false, err.Error()
	}
	if strings.TrimSpace(w.Config.NotifyURL) == "" || strings.TrimSpace(w.Config.RefundNotifyURL) == "" {
		return false, "WeChat Pay callback URLs are incomplete"
	}
	return true, ""
}

func (w *WeChatPayPartner) Create(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResult, error) {
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return CreatePaymentResult{}, err
	}
	if req.MerchantNo == "" || req.OrderNo == "" || req.OpenID == "" || req.Amount <= 0 {
		return CreatePaymentResult{}, errors.New("sub-merchant number, order number, openid and positive amount are required")
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = "摊伴门店消费"
	}
	notifyURL := strings.TrimSpace(req.NotifyURL)
	if notifyURL == "" {
		notifyURL = strings.TrimSpace(w.Config.NotifyURL)
	}
	payer := &partnerjsapi.Payer{}
	requestPaymentAppID := w.Config.ServiceProviderAppID
	if req.SubAppID == "" {
		payer.SpOpenid = wechatcore.String(req.OpenID)
	} else {
		payer.SubOpenid = wechatcore.String(req.OpenID)
		requestPaymentAppID = req.SubAppID
	}
	request := partnerjsapi.PrepayRequest{
		SpAppid: wechatcore.String(w.Config.ServiceProviderAppID), SpMchid: wechatcore.String(w.Config.ServiceProviderMchID),
		SubMchid: wechatcore.String(req.MerchantNo), Description: wechatcore.String(description),
		OutTradeNo: wechatcore.String(req.OrderNo), NotifyUrl: wechatcore.String(notifyURL),
		Amount: &partnerjsapi.Amount{Total: wechatcore.Int64(req.Amount), Currency: wechatcore.String("CNY")}, Payer: payer,
	}
	if req.SubAppID != "" {
		request.SubAppid = wechatcore.String(req.SubAppID)
	}
	response, _, err := (&partnerjsapi.JsapiApiService{Client: runtime.client}).PrepayWithRequestPayment(ctx, request, requestPaymentAppID)
	if err != nil {
		return CreatePaymentResult{}, err
	}
	if response == nil || response.PrepayId == nil || response.Package == nil || response.PaySign == nil {
		return CreatePaymentResult{}, errors.New("WeChat Pay returned incomplete requestPayment parameters")
	}
	return CreatePaymentResult{
		ProviderOrderNo: wechatProviderOrderNo(req.MerchantNo, req.OrderNo), Status: PaymentPending,
		PayParams: map[string]string{
			"appId": stringValue(response.Appid), "timeStamp": stringValue(response.TimeStamp),
			"nonceStr": stringValue(response.NonceStr), "package": stringValue(response.Package),
			"signType": stringValue(response.SignType), "paySign": stringValue(response.PaySign),
		},
	}, nil
}

func (w *WeChatPayPartner) Query(ctx context.Context, providerNo string) (QueryPaymentResult, error) {
	merchantNo, orderNo, err := parseWechatProviderOrderNo(providerNo)
	if err != nil {
		return QueryPaymentResult{}, err
	}
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return QueryPaymentResult{}, err
	}
	transaction, _, err := (&partnerjsapi.JsapiApiService{Client: runtime.client}).QueryOrderByOutTradeNo(ctx,
		partnerjsapi.QueryOrderByOutTradeNoRequest{OutTradeNo: wechatcore.String(orderNo), SpMchid: wechatcore.String(w.Config.ServiceProviderMchID), SubMchid: wechatcore.String(merchantNo)})
	if err != nil {
		return QueryPaymentResult{}, err
	}
	return paymentQueryResult(providerNo, transaction)
}

func (w *WeChatPayPartner) Close(ctx context.Context, providerNo string) error {
	merchantNo, orderNo, err := parseWechatProviderOrderNo(providerNo)
	if err != nil {
		return err
	}
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return err
	}
	_, err = (&partnerjsapi.JsapiApiService{Client: runtime.client}).CloseOrder(ctx,
		partnerjsapi.CloseOrderRequest{OutTradeNo: wechatcore.String(orderNo), SpMchid: wechatcore.String(w.Config.ServiceProviderMchID), SubMchid: wechatcore.String(merchantNo)})
	return err
}

func (w *WeChatPayPartner) Refund(ctx context.Context, req RefundRequest) (RefundResult, error) {
	merchantNo, orderNo, err := parseWechatProviderOrderNo(req.ProviderOrderNo)
	if err != nil {
		return RefundResult{}, err
	}
	if req.MerchantNo != merchantNo || req.RefundNo == "" || req.Amount <= 0 || req.TotalAmount <= 0 || req.Amount > req.TotalAmount {
		return RefundResult{}, errors.New("invalid refund identity or amount")
	}
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return RefundResult{}, err
	}
	response, _, err := (&refunddomestic.RefundsApiService{Client: runtime.client}).Create(ctx, refunddomestic.CreateRequest{
		SubMchid: wechatcore.String(merchantNo), OutTradeNo: wechatcore.String(orderNo), OutRefundNo: wechatcore.String(req.RefundNo),
		Reason: wechatcore.String("商户退款"), NotifyUrl: wechatcore.String(w.Config.RefundNotifyURL),
		Amount: &refunddomestic.AmountReq{Refund: wechatcore.Int64(req.Amount), Total: wechatcore.Int64(req.TotalAmount), Currency: wechatcore.String("CNY")},
	})
	if err != nil {
		return RefundResult{}, err
	}
	return RefundResult{ProviderRefundNo: stringValue(response.RefundId), Status: refundStatus(response)}, nil
}

func (w *WeChatPayPartner) QueryRefund(ctx context.Context, req QueryRefundRequest) (QueryRefundResult, error) {
	merchantNo, orderNo, err := parseWechatProviderOrderNo(req.ProviderOrderNo)
	if err != nil {
		return QueryRefundResult{}, err
	}
	if merchantNo != req.MerchantNo || req.RefundNo == "" {
		return QueryRefundResult{}, errors.New("invalid refund query identity")
	}
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return QueryRefundResult{}, err
	}
	response, _, err := (&refunddomestic.RefundsApiService{Client: runtime.client}).QueryByOutRefundNo(ctx,
		refunddomestic.QueryByOutRefundNoRequest{OutRefundNo: wechatcore.String(req.RefundNo), SubMchid: wechatcore.String(merchantNo)})
	if err != nil {
		return QueryRefundResult{}, err
	}
	return QueryRefundResult{RefundNo: stringValue(response.OutRefundNo), ProviderRefundNo: stringValue(response.RefundId),
		ProviderOrderNo: wechatProviderOrderNo(merchantNo, orderNo), MerchantNo: merchantNo,
		Amount: int64Value(response.Amount.Refund), Status: refundStatus(response)}, nil
}

func (w *WeChatPayPartner) ParsePaymentNotification(ctx context.Context, request *http.Request) (WeChatPaymentNotification, error) {
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return WeChatPaymentNotification{}, err
	}
	transaction := new(partnerpayments.Transaction)
	message, err := runtime.notify.ParseNotifyRequest(ctx, request, transaction)
	if err != nil {
		return WeChatPaymentNotification{}, err
	}
	if message.EventType != "TRANSACTION.SUCCESS" || transaction == nil {
		return WeChatPaymentNotification{}, fmt.Errorf("unsupported WeChat Pay event %q", message.EventType)
	}
	result, err := paymentQueryResult(wechatProviderOrderNo(stringValue(transaction.SubMchid), stringValue(transaction.OutTradeNo)), transaction)
	if err != nil {
		return WeChatPaymentNotification{}, err
	}
	paidAt := time.Now()
	if result.PaidAt != nil {
		paidAt = *result.PaidAt
	}
	return WeChatPaymentNotification{ProviderOrderNo: result.ProviderOrderNo, MerchantNo: result.MerchantNo,
		OrderNo: result.OrderNo, Amount: result.Amount, Status: result.Status, PaidAt: paidAt}, nil
}

func (w *WeChatPayPartner) ParseRefundNotification(ctx context.Context, request *http.Request) (WeChatRefundNotification, error) {
	runtime, err := w.v3Runtime(ctx)
	if err != nil {
		return WeChatRefundNotification{}, err
	}
	refund := new(refunddomestic.Refund)
	message, err := runtime.notify.ParseNotifyRequest(ctx, request, refund)
	if err != nil {
		return WeChatRefundNotification{}, err
	}
	if message.EventType != "REFUND.SUCCESS" || refund == nil {
		return WeChatRefundNotification{}, fmt.Errorf("unsupported WeChat Pay refund event %q", message.EventType)
	}
	// Refund notifications do not include sub_mchid in the decrypted resource.
	// The app resolves the durable refund first and supplies that identity check.
	return WeChatRefundNotification{ProviderRefundNo: stringValue(refund.RefundId), RefundNo: stringValue(refund.OutRefundNo),
		Amount: int64Value(refund.Amount.Refund), Status: refundStatus(refund)}, nil
}

func wechatProviderOrderNo(merchantNo, orderNo string) string { return merchantNo + ":" + orderNo }

func parseWechatProviderOrderNo(value string) (string, string, error) {
	merchantNo, orderNo, ok := strings.Cut(value, ":")
	if !ok || merchantNo == "" || orderNo == "" || strings.Contains(orderNo, ":") {
		return "", "", errors.New("invalid WeChat Pay provider order number")
	}
	return merchantNo, orderNo, nil
}

func paymentQueryResult(providerNo string, transaction *partnerpayments.Transaction) (QueryPaymentResult, error) {
	if transaction == nil || transaction.SubMchid == nil || transaction.OutTradeNo == nil || transaction.Amount == nil || transaction.Amount.Total == nil {
		return QueryPaymentResult{}, errors.New("WeChat Pay returned an incomplete transaction")
	}
	merchantNo, orderNo, err := parseWechatProviderOrderNo(providerNo)
	if err != nil {
		return QueryPaymentResult{}, err
	}
	if merchantNo != stringValue(transaction.SubMchid) || orderNo != stringValue(transaction.OutTradeNo) {
		return QueryPaymentResult{}, errors.New("WeChat Pay transaction identity mismatch")
	}
	status := PaymentPending
	switch stringValue(transaction.TradeState) {
	case "SUCCESS":
		status = PaymentSuccess
	case "CLOSED", "REVOKED":
		status = PaymentClosed
	case "PAYERROR":
		status = PaymentFailed
	case "REFUND":
		status = PaymentRefunded
	}
	result := QueryPaymentResult{ProviderOrderNo: providerNo, Status: status, MerchantNo: merchantNo, OrderNo: orderNo, Amount: int64Value(transaction.Amount.Total)}
	if transaction.SuccessTime != nil && *transaction.SuccessTime != "" {
		if paidAt, parseErr := time.Parse(time.RFC3339, *transaction.SuccessTime); parseErr == nil {
			result.PaidAt = &paidAt
		}
	}
	return result, nil
}

func refundStatus(refund *refunddomestic.Refund) PaymentStatus {
	if refund == nil || refund.Status == nil {
		return PaymentPending
	}
	switch string(*refund.Status) {
	case "SUCCESS":
		return PaymentSuccess
	case "CLOSED":
		return PaymentClosed
	case "ABNORMAL":
		return PaymentFailed
	default:
		return PaymentPending
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
