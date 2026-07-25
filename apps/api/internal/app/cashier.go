package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
	"github.com/golang-jwt/jwt/v5"
)

const cashierSessionTTL = 365 * 24 * time.Hour

type cashierSettlementInput struct {
	Method string `json:"method"`
	Remark string `json:"remark"`
}

type cashierDinerCountInput struct {
	DinerCount int `json:"diner_count"`
}

type cashierHandoverInput struct {
	Remark string `json:"remark"`
}

func cashierTokenPathAllowed(path string) bool {
	for _, exact := range []string{
		"/api/v1/auth/me",
		"/api/v1/auth/workspaces",
		"/api/v1/merchant/dashboard",
		"/api/v1/merchant/table-board",
		"/api/v1/merchant/cashier/session",
		"/api/v1/merchant/cashier/context",
		"/api/v1/merchant/cashier/handover",
	} {
		if path == exact {
			return true
		}
	}
	return strings.HasPrefix(path, "/api/v1/merchant/orders/") ||
		path == "/api/v1/merchant/orders" ||
		strings.HasPrefix(path, "/api/v1/merchant/print-jobs")
}

func (s *Server) optionalCashierIdentity(ctx context.Context, r *http.Request, tenantID int64) (identity, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return identity{}, false
	}
	parsed := &claims{}
	token, err := jwt.ParseWithClaims(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), parsed, func(token *jwt.Token) (any, error) {
		return []byte(s.Config.JWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("tanban-api"))
	if err != nil || !token.Valid || parsed.TokenKind != "cashier" || parsed.TenantID != tenantID {
		return identity{}, false
	}
	userID, err := parseInt64(parsed.Subject)
	if err != nil {
		return identity{}, false
	}
	username, displayName, accountErr := s.loadActiveAccount(ctx, userID)
	workspace, workspaceErr := s.loadMerchantWorkspace(ctx, userID, tenantID)
	if accountErr != nil || workspaceErr != nil || workspace.Role != parsed.Role ||
		parsed.MembershipID > 0 && workspace.MembershipID != parsed.MembershipID {
		return identity{}, false
	}
	return workspaceIdentity(userID, username, displayName, workspace), true
}

func (s *Server) createCashierSession(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		MembershipID: actor.MembershipID,
		TenantID:     actor.TenantID,
		Role:         actor.Role,
		Username:     actor.Username,
		TokenKind:    "cashier",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   int64String(actor.UserID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cashierSessionTTL)),
			Issuer:    "tanban-api",
		},
	}).SignedString([]byte(s.Config.JWTSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "无法创建收银终端会话")
		return
	}
	s.audit(r.Context(), actor, "cashier.session.issue", "account", int64String(actor.UserID), map[string]any{
		"expires_at": now.Add(cashierSessionTTL).UTC().Format(time.RFC3339),
	}, r)
	writeData(w, http.StatusOK, map[string]any{
		"accessToken": token,
		"tokenType":   "Bearer",
		"expiresIn":   int64(cashierSessionTTL.Seconds()),
		"expiresAt":   now.Add(cashierSessionTTL).UTC().Format(time.RFC3339),
	})
}

func (s *Server) getCashierContext(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var storeCode, storeName, logoURL string
	err = s.DB.QueryRowContext(r.Context(), `SELECT code,name,logo_url FROM stores
		WHERE id=? AND tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL`, storeID, actor.TenantID).
		Scan(&storeCode, &storeName, &logoURL)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"storeId": storeID, "storeCode": storeCode, "storeName": storeName, "logoUrl": logoURL,
		"operatorName": actor.DisplayName, "role": actor.Role,
	})
}

func (s *Server) handoverCashierSession(w http.ResponseWriter, r *http.Request) {
	var input cashierHandoverInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Remark = strings.TrimSpace(input.Remark)
	if len([]rune(input.Remark)) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "remark must not exceed 255 characters")
		return
	}
	actor := currentIdentity(r.Context())
	handedOverAt := time.Now()
	s.audit(r.Context(), actor, "cashier.shift.handover", "account", int64String(actor.UserID), map[string]any{
		"remark":         input.Remark,
		"handed_over_at": handedOverAt.UTC().Format(time.RFC3339),
	}, r)
	writeData(w, http.StatusOK, map[string]any{
		"handedOverAt": handedOverAt.UTC().Format(time.RFC3339),
		"operatorName": actor.DisplayName,
	})
}

// cashierSettlePayBeforeOrder records cash or an external receipt for a
// pay-before order. Pay-after bills continue through settlePayAfterOrder so the
// two settlement state machines remain explicit and auditable.
func (s *Server) cashierSettlePayBeforeOrder(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input cashierSettlementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	input.Remark = strings.TrimSpace(input.Remark)
	if !validStatus(input.Method, "CASH", "EXTERNAL") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "method must be CASH or EXTERNAL")
		return
	}
	if len([]rune(input.Remark)) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "remark must not exceed 255 characters")
		return
	}
	actor := currentIdentity(r.Context())
	conn, release, err := s.acquirePaymentOrderLock(r.Context(), actor.TenantID, orderID)
	if err != nil {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", err.Error())
		return
	}
	defer release()

	var storeID, totalCents, paidCents int64
	var status, paymentStatus, settlementMode string
	err = conn.QueryRowContext(r.Context(), `SELECT store_id,total_cents,paid_cents,status,payment_status,settlement_mode_snapshot
		FROM orders WHERE id=? AND tenant_id=?`, orderID, actor.TenantID).
		Scan(&storeID, &totalCents, &paidCents, &status, &paymentStatus, &settlementMode)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if paymentStatus == "PAID" {
		s.getOrderByID(w, r, actor.TenantID, orderID)
		return
	}
	if settlementMode != "PAY_BEFORE" || status != "PENDING_PAYMENT" || paymentStatus != "UNPAID" {
		writeError(w, http.StatusConflict, "ORDER_NOT_SETTLEABLE", "当前订单不是可由收银台结账的先付订单")
		return
	}
	var pendingPayments int
	if err = conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_transactions
		WHERE tenant_id=? AND order_id=? AND status IN ('CREATING','PENDING')`, actor.TenantID, orderID).
		Scan(&pendingPayments); err != nil {
		handleSQLError(w, err)
		return
	}
	if pendingPayments > 0 {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", "订单已有支付处理中，请确认结果后再操作")
		return
	}
	if _, err = ensureOrderStockReservationLocked(r.Context(), conn, actor.TenantID, orderID); errors.Is(err, errInsufficientStock) {
		writeError(w, http.StatusConflict, "ITEM_UNAVAILABLE", "库存已变化，请调整订单后重试")
		return
	} else if err != nil {
		handleSQLError(w, err)
		return
	}
	remainingCents := totalCents - paidCents
	if remainingCents <= 0 {
		writeError(w, http.StatusConflict, "ORDER_NOT_SETTLEABLE", "订单已无待收金额")
		return
	}
	reference := newBusinessNo("POS")
	providerName := "external"
	if input.Method == "CASH" {
		providerName = "offline_cash"
	}
	raw, _ := json.Marshal(map[string]any{"method": input.Method, "remark": input.Remark, "confirmedBy": actor.UserID})
	_, err = conn.ExecContext(r.Context(), `INSERT INTO payment_transactions(
		tenant_id,store_id,order_id,provider,provider_request_no,provider_order_no,amount_cents,status,raw_response
	) VALUES(?,?,?,?,?,?,?,'PENDING',?)`,
		actor.TenantID, storeID, orderID, providerName, reference, reference, remainingCents, string(raw))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if err = s.markPaymentPaidLocked(r.Context(), conn, providerName, reference, time.Now()); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "cashier.order.settle", "order", int64String(orderID), map[string]any{
		"method": input.Method, "amount_cents": remainingCents, "provider_status": provider.PaymentSuccess,
	}, r)
	s.getOrderByID(w, r, actor.TenantID, orderID)
}

func (s *Server) updateCashierOrderDinerCount(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input cashierDinerCountInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.DinerCount < 1 || input.DinerCount > 99 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "diner_count must be between 1 and 99")
		return
	}
	actor := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), `UPDATE orders SET diner_count=?,updated_at=NOW(3)
		WHERE id=? AND tenant_id=? AND order_type='DINE_IN'
		  AND status IN ('PENDING_PAYMENT','PAID','ACCEPTED','PREPARING','READY')`,
		input.DinerCount, orderID, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "ORDER_NOT_EDITABLE", "当前订单不能修改就餐人数")
		return
	}
	s.audit(r.Context(), actor, "cashier.order.diner_count.update", "order", int64String(orderID), map[string]any{
		"diner_count": input.DinerCount,
	}, r)
	s.getOrderByID(w, r, actor.TenantID, orderID)
}
