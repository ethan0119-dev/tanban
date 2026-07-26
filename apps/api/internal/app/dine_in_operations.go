package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type transferTableInput struct {
	TargetTableID int64  `json:"target_table_id"`
	Remark        string `json:"remark"`
}

type mergeOrderInput struct {
	SourceOrderID int64  `json:"source_order_id"`
	Remark        string `json:"remark"`
}

type splitSettlementInput struct {
	AmountCents int64  `json:"amount_cents"`
	Method      string `json:"method"`
	Remark      string `json:"remark"`
}

type returnRequestInput struct {
	OrderItemID int64  `json:"order_item_id"`
	Quantity    int    `json:"quantity"`
	Reason      string `json:"reason"`
}

type reviewReturnInput struct {
	Action string `json:"action"`
	Remark string `json:"remark"`
}

type offlineReconciliationInput struct {
	BusinessDate        string `json:"business_date"`
	ActualCashCents     int64  `json:"actual_cash_cents"`
	ActualExternalCents int64  `json:"actual_external_cents"`
	Note                string `json:"note"`
}

func validDineInOperationOrder(orderType, settlementMode, paymentStatus, status string, paidCents int64) bool {
	return orderType == orderTypeDineIn && settlementMode == "PAY_AFTER" && paymentStatus == "UNPAID" &&
		paidCents == 0 && validStatus(status, "PAID", "ACCEPTED", "PREPARING", "READY")
}

func validDineInItemChangeOrder(orderType, settlementMode, paymentStatus, status string, paidCents int64) bool {
	if orderType != orderTypeDineIn || paymentStatus != "UNPAID" || paidCents != 0 {
		return false
	}
	if settlementMode == "PAY_BEFORE" {
		return status == "PENDING_PAYMENT"
	}
	return settlementMode == "PAY_AFTER" && validStatus(status, "PAID", "ACCEPTED", "PREPARING", "READY")
}

func earlierDineInStatus(left, right string) string {
	rank := map[string]int{"PAID": 0, "ACCEPTED": 1, "PREPARING": 2, "READY": 3}
	if rank[right] < rank[left] {
		return right
	}
	return left
}

func (s *Server) transferOrderTable(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input transferTableInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Remark = strings.TrimSpace(input.Remark)
	if input.TargetTableID <= 0 || len([]rune(input.Remark)) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "target_table_id is required and remark must not exceed 255 characters")
		return
	}
	actor := currentIdentity(r.Context())
	conn, release, err := s.acquirePaymentOrderLock(r.Context(), actor.TenantID, orderID)
	if err != nil {
		writeError(w, http.StatusConflict, "ORDER_OPERATION_IN_PROGRESS", err.Error())
		return
	}
	defer release()
	tx, err := conn.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()

	var storeID, currentTableID, paidCents int64
	var orderType, settlementMode, paymentStatus, status, currentTableName string
	err = tx.QueryRowContext(r.Context(), `SELECT store_id,COALESCE(table_id,0),table_name_snapshot,order_type,settlement_mode_snapshot,payment_status,status,paid_cents
		FROM orders WHERE id=? AND tenant_id=? FOR UPDATE`, orderID, actor.TenantID).
		Scan(&storeID, &currentTableID, &currentTableName, &orderType, &settlementMode, &paymentStatus, &status, &paidCents)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if !validDineInOperationOrder(orderType, settlementMode, paymentStatus, status, paidCents) {
		writeError(w, http.StatusConflict, "ORDER_NOT_TRANSFERABLE", "仅未收款的后付账堂食订单可以转台")
		return
	}
	var activePayment int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_transactions
		WHERE tenant_id=? AND order_id=? AND status IN ('CREATING','PENDING','SUCCESS')`,
		actor.TenantID, orderID).Scan(&activePayment); err != nil {
		handleSQLError(w, err)
		return
	}
	if activePayment > 0 {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", "订单已开始支付，请确认支付结果后再转台")
		return
	}
	if currentTableID == input.TargetTableID {
		writeError(w, http.StatusConflict, "TABLE_UNCHANGED", "目标桌台与当前桌台相同")
		return
	}
	var targetName, targetCode, targetScene, targetArea string
	err = tx.QueryRowContext(r.Context(), `SELECT t.name,t.table_code,t.public_scene,a.name
		FROM table_codes t JOIN table_areas a ON a.id=t.area_id AND a.tenant_id=t.tenant_id
		WHERE t.id=? AND t.tenant_id=? AND t.store_id=? AND t.status='ACTIVE' AND t.deleted_at IS NULL
		  AND a.status='ACTIVE' AND a.deleted_at IS NULL FOR UPDATE`,
		input.TargetTableID, actor.TenantID, storeID).Scan(&targetName, &targetCode, &targetScene, &targetArea)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var occupied int
	err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM orders WHERE tenant_id=? AND store_id=? AND table_id=?
		AND id<>? AND order_type='DINE_IN' AND status IN ('PENDING_PAYMENT','PAID','ACCEPTED','PREPARING','READY')`,
		actor.TenantID, storeID, input.TargetTableID, orderID).Scan(&occupied)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if occupied > 0 {
		writeError(w, http.StatusConflict, "TABLE_OCCUPIED", "目标桌台已有进行中的订单，请先并台或选择空闲桌台")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE orders SET table_id=?,table_public_id_snapshot=?,table_area_name_snapshot=?,
		table_name_snapshot=?,table_code_snapshot=?,updated_at=NOW(3) WHERE id=? AND tenant_id=?`,
		input.TargetTableID, targetScene, targetArea, targetName, targetCode, orderID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO order_table_events(
		tenant_id,store_id,order_id,event_type,from_table_id,from_table_name,to_table_id,to_table_name,remark,created_by
	) VALUES(?,?,?,'TRANSFER',?,?,?,?,?,?)`,
		actor.TenantID, storeID, orderID, nullableID(currentTableID), currentTableName, input.TargetTableID, targetName, input.Remark, actor.UserID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "order.transfer_table", "order", int64String(orderID), input, r)
	s.getOrderByID(w, r, actor.TenantID, orderID)
}

func (s *Server) mergeDineInOrders(w http.ResponseWriter, r *http.Request) {
	targetOrderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input mergeOrderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Remark = strings.TrimSpace(input.Remark)
	if input.SourceOrderID <= 0 || input.SourceOrderID == targetOrderID || len([]rune(input.Remark)) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "source_order_id must reference a different order and remark must not exceed 255 characters")
		return
	}
	actor := currentIdentity(r.Context())
	conn, release, err := s.acquireDineInOrderLocks(r.Context(), actor.TenantID, targetOrderID, input.SourceOrderID)
	if err != nil {
		writeError(w, http.StatusConflict, "ORDER_OPERATION_IN_PROGRESS", err.Error())
		return
	}
	defer release()
	tx, err := conn.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()

	type mergeOrder struct {
		id, storeID, tableID, total, merchandise, storeDiscount, couponDiscount, memberDiscount, paid, additions, diners int64
		orderType, settlementMode, paymentStatus, status, tableName                                                      string
	}
	orders := map[int64]*mergeOrder{targetOrderID: {id: targetOrderID}, input.SourceOrderID: {id: input.SourceOrderID}}
	ids := []int64{targetOrderID, input.SourceOrderID}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		row := orders[id]
		err = tx.QueryRowContext(r.Context(), `SELECT store_id,COALESCE(table_id,0),table_name_snapshot,total_cents,
			merchandise_subtotal_cents,store_promotion_discount_cents,coupon_discount_cents,member_discount_cents,
			paid_cents,addition_count,diner_count,order_type,settlement_mode_snapshot,payment_status,status
			FROM orders WHERE id=? AND tenant_id=? FOR UPDATE`, id, actor.TenantID).
			Scan(&row.storeID, &row.tableID, &row.tableName, &row.total, &row.merchandise, &row.storeDiscount,
				&row.couponDiscount, &row.memberDiscount, &row.paid, &row.additions, &row.diners,
				&row.orderType, &row.settlementMode, &row.paymentStatus, &row.status)
		if err != nil {
			handleSQLError(w, err)
			return
		}
		if !validDineInOperationOrder(row.orderType, row.settlementMode, row.paymentStatus, row.status, row.paid) {
			writeError(w, http.StatusConflict, "ORDER_NOT_MERGEABLE", "仅同门店、未收款的后付账堂食订单可以并台")
			return
		}
	}
	target, source := orders[targetOrderID], orders[input.SourceOrderID]
	if target.storeID != source.storeID {
		writeError(w, http.StatusConflict, "ORDER_STORE_MISMATCH", "不同门店的订单不能并台")
		return
	}
	if source.total > maxCatalogOrderCents-target.total {
		writeError(w, http.StatusConflict, "ORDER_AMOUNT_LIMIT", "并台后的账单金额超过系统上限")
		return
	}
	var pending int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_transactions WHERE tenant_id=? AND order_id IN (?,?)
		AND status IN ('CREATING','PENDING','SUCCESS')`, actor.TenantID, targetOrderID, input.SourceOrderID).Scan(&pending); err != nil {
		handleSQLError(w, err)
		return
	}
	if pending > 0 {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", "订单存在进行中的支付，请确认结果后再并台")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE order_items SET addition_sequence=addition_sequence+?
		WHERE tenant_id=? AND order_id=?`, target.additions, actor.TenantID, input.SourceOrderID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE order_addition_requests SET addition_sequence=addition_sequence+?
		WHERE tenant_id=? AND order_id=?`, target.additions, actor.TenantID, input.SourceOrderID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE order_items SET order_id=? WHERE tenant_id=? AND order_id=?",
		targetOrderID, actor.TenantID, input.SourceOrderID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE order_addition_requests SET order_id=? WHERE tenant_id=? AND order_id=?",
		targetOrderID, actor.TenantID, input.SourceOrderID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE order_return_requests SET order_id=? WHERE tenant_id=? AND order_id=?",
		targetOrderID, actor.TenantID, input.SourceOrderID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE customer_coupons SET order_id=? WHERE tenant_id=? AND order_id=? AND status='RESERVED'",
		targetOrderID, actor.TenantID, input.SourceOrderID); err != nil {
		handleSQLError(w, err)
		return
	}
	mergedStatus := earlierDineInStatus(target.status, source.status)
	if _, err = tx.ExecContext(r.Context(), `UPDATE orders SET total_cents=total_cents+?,merchandise_subtotal_cents=merchandise_subtotal_cents+?,
		store_promotion_discount_cents=store_promotion_discount_cents+?,coupon_discount_cents=coupon_discount_cents+?,
		member_discount_cents=member_discount_cents+?,addition_count=addition_count+?,diner_count=diner_count+?,
		status=?,updated_at=NOW(3)
		WHERE id=? AND tenant_id=?`, source.total, source.merchandise, source.storeDiscount, source.couponDiscount,
		source.memberDiscount, source.additions, source.diners, mergedStatus, targetOrderID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE orders SET status='CLOSED',inventory_reserved=0,stock_reserved_at=NULL,
		closed_at=NOW(3),remark=CONCAT(remark,IF(remark='','','；'),'已并入订单 #',?),updated_at=NOW(3)
		WHERE id=? AND tenant_id=?`, targetOrderID, input.SourceOrderID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO order_table_events(
		tenant_id,store_id,order_id,related_order_id,event_type,from_table_id,from_table_name,to_table_id,to_table_name,remark,created_by
	) VALUES(?,?,?,?,'MERGE',?,?,?,?,?,?)`, actor.TenantID, target.storeID, targetOrderID, input.SourceOrderID,
		nullableID(source.tableID), source.tableName, nullableID(target.tableID), target.tableName, input.Remark, actor.UserID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "order.merge", "order", int64String(targetOrderID), input, r)
	s.getOrderByID(w, r, actor.TenantID, targetOrderID)
}

func (s *Server) splitSettleOrder(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	key, ok := requiredIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input splitSettlementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	input.Remark = strings.TrimSpace(input.Remark)
	if input.AmountCents <= 0 || input.AmountCents > maxBusinessAmountCents || !validStatus(input.Method, "CASH", "EXTERNAL") || len([]rune(input.Remark)) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "amount_cents, CASH/EXTERNAL method and a remark up to 255 characters are required")
		return
	}
	actor := currentIdentity(r.Context())
	fingerprint := requestFingerprint(input)
	var existingOrderID int64
	var existingFingerprint string
	err := s.DB.QueryRowContext(r.Context(), `SELECT order_id,request_fingerprint FROM order_settlement_parts
		WHERE tenant_id=? AND idempotency_key=?`, actor.TenantID, key).Scan(&existingOrderID, &existingFingerprint)
	if err == nil {
		if existingOrderID != orderID || existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with different settlement details")
			return
		}
		s.getOrderByID(w, r, actor.TenantID, orderID)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	conn, release, err := s.acquirePaymentOrderLock(r.Context(), actor.TenantID, orderID)
	if err != nil {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", err.Error())
		return
	}
	defer release()
	tx, err := conn.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(r.Context(), `SELECT order_id,request_fingerprint FROM order_settlement_parts
		WHERE tenant_id=? AND idempotency_key=? FOR UPDATE`, actor.TenantID, key).Scan(&existingOrderID, &existingFingerprint)
	if err == nil {
		if existingOrderID != orderID || existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with different settlement details")
			return
		}
		if err = tx.Commit(); err != nil {
			handleSQLError(w, err)
			return
		}
		s.getOrderByID(w, r, actor.TenantID, orderID)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	var storeID, totalCents, paidCents int64
	var orderType, settlementMode, paymentStatus, status string
	err = tx.QueryRowContext(r.Context(), `SELECT store_id,total_cents,paid_cents,order_type,settlement_mode_snapshot,payment_status,status
		FROM orders WHERE id=? AND tenant_id=? FOR UPDATE`, orderID, actor.TenantID).
		Scan(&storeID, &totalCents, &paidCents, &orderType, &settlementMode, &paymentStatus, &status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if orderType != orderTypeDineIn || settlementMode != "PAY_AFTER" || paymentStatus != "UNPAID" ||
		!validStatus(status, "PAID", "ACCEPTED", "PREPARING", "READY") {
		writeError(w, http.StatusConflict, "ORDER_NOT_SETTLEABLE", "当前订单不能拆分收款")
		return
	}
	remaining := totalCents - paidCents
	if input.AmountCents > remaining {
		writeError(w, http.StatusConflict, "SETTLEMENT_EXCEEDS_REMAINING", "本次收款金额超过订单待收金额")
		return
	}
	var pending int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_transactions WHERE tenant_id=? AND order_id=?
		AND status IN ('CREATING','PENDING')`, actor.TenantID, orderID).Scan(&pending); err != nil {
		handleSQLError(w, err)
		return
	}
	if pending > 0 {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", "顾客正在支付，请确认支付结果后再拆分收款")
		return
	}
	var partNo int
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(part_no),0)+1 FROM order_settlement_parts
		WHERE tenant_id=? AND order_id=?`, actor.TenantID, orderID).Scan(&partNo); err != nil {
		handleSQLError(w, err)
		return
	}
	reference := newBusinessNo("SP")
	providerName := "external"
	if input.Method == "CASH" {
		providerName = "offline_cash"
	}
	raw := fmt.Sprintf(`{"kind":"SPLIT","partNo":%d,"method":%q,"remark":%q,"confirmedBy":%d}`, partNo, input.Method, input.Remark, actor.UserID)
	result, err := tx.ExecContext(r.Context(), `INSERT INTO payment_transactions(
		tenant_id,store_id,order_id,provider,provider_request_no,provider_order_no,amount_cents,status,raw_response,paid_at
	) VALUES(?,?,?,?,?,?,?,'SUCCESS',?,NOW(3))`, actor.TenantID, storeID, orderID, providerName, reference, reference, input.AmountCents, raw)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	paymentID, _ := result.LastInsertId()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO order_settlement_parts(
		tenant_id,store_id,order_id,payment_id,part_no,amount_cents,method,remark,idempotency_key,request_fingerprint,created_by
	) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, storeID, orderID, paymentID, partNo, input.AmountCents,
		input.Method, input.Remark, key, fingerprint, actor.UserID); err != nil {
		handleSQLError(w, err)
		return
	}
	final := input.AmountCents == remaining
	if final {
		if _, err = tx.ExecContext(r.Context(), `UPDATE orders SET status='COMPLETED',payment_status='PAID',paid_cents=total_cents,
			inventory_reserved=0,stock_reserved_at=NULL,paid_at=NOW(3),completed_at=NOW(3) WHERE id=? AND tenant_id=?`, orderID, actor.TenantID); err != nil {
			handleSQLError(w, err)
			return
		}
		if err = useOrderCoupon(r.Context(), tx, actor.TenantID, orderID); err != nil {
			handleSQLError(w, err)
			return
		}
		if err = enqueuePrintOutboxWith(r.Context(), tx, actor.TenantID, storeID, orderID, "PAYMENT_SUCCESS", paymentPrintDedupeKey(paymentID), actor.UserID, "拆分收款完成"); err != nil {
			handleSQLError(w, err)
			return
		}
	} else if _, err = tx.ExecContext(r.Context(), "UPDATE orders SET paid_cents=paid_cents+?,updated_at=NOW(3) WHERE id=? AND tenant_id=?",
		input.AmountCents, orderID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "order.split_settle", "order", int64String(orderID), input, r)
	s.getOrderByID(w, r, actor.TenantID, orderID)
}

func (s *Server) createOrderReturnRequest(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input returnRequestInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.OrderItemID <= 0 || input.Quantity <= 0 || input.Reason == "" || len([]rune(input.Reason)) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "order_item_id, positive quantity and reason up to 255 characters are required")
		return
	}
	actor := currentIdentity(r.Context())
	conn, release, err := s.acquirePaymentOrderLock(r.Context(), actor.TenantID, orderID)
	if err != nil {
		writeError(w, http.StatusConflict, "ORDER_OPERATION_IN_PROGRESS", err.Error())
		return
	}
	defer release()
	tx, err := conn.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var storeID, paidCents int64
	var orderType, settlementMode, paymentStatus, status string
	if err = tx.QueryRowContext(r.Context(), `SELECT store_id,paid_cents,order_type,settlement_mode_snapshot,payment_status,status
		FROM orders WHERE id=? AND tenant_id=? FOR UPDATE`, orderID, actor.TenantID).
		Scan(&storeID, &paidCents, &orderType, &settlementMode, &paymentStatus, &status); err != nil {
		handleSQLError(w, err)
		return
	}
	if !validDineInItemChangeOrder(orderType, settlementMode, paymentStatus, status, paidCents) {
		writeError(w, http.StatusConflict, "ORDER_NOT_RETURNABLE", "仅未开始收款的堂食订单可以退菜")
		return
	}
	var activePayment int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_transactions
		WHERE tenant_id=? AND order_id=? AND status IN ('CREATING','PENDING','SUCCESS')`,
		actor.TenantID, orderID).Scan(&activePayment); err != nil {
		handleSQLError(w, err)
		return
	}
	if activePayment > 0 {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", "订单已开始支付，请确认支付结果后再退菜")
		return
	}
	var skuID, unitPrice int64
	var itemQuantity int
	var productName string
	if err = tx.QueryRowContext(r.Context(), `SELECT sku_id,product_name,unit_price_cents,quantity FROM order_items
		WHERE id=? AND order_id=? AND tenant_id=? FOR UPDATE`, input.OrderItemID, orderID, actor.TenantID).
		Scan(&skuID, &productName, &unitPrice, &itemQuantity); err != nil {
		handleSQLError(w, err)
		return
	}
	if input.Quantity > itemQuantity {
		writeError(w, http.StatusConflict, "RETURN_QUANTITY_EXCEEDED", "退菜数量超过该商品当前可退数量")
		return
	}
	var orderQuantity int
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(quantity),0) FROM order_items
		WHERE tenant_id=? AND order_id=?`, actor.TenantID, orderID).Scan(&orderQuantity); err != nil {
		handleSQLError(w, err)
		return
	}
	if input.Quantity >= orderQuantity {
		writeError(w, http.StatusConflict, "RETURN_ALL_ITEMS_NOT_ALLOWED", "不能通过退菜清空整单；如需取消整单，请关闭订单")
		return
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO order_return_requests(
		tenant_id,store_id,order_id,order_item_id,sku_id,product_name_snapshot,quantity,amount_cents,reason,status,
		requested_by,reviewed_by,reviewed_at,review_remark
	) VALUES(?,?,?,?,?,?,?,?,?,'APPROVED',?,?,NOW(3),'退菜直接生效')`,
		actor.TenantID, storeID, orderID, input.OrderItemID, skuID, productName,
		input.Quantity, unitPrice*int64(input.Quantity), input.Reason, actor.UserID, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	requestID, _ := result.LastInsertId()
	if _, err = tx.ExecContext(r.Context(), `UPDATE order_items SET quantity=quantity-?,subtotal_cents=unit_price_cents*(quantity-?)
		WHERE id=? AND tenant_id=?`, input.Quantity, input.Quantity, input.OrderItemID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE inventory SET stock=stock+? WHERE tenant_id=? AND sku_id=?",
		input.Quantity, actor.TenantID, skuID); err != nil {
		handleSQLError(w, err)
		return
	}
	var merchandise, memberTotal, storeDiscount, couponDiscount int64
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(subtotal_cents),0),COALESCE(SUM(member_discount_cents*quantity),0)
		FROM order_items WHERE tenant_id=? AND order_id=?`, actor.TenantID, orderID).Scan(&merchandise, &memberTotal); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT store_promotion_discount_cents,coupon_discount_cents FROM orders
		WHERE id=? AND tenant_id=?`, orderID, actor.TenantID).Scan(&storeDiscount, &couponDiscount); err != nil {
		handleSQLError(w, err)
		return
	}
	if storeDiscount > merchandise {
		storeDiscount = merchandise
	}
	if couponDiscount > merchandise-storeDiscount {
		couponDiscount = merchandise - storeDiscount
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE orders SET merchandise_subtotal_cents=?,member_discount_cents=?,
		store_promotion_discount_cents=?,coupon_discount_cents=?,total_cents=?,updated_at=NOW(3)
		WHERE id=? AND tenant_id=?`, merchandise, memberTotal, storeDiscount, couponDiscount,
		merchandise-storeDiscount-couponDiscount, orderID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = enqueuePrintOutboxWith(r.Context(), tx, actor.TenantID, storeID, orderID, "ORDER_CREATED",
		fmt.Sprintf("ORDER:%d:RETURN:%d", orderID, requestID), actor.UserID,
		fmt.Sprintf("【退菜】%s ×%d；原因：%s", productName, input.Quantity, input.Reason)); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "order.return_direct", "order_return_request", int64String(requestID), input, r)
	s.getOrderByID(w, r, actor.TenantID, orderID)
}

func (s *Server) listOrderReturnRequests(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,order_item_id,sku_id,product_name_snapshot,quantity,amount_cents,
		reason,status,requested_by,COALESCE(reviewed_by,0),COALESCE(DATE_FORMAT(reviewed_at,'%Y-%m-%d %H:%i:%s'),''),
		review_remark,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s')
		FROM order_return_requests WHERE tenant_id=? AND order_id=? ORDER BY id DESC`, actor.TenantID, orderID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, itemID, skuID, amount, requestedBy, reviewedBy int64
		var quantity int
		var productName, reason, status, reviewedAt, reviewRemark, createdAt string
		if err = rows.Scan(&id, &itemID, &skuID, &productName, &quantity, &amount, &reason, &status,
			&requestedBy, &reviewedBy, &reviewedAt, &reviewRemark, &createdAt); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "order_item_id": itemID, "sku_id": skuID, "product_name": productName,
			"quantity": quantity, "amount_cents": amount, "reason": reason, "status": status, "requested_by": requestedBy,
			"reviewed_by": reviewedBy, "reviewed_at": reviewedAt, "review_remark": reviewRemark, "created_at": createdAt})
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) reviewOrderReturnRequest(w http.ResponseWriter, r *http.Request) {
	requestID, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	var input reviewReturnInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Action = strings.ToUpper(strings.TrimSpace(input.Action))
	input.Remark = strings.TrimSpace(input.Remark)
	if !validStatus(input.Action, "APPROVE", "REJECT") || len([]rune(input.Remark)) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "action must be APPROVE or REJECT and remark must not exceed 255 characters")
		return
	}
	actor := currentIdentity(r.Context())
	var orderID int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT order_id FROM order_return_requests WHERE id=? AND tenant_id=?`,
		requestID, actor.TenantID).Scan(&orderID); err != nil {
		handleSQLError(w, err)
		return
	}
	conn, release, err := s.acquirePaymentOrderLock(r.Context(), actor.TenantID, orderID)
	if err != nil {
		writeError(w, http.StatusConflict, "ORDER_OPERATION_IN_PROGRESS", err.Error())
		return
	}
	defer release()
	tx, err := conn.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var storeID, itemID, skuID, amount, paidCents int64
	var quantity int
	var productName, requestStatus, orderType, settlementMode, paymentStatus, orderStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT rr.store_id,rr.order_item_id,rr.sku_id,rr.product_name_snapshot,rr.quantity,rr.amount_cents,rr.status,
		o.paid_cents,o.order_type,o.settlement_mode_snapshot,o.payment_status,o.status
		FROM order_return_requests rr JOIN orders o ON o.id=rr.order_id AND o.tenant_id=rr.tenant_id
		WHERE rr.id=? AND rr.tenant_id=? FOR UPDATE`, requestID, actor.TenantID).
		Scan(&storeID, &itemID, &skuID, &productName, &quantity, &amount, &requestStatus, &paidCents,
			&orderType, &settlementMode, &paymentStatus, &orderStatus)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if requestStatus != "PENDING" {
		writeError(w, http.StatusConflict, "RETURN_ALREADY_REVIEWED", "该退菜申请已经处理")
		return
	}
	targetStatus := "REJECTED"
	if input.Action == "APPROVE" {
		if !validDineInItemChangeOrder(orderType, settlementMode, paymentStatus, orderStatus, paidCents) {
			writeError(w, http.StatusConflict, "ORDER_NOT_RETURNABLE", "订单已开始收款或已关闭，不能再批准退菜")
			return
		}
		var activePayment int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_transactions
			WHERE tenant_id=? AND order_id=? AND status IN ('CREATING','PENDING','SUCCESS')`,
			actor.TenantID, orderID).Scan(&activePayment); err != nil {
			handleSQLError(w, err)
			return
		}
		if activePayment > 0 {
			writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", "订单已开始支付，不能再批准退菜")
			return
		}
		var currentQuantity, orderQuantity int
		var unitPrice, memberDiscount int64
		if err = tx.QueryRowContext(r.Context(), `SELECT quantity,unit_price_cents,member_discount_cents FROM order_items
			WHERE id=? AND order_id=? AND tenant_id=? FOR UPDATE`, itemID, orderID, actor.TenantID).
			Scan(&currentQuantity, &unitPrice, &memberDiscount); err != nil {
			handleSQLError(w, err)
			return
		}
		if quantity > currentQuantity {
			writeError(w, http.StatusConflict, "RETURN_QUANTITY_CHANGED", "商品数量已变化，请重新提交退菜申请")
			return
		}
		if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(quantity),0) FROM order_items
			WHERE tenant_id=? AND order_id=?`, actor.TenantID, orderID).Scan(&orderQuantity); err != nil {
			handleSQLError(w, err)
			return
		}
		if quantity >= orderQuantity {
			writeError(w, http.StatusConflict, "RETURN_ALL_ITEMS_NOT_ALLOWED", "不能通过退菜清空整单；如需取消整单，请关闭订单")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE order_items SET quantity=quantity-?,subtotal_cents=unit_price_cents*(quantity-?)
			WHERE id=? AND tenant_id=?`, quantity, quantity, itemID, actor.TenantID); err != nil {
			handleSQLError(w, err)
			return
		}
		if _, err = tx.ExecContext(r.Context(), "UPDATE inventory SET stock=stock+? WHERE tenant_id=? AND sku_id=?",
			quantity, actor.TenantID, skuID); err != nil {
			handleSQLError(w, err)
			return
		}
		var merchandise, memberTotal, storeDiscount, couponDiscount int64
		if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(subtotal_cents),0),COALESCE(SUM(member_discount_cents*quantity),0)
			FROM order_items WHERE tenant_id=? AND order_id=?`, actor.TenantID, orderID).Scan(&merchandise, &memberTotal); err != nil {
			handleSQLError(w, err)
			return
		}
		if err = tx.QueryRowContext(r.Context(), `SELECT store_promotion_discount_cents,coupon_discount_cents FROM orders
			WHERE id=? AND tenant_id=?`, orderID, actor.TenantID).Scan(&storeDiscount, &couponDiscount); err != nil {
			handleSQLError(w, err)
			return
		}
		if storeDiscount > merchandise {
			storeDiscount = merchandise
		}
		if couponDiscount > merchandise-storeDiscount {
			couponDiscount = merchandise - storeDiscount
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE orders SET merchandise_subtotal_cents=?,member_discount_cents=?,
			store_promotion_discount_cents=?,coupon_discount_cents=?,total_cents=?,updated_at=NOW(3)
			WHERE id=? AND tenant_id=?`, merchandise, memberTotal, storeDiscount, couponDiscount,
			merchandise-storeDiscount-couponDiscount, orderID, actor.TenantID); err != nil {
			handleSQLError(w, err)
			return
		}
		targetStatus = "APPROVED"
		if err = enqueuePrintOutboxWith(r.Context(), tx, actor.TenantID, storeID, orderID, "ORDER_CREATED",
			fmt.Sprintf("ORDER:%d:RETURN:%d", orderID, requestID), actor.UserID,
			fmt.Sprintf("【退菜】%s ×%d", productName, quantity)); err != nil {
			handleSQLError(w, err)
			return
		}
		_ = amount
		_ = unitPrice
		_ = memberDiscount
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE order_return_requests SET status=?,reviewed_by=?,reviewed_at=NOW(3),
		review_remark=? WHERE id=? AND tenant_id=? AND status='PENDING'`, targetStatus, actor.UserID, input.Remark,
		requestID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "order.return_review", "order_return_request", int64String(requestID), input, r)
	writeData(w, http.StatusOK, map[string]any{"id": requestID, "order_id": orderID, "status": targetStatus})
}

func (s *Server) getOfflineReconciliation(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	date, ok := normalizeBusinessDate(w, r.URL.Query().Get("business_date"))
	if !ok {
		return
	}
	report, err := s.loadOfflineReconciliation(r.Context(), actor.TenantID, storeID, date)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, report)
}

func (s *Server) confirmOfflineReconciliation(w http.ResponseWriter, r *http.Request) {
	var input offlineReconciliationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	date, ok := normalizeBusinessDate(w, input.BusinessDate)
	if !ok {
		return
	}
	if input.ActualCashCents < 0 || input.ActualExternalCents < 0 || input.ActualCashCents > maxBusinessAmountCents ||
		input.ActualExternalCents > maxBusinessAmountCents || len([]rune(input.Note)) > 500 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "actual amounts must be non-negative and note must not exceed 500 characters")
		return
	}
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	report, err := s.loadOfflineReconciliation(r.Context(), actor.TenantID, storeID, date)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	expectedCash := report["expected_cash_cents"].(int64)
	expectedExternal := report["expected_external_cents"].(int64)
	discrepancy := input.ActualCashCents + input.ActualExternalCents - expectedCash - expectedExternal
	_, err = s.DB.ExecContext(r.Context(), `INSERT INTO offline_reconciliations(
		tenant_id,store_id,business_date,expected_cash_cents,expected_external_cents,actual_cash_cents,actual_external_cents,
		discrepancy_cents,note,status,confirmed_by,confirmed_at
	) VALUES(?,?,?,?,?,?,?,?,?,'CONFIRMED',?,NOW(3))
	ON DUPLICATE KEY UPDATE expected_cash_cents=VALUES(expected_cash_cents),expected_external_cents=VALUES(expected_external_cents),
		actual_cash_cents=VALUES(actual_cash_cents),actual_external_cents=VALUES(actual_external_cents),
		discrepancy_cents=VALUES(discrepancy_cents),note=VALUES(note),status='CONFIRMED',
		confirmed_by=VALUES(confirmed_by),confirmed_at=NOW(3)`,
		actor.TenantID, storeID, date, expectedCash, expectedExternal, input.ActualCashCents, input.ActualExternalCents,
		discrepancy, input.Note, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "payment.offline_reconciliation", "store", int64String(storeID), input, r)
	report, err = s.loadOfflineReconciliation(r.Context(), actor.TenantID, storeID, date)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, report)
}

func normalizeBusinessDate(w http.ResponseWriter, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now().In(beijingLocation).Format("2006-01-02"), true
	}
	if _, err := time.ParseInLocation("2006-01-02", raw, beijingLocation); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "business_date must use YYYY-MM-DD")
		return "", false
	}
	return raw, true
}

func (s *Server) loadOfflineReconciliation(ctx context.Context, tenantID, storeID int64, date string) (map[string]any, error) {
	expected := map[string]int64{"offline_cash": 0, "external": 0}
	counts := map[string]int{"offline_cash": 0, "external": 0}
	rows, err := s.DB.QueryContext(ctx, `SELECT provider,COUNT(*),COALESCE(SUM(amount_cents),0)
		FROM payment_transactions WHERE tenant_id=? AND store_id=? AND provider IN ('offline_cash','external')
		  AND status IN ('SUCCESS','REFUNDED') AND DATE(paid_at)=? GROUP BY provider`, tenantID, storeID, date)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var provider string
		var count int
		var amount int64
		if err = rows.Scan(&provider, &count, &amount); err != nil {
			rows.Close()
			return nil, err
		}
		expected[provider], counts[provider] = amount, count
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	report := map[string]any{
		"business_date": date, "expected_cash_cents": expected["offline_cash"],
		"expected_external_cents": expected["external"], "cash_count": counts["offline_cash"],
		"external_count": counts["external"], "status": "UNCONFIRMED",
	}
	var id, actualCash, actualExternal, discrepancy, confirmedBy int64
	var note, status, confirmedAt string
	err = s.DB.QueryRowContext(ctx, `SELECT id,actual_cash_cents,actual_external_cents,discrepancy_cents,note,status,
		confirmed_by,DATE_FORMAT(confirmed_at,'%Y-%m-%d %H:%i:%s') FROM offline_reconciliations
		WHERE tenant_id=? AND store_id=? AND business_date=?`, tenantID, storeID, date).
		Scan(&id, &actualCash, &actualExternal, &discrepancy, &note, &status, &confirmedBy, &confirmedAt)
	if err == nil {
		report["id"], report["actual_cash_cents"], report["actual_external_cents"] = id, actualCash, actualExternal
		report["discrepancy_cents"], report["note"], report["status"] = discrepancy, note, status
		report["confirmed_by"], report["confirmed_at"] = confirmedBy, confirmedAt
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return report, nil
}

func (s *Server) acquireDineInOrderLocks(ctx context.Context, tenantID int64, orderIDs ...int64) (*sql.Conn, func(), error) {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	ids := append([]int64(nil), orderIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		key := fmt.Sprintf("tanban:payment:%d:%d", tenantID, id)
		var acquired int
		if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,5)", key).Scan(&acquired); err != nil || acquired != 1 {
			for i := len(keys) - 1; i >= 0; i-- {
				var released sql.NullInt64
				_ = conn.QueryRowContext(context.Background(), "SELECT RELEASE_LOCK(?)", keys[i]).Scan(&released)
			}
			conn.Close()
			if err == nil {
				err = errors.New("another order operation is in progress; retry shortly")
			}
			return nil, nil, err
		}
		keys = append(keys, key)
	}
	release := func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		for i := len(keys) - 1; i >= 0; i-- {
			var released sql.NullInt64
			_ = conn.QueryRowContext(releaseCtx, "SELECT RELEASE_LOCK(?)", keys[i]).Scan(&released)
		}
		_ = conn.Close()
	}
	return conn, release, nil
}
