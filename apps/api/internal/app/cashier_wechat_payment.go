package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

const wechatCodePaymentMethod = "WECHAT_MICROPAY"

type wechatCodePaymentInput struct {
	AuthCode string `json:"auth_code"`
	DeviceID string `json:"device_id"`
}

type wechatCodePayableOrder struct {
	OrderNo, Status, PaymentStatus, SettlementMode                 string
	MerchantNo, SubAppID, TenantPaymentProvider, OnboardingStatus  string
	ProductAuthorizationStatus, StoreCode, StoreName, StoreAddress string
	StoreID, TotalCents, PaidCents, CustomerID                     int64
}

func (s *Server) createWechatCodePayment(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input wechatCodePaymentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.AuthCode = strings.TrimSpace(input.AuthCode)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	if !validWechatPaymentCode(input.AuthCode) {
		writeError(w, http.StatusBadRequest, "WECHAT_AUTH_CODE_INVALID", "请扫描微信付款码；付款码应为 18 位数字")
		return
	}
	if len([]rune(input.DeviceID)) > 32 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "device_id must not exceed 32 characters")
		return
	}
	codeProvider, ok := s.Payment.(provider.PaymentCodeProvider)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "WECHAT_CODE_PAY_NOT_AVAILABLE", "服务器尚未启用微信付款码支付")
		return
	}
	if ready, reason := codeProvider.CodePaymentReady(); !ready {
		writeError(w, http.StatusServiceUnavailable, "WECHAT_CODE_PAY_NOT_CONFIGURED", reason)
		return
	}
	actor := currentIdentity(r.Context())
	conn, release, err := s.acquirePaymentOrderLock(r.Context(), actor.TenantID, orderID)
	if err != nil {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", err.Error())
		return
	}
	defer release()

	order, err := s.loadWechatCodePayableOrder(r.Context(), conn, actor.TenantID, orderID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if errorCode, message := validateWechatCodePayableOrder(order, s.Payment.Name()); errorCode != "" {
		writeError(w, http.StatusConflict, errorCode, message)
		return
	}
	enabled, err := s.paymentAcceptanceEnabled(r.Context())
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if !enabled {
		writeError(w, http.StatusServiceUnavailable, "PAYMENTS_DISABLED", "平台已暂停支付受理")
		return
	}
	if order.SettlementMode == "PAY_BEFORE" {
		if _, reserveErr := ensureOrderStockReservationLocked(r.Context(), conn, actor.TenantID, orderID); errors.Is(reserveErr, errInsufficientStock) {
			writeError(w, http.StatusConflict, "ITEM_UNAVAILABLE", "库存已变化，请调整订单后重试")
			return
		} else if reserveErr != nil {
			handleSQLError(w, reserveErr)
			return
		}
	}
	balancePayment, err := s.applyOrderBalancePaymentLocked(r.Context(), conn, actor.TenantID, orderID, order.CustomerID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if balancePayment.FullyPaid {
		writeData(w, http.StatusOK, map[string]any{
			"status": "SUCCESS", "paymentMethod": "BALANCE", "message": "会员余额已完成本单支付",
			"needCustomerAction": false,
		})
		return
	}

	var activeID int64
	var activeStatus, activeMethod string
	err = conn.QueryRowContext(r.Context(), `SELECT id,status,payment_method FROM payment_transactions
		WHERE tenant_id=? AND order_id=? AND status IN ('CREATING','PENDING')
		ORDER BY id DESC LIMIT 1`, actor.TenantID, orderID).Scan(&activeID, &activeStatus, &activeMethod)
	if err == nil {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", "订单已有支付处理中，请先确认该笔支付结果")
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	remainingCents := order.TotalCents - order.PaidCents
	if balancePayment.AmountCents > 0 {
		remainingCents = balancePayment.RemainingCents
	}
	if remainingCents <= 0 {
		writeError(w, http.StatusConflict, "ORDER_NOT_SETTLEABLE", "订单已无待收金额")
		return
	}

	providerRequestNo := newBusinessNo("MP")
	initialRaw, _ := json.Marshal(map[string]any{
		"phase": "submitting", "method": wechatCodePaymentMethod, "confirmedBy": actor.UserID,
	})
	result, err := conn.ExecContext(r.Context(), `INSERT INTO payment_transactions(
		tenant_id,store_id,order_id,provider,payment_method,merchant_no,sub_appid,provider_request_no,
		provider_order_no,device_info,amount_cents,status,raw_response
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,'CREATING',?)`,
		actor.TenantID, order.StoreID, orderID, s.Payment.Name(), wechatCodePaymentMethod,
		order.MerchantNo, order.SubAppID, providerRequestNo, providerRequestNo, input.DeviceID,
		remainingCents, string(initialRaw))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	paymentID, _ := result.LastInsertId()

	providerCtx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	paymentResult, providerErr := codeProvider.PayCode(providerCtx, provider.CodePaymentRequest{
		MerchantNo: order.MerchantNo, OrderNo: providerRequestNo, Amount: remainingCents,
		AuthCode: input.AuthCode, Description: order.StoreName + " " + order.OrderNo,
		SubAppID: order.SubAppID, DeviceID: input.DeviceID, ServerIP: s.Config.WeChatPayPartner.ServerIP,
		StoreID: order.StoreCode, StoreName: order.StoreName, StoreAddress: order.StoreAddress,
	})
	cancel()
	// The customer payment code is intentionally not retained beyond the
	// provider call. Persist only the provider's sanitized response.
	rawResponse, _ := json.Marshal(map[string]any{
		"status": paymentResult.Status, "error_code": paymentResult.ErrorCode,
		"message": paymentResult.Message, "response": paymentResult.Raw,
	})
	if providerErr != nil {
		_, _ = conn.ExecContext(r.Context(), `UPDATE payment_transactions SET status='PENDING',raw_response=?,updated_at=NOW(3)
			WHERE id=? AND tenant_id=? AND status='CREATING'`, string(rawResponse), paymentID, actor.TenantID)
		s.Logger.Warn("WeChat code payment outcome requires query", "payment_id", paymentID, "order_id", orderID, "error", truncateError(providerErr))
		s.audit(r.Context(), actor, "cashier.wechat_code_pay.pending", "order", int64String(orderID), map[string]any{
			"payment_id": paymentID, "amount_cents": remainingCents, "reason": truncateError(providerErr),
		}, r)
		writeWechatCodePaymentResult(w, http.StatusAccepted, paymentID, paymentResult)
		return
	}
	localStatus := string(paymentResult.Status)
	if localStatus == "" {
		localStatus = string(provider.PaymentPending)
	}
	if _, err = conn.ExecContext(r.Context(), `UPDATE payment_transactions SET status=?,provider_transaction_no=?,
		raw_response=?,updated_at=NOW(3) WHERE id=? AND tenant_id=? AND status='CREATING'`,
		localStatus, paymentResult.ProviderTransactionNo, string(rawResponse), paymentID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if paymentResult.Status == provider.PaymentSuccess {
		paidAt := time.Now()
		if paymentResult.PaidAt != nil {
			paidAt = *paymentResult.PaidAt
		}
		if err = s.markPaymentPaidLocked(r.Context(), conn, s.Payment.Name(), providerRequestNo, paidAt); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	s.audit(r.Context(), actor, "cashier.wechat_code_pay", "order", int64String(orderID), map[string]any{
		"payment_id": paymentID, "amount_cents": remainingCents, "status": localStatus,
	}, r)
	statusCode := http.StatusOK
	if paymentResult.Status == provider.PaymentPending {
		statusCode = http.StatusAccepted
	}
	writeWechatCodePaymentResult(w, statusCode, paymentID, paymentResult)
}

func (s *Server) getWechatCodePaymentStatus(w http.ResponseWriter, r *http.Request) {
	s.resolveWechatCodePayment(w, r, false)
}

func (s *Server) cancelWechatCodePayment(w http.ResponseWriter, r *http.Request) {
	s.resolveWechatCodePayment(w, r, true)
}

func (s *Server) resolveWechatCodePayment(w http.ResponseWriter, r *http.Request, cancelPayment bool) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	codeProvider, ok := s.Payment.(provider.PaymentCodeProvider)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "WECHAT_CODE_PAY_NOT_AVAILABLE", "服务器尚未启用微信付款码支付")
		return
	}
	actor := currentIdentity(r.Context())
	conn, release, err := s.acquirePaymentOrderLock(r.Context(), actor.TenantID, orderID)
	if err != nil {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", err.Error())
		return
	}
	defer release()

	var paymentID int64
	var merchantNo, providerNo, status string
	var createdAt time.Time
	err = conn.QueryRowContext(r.Context(), `SELECT id,merchant_no,provider_order_no,status,created_at
		FROM payment_transactions WHERE tenant_id=? AND order_id=? AND provider=? AND payment_method=?
		ORDER BY id DESC LIMIT 1`, actor.TenantID, orderID, s.Payment.Name(), wechatCodePaymentMethod).
		Scan(&paymentID, &merchantNo, &providerNo, &status, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "WECHAT_CODE_PAYMENT_NOT_FOUND", "该订单没有微信付款码支付记录")
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if status == string(provider.PaymentSuccess) {
		writeWechatCodePaymentResult(w, http.StatusOK, paymentID, provider.CodePaymentResult{
			ProviderOrderNo: providerNo, Status: provider.PaymentSuccess, Message: "微信支付成功",
		})
		return
	}
	if status == string(provider.PaymentFailed) || status == string(provider.PaymentClosed) {
		writeWechatCodePaymentResult(w, http.StatusOK, paymentID, provider.CodePaymentResult{
			ProviderOrderNo: providerNo, Status: provider.PaymentStatus(status),
			Message: map[bool]string{true: "本次付款已关闭", false: "本次付款失败"}[status == string(provider.PaymentClosed)],
		})
		return
	}

	queryCtx, queryCancel := context.WithTimeout(r.Context(), 8*time.Second)
	queryResult, queryErr := codeProvider.QueryCode(queryCtx, provider.QueryCodePaymentRequest{
		MerchantNo: merchantNo, OrderNo: providerNo,
	})
	queryCancel()
	if queryErr == nil {
		if err = s.recordWechatCodeQueryResult(r.Context(), conn, actor.TenantID, paymentID, providerNo, queryResult); err != nil {
			handleSQLError(w, err)
			return
		}
		if queryResult.Status == provider.PaymentSuccess || queryResult.Status == provider.PaymentFailed ||
			queryResult.Status == provider.PaymentClosed || !cancelPayment {
			writeWechatCodePaymentResult(w, statusCodeForWechatPayment(queryResult.Status), paymentID, queryResult)
			return
		}
	} else if !cancelPayment {
		s.Logger.Warn("query WeChat code payment", "payment_id", paymentID, "error", truncateError(queryErr))
		writeWechatCodePaymentResult(w, http.StatusAccepted, paymentID, provider.CodePaymentResult{
			ProviderOrderNo: providerNo, Status: provider.PaymentPending, Message: "仍在确认微信支付结果",
		})
		return
	}

	elapsed := time.Since(createdAt)
	if elapsed < 15*time.Second {
		retryAfter := int64((15*time.Second - elapsed + time.Second - 1) / time.Second)
		writeData(w, http.StatusAccepted, map[string]any{
			"paymentId": paymentID, "status": "PENDING", "message": "支付提交后需至少等待 15 秒再撤销",
			"retryAfterSeconds": retryAfter, "needCustomerAction": queryResult.NeedCustomerAction,
		})
		return
	}
	reverseCtx, reverseCancel := context.WithTimeout(r.Context(), 10*time.Second)
	reverseResult, reverseErr := codeProvider.ReverseCode(reverseCtx, provider.ReverseCodePaymentRequest{
		MerchantNo: merchantNo, OrderNo: providerNo,
	})
	reverseCancel()
	if reverseErr == nil && reverseResult.Status == provider.PaymentClosed {
		_, err = conn.ExecContext(r.Context(), `UPDATE payment_transactions SET status='CLOSED',updated_at=NOW(3)
			WHERE id=? AND tenant_id=? AND status IN ('CREATING','PENDING')`, paymentID, actor.TenantID)
		if err != nil {
			handleSQLError(w, err)
			return
		}
		s.audit(r.Context(), actor, "cashier.wechat_code_pay.reverse", "order", int64String(orderID), map[string]any{
			"payment_id": paymentID, "status": reverseResult.Status,
		}, r)
		writeWechatCodePaymentResult(w, http.StatusOK, paymentID, provider.CodePaymentResult{
			ProviderOrderNo: providerNo, Status: provider.PaymentClosed, Message: reverseResult.Message,
		})
		return
	}
	message := reverseResult.Message
	if message == "" {
		message = "撤销结果仍需确认，系统将继续查单"
	}
	if reverseErr != nil {
		s.Logger.Warn("reverse WeChat code payment", "payment_id", paymentID, "error", truncateError(reverseErr))
	}
	writeWechatCodePaymentResult(w, http.StatusAccepted, paymentID, provider.CodePaymentResult{
		ProviderOrderNo: providerNo, Status: provider.PaymentPending, Message: message,
		NeedCustomerAction: reverseResult.ErrorCode == "USERPAYING",
	})
}

func (s *Server) recordWechatCodeQueryResult(ctx context.Context, conn *sql.Conn, tenantID, paymentID int64, providerNo string, result provider.CodePaymentResult) error {
	raw, _ := json.Marshal(map[string]any{
		"status": result.Status, "error_code": result.ErrorCode, "message": result.Message, "response": result.Raw,
	})
	if _, err := conn.ExecContext(ctx, `UPDATE payment_transactions SET status=?,provider_transaction_no=?,
		raw_response=?,updated_at=NOW(3) WHERE id=? AND tenant_id=? AND status IN ('CREATING','PENDING')`,
		string(result.Status), result.ProviderTransactionNo, string(raw), paymentID, tenantID); err != nil {
		return err
	}
	if result.Status == provider.PaymentSuccess {
		paidAt := time.Now()
		if result.PaidAt != nil {
			paidAt = *result.PaidAt
		}
		return s.markPaymentPaidLocked(ctx, conn, s.Payment.Name(), providerNo, paidAt)
	}
	return nil
}

func (s *Server) loadWechatCodePayableOrder(ctx context.Context, conn *sql.Conn, tenantID, orderID int64) (wechatCodePayableOrder, error) {
	var order wechatCodePayableOrder
	err := conn.QueryRowContext(ctx, `SELECT o.order_no,o.store_id,o.total_cents,o.paid_cents,o.status,o.payment_status,
		o.settlement_mode_snapshot,COALESCE(o.customer_id,0),t.payment_provider,t.payment_merchant_no,t.payment_sub_appid,
		t.payment_onboarding_status,t.payment_product_authorization_status,st.code,st.name,st.address
		FROM orders o
		JOIN tenants t ON t.id=o.tenant_id AND t.status='ACTIVE' AND t.deleted_at IS NULL
		JOIN stores st ON st.id=o.store_id AND st.tenant_id=o.tenant_id AND st.status='ACTIVE' AND st.deleted_at IS NULL
		WHERE o.id=? AND o.tenant_id=?`,
		orderID, tenantID).Scan(&order.OrderNo, &order.StoreID, &order.TotalCents, &order.PaidCents,
		&order.Status, &order.PaymentStatus, &order.SettlementMode, &order.CustomerID,
		&order.TenantPaymentProvider, &order.MerchantNo, &order.SubAppID, &order.OnboardingStatus,
		&order.ProductAuthorizationStatus, &order.StoreCode, &order.StoreName, &order.StoreAddress)
	return order, err
}

func validateWechatCodePayableOrder(order wechatCodePayableOrder, activeProvider string) (string, string) {
	if order.PaymentStatus != "UNPAID" {
		return "ORDER_ALREADY_PAID", "订单已经完成付款"
	}
	payable := order.Status == "PENDING_PAYMENT"
	if order.SettlementMode == "PAY_AFTER" {
		payable = validStatus(order.Status, "PAID", "ACCEPTED", "PREPARING", "READY")
	}
	if !payable {
		return "ORDER_NOT_SETTLEABLE", "当前订单状态不能结账"
	}
	if order.TenantPaymentProvider != "wechat_partner" || activeProvider != "wechat_partner" {
		return "WECHAT_CODE_PAY_NOT_AVAILABLE", "当前商户尚未启用微信支付服务商"
	}
	if order.MerchantNo == "" || order.OnboardingStatus != "ACTIVE" || order.ProductAuthorizationStatus != "AUTHORIZED" {
		return "WECHAT_PAY_MERCHANT_NOT_READY", "特约商户进件或付款码支付产品授权尚未完成"
	}
	return "", ""
}

func validWechatPaymentCode(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 18 || value[:2] < "10" || value[:2] > "15" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func writeWechatCodePaymentResult(w http.ResponseWriter, statusCode int, paymentID int64, result provider.CodePaymentResult) {
	writeData(w, statusCode, map[string]any{
		"paymentId": paymentID, "providerOrderNo": result.ProviderOrderNo,
		"providerTransactionNo": result.ProviderTransactionNo, "status": result.Status,
		"errorCode": result.ErrorCode, "message": result.Message,
		"needCustomerAction": result.NeedCustomerAction,
	})
}

func statusCodeForWechatPayment(status provider.PaymentStatus) int {
	if status == provider.PaymentPending {
		return http.StatusAccepted
	}
	return http.StatusOK
}
