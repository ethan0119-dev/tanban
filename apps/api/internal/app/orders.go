package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
	"github.com/go-chi/chi/v5"
)

type orderDTO struct {
	ID             int64                  `json:"id"`
	TenantID       int64                  `json:"tenant_id"`
	StoreID        int64                  `json:"store_id"`
	StoreName      string                 `json:"store_name"`
	OrderNo        string                 `json:"order_no"`
	CustomerName   string                 `json:"customer_name"`
	CustomerPhone  string                 `json:"customer_phone"`
	Remark         string                 `json:"remark"`
	Source         string                 `json:"source"`
	Fulfillment    string                 `json:"fulfillment_type"`
	OrderType      string                 `json:"order_type"`
	SettlementMode string                 `json:"settlement_mode"`
	AdditionCount  int                    `json:"addition_count"`
	BusinessDate   string                 `json:"business_date,omitempty"`
	PickupSequence int64                  `json:"pickup_sequence,omitempty"`
	PickupCode     string                 `json:"pickup_code,omitempty"`
	FastFoodPlate  *orderFastFoodPlateDTO `json:"fast_food_plate,omitempty"`
	Table          *orderTableDTO         `json:"table,omitempty"`
	Status         string                 `json:"status"`
	PaymentStatus  string                 `json:"payment_status"`
	TotalCents     int64                  `json:"total_cents"`
	PaidCents      int64                  `json:"paid_cents"`
	RefundedCents  int64                  `json:"refunded_cents"`
	PaidAt         *string                `json:"paid_at,omitempty"`
	CreatedAt      string                 `json:"created_at"`
	Items          []orderItemDTO         `json:"items,omitempty"`
	Payment        any                    `json:"payment,omitempty"`
	AvailableSteps []string               `json:"available_transitions"`
}

type orderFastFoodPlateDTO struct {
	ID        int64  `json:"id"`
	PublicID  string `json:"publicId"`
	Name      string `json:"plateName"`
	PlateCode string `json:"plateCode"`
}

type orderTableDTO struct {
	ID        int64  `json:"id"`
	PublicID  string `json:"publicId"`
	AreaName  string `json:"areaName"`
	Name      string `json:"name"`
	TableCode string `json:"tableCode"`
}

type orderItemDTO struct {
	ID             int64          `json:"id"`
	ProductID      int64          `json:"product_id"`
	SKUID          int64          `json:"sku_id"`
	ProductName    string         `json:"product_name"`
	SKUName        string         `json:"sku_name"`
	Attributes     map[string]any `json:"attributes"`
	Configuration  map[string]any `json:"configuration"`
	ItemRemark     string         `json:"item_remark"`
	BasePriceCents int64          `json:"base_price_cents"`
	ModifierCents  int64          `json:"modifier_price_cents"`
	UnitPriceCents int64          `json:"unit_price_cents"`
	Quantity       int            `json:"quantity"`
	SubtotalCents  int64          `json:"subtotal_cents"`
}

func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	status := strings.ToUpper(r.URL.Query().Get("status"))
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	where := " WHERE tenant_id=? AND store_id=?"
	args := []any{identity.TenantID, storeID}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	orderType, err := normalizeOrderTypeFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if orderType != "" {
		where += " AND order_type=?"
		args = append(args, orderType)
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 100 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "keyword must not exceed 100 characters")
		return
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (order_no LIKE ? OR customer_name LIKE ? OR customer_phone LIKE ? OR table_area_name_snapshot LIKE ? OR table_name_snapshot LIKE ? OR table_code_snapshot LIKE ? OR pickup_code LIKE ? OR fast_food_plate_name_snapshot LIKE ? OR fast_food_plate_code_snapshot LIKE ?)"
		args = append(args, like, like, like, like, like, like, like, like, like)
	}
	for _, filter := range []struct {
		camel, snake, operator string
	}{{"startAt", "start_at", ">="}, {"endAt", "end_at", "<="}} {
		raw := strings.TrimSpace(r.URL.Query().Get(filter.camel))
		if raw == "" {
			raw = strings.TrimSpace(r.URL.Query().Get(filter.snake))
		}
		if raw == "" {
			continue
		}
		parsed, parseErr := parseBeijingDateTime(raw)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", filter.camel+" must use Beijing time")
			return
		}
		where += " AND created_at " + filter.operator + " ?"
		args = append(args, formatBeijingDateTime(parsed))
	}
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM orders"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	args = append(args, size, offset)
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,tenant_id,store_id,(SELECT name FROM stores WHERE stores.id=orders.store_id),order_no,customer_name,customer_phone,remark,source,fulfillment_type,order_type,settlement_mode_snapshot,addition_count,IF(business_date IS NULL,'',DATE_FORMAT(business_date,'%Y-%m-%d')),pickup_sequence,pickup_code,fast_food_plate_id,fast_food_plate_public_id_snapshot,fast_food_plate_name_snapshot,fast_food_plate_code_snapshot,table_id,table_public_id_snapshot,table_area_name_snapshot,table_name_snapshot,table_code_snapshot,status,payment_status,total_cents,paid_cents,refunded_cents,
		IF(paid_at IS NULL,NULL,DATE_FORMAT(paid_at,'%Y-%m-%d %H:%i:%s')),DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM orders`+where+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []orderDTO{}
	for rows.Next() {
		var item orderDTO
		var paidAt sql.NullString
		var tableID sql.NullInt64
		var pickupSequence, fastFoodPlateID sql.NullInt64
		var fastFoodPublicID, fastFoodName, fastFoodCode string
		var tablePublicID, tableArea, tableName, tableCode string
		if err := rows.Scan(&item.ID, &item.TenantID, &item.StoreID, &item.StoreName, &item.OrderNo, &item.CustomerName, &item.CustomerPhone, &item.Remark, &item.Source, &item.Fulfillment, &item.OrderType, &item.SettlementMode, &item.AdditionCount, &item.BusinessDate, &pickupSequence, &item.PickupCode, &fastFoodPlateID, &fastFoodPublicID, &fastFoodName, &fastFoodCode, &tableID, &tablePublicID, &tableArea, &tableName, &tableCode, &item.Status, &item.PaymentStatus, &item.TotalCents, &item.PaidCents, &item.RefundedCents, &paidAt, &item.CreatedAt); err != nil {
			handleSQLError(w, err)
			return
		}
		setOrderTable(&item, tableID, tablePublicID, tableArea, tableName, tableCode)
		setOrderPickupContext(&item, pickupSequence, fastFoodPlateID, fastFoodPublicID, fastFoodName, fastFoodCode)
		if paidAt.Valid {
			item.PaidAt = &paidAt.String
		}
		item.AvailableSteps = orderTransitions[item.Status]
		items = append(items, item)
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "orderID")
	if ok {
		identity := currentIdentity(r.Context())
		s.getOrderByID(w, r, identity.TenantID, id)
	}
}

func (s *Server) getOrderByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	item, err := s.loadOrder(r.Context(), tenantID, id, "")
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) loadOrder(ctx context.Context, tenantID, id int64, orderNo string) (orderDTO, error) {
	return s.loadOrderWith(ctx, s.DB, tenantID, id, orderNo)
}

type sqlQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Server) loadOrderWith(ctx context.Context, queryer sqlQueryer, tenantID, id int64, orderNo string) (orderDTO, error) {
	query := `SELECT id,tenant_id,store_id,(SELECT name FROM stores WHERE stores.id=orders.store_id),order_no,customer_name,customer_phone,remark,source,fulfillment_type,order_type,settlement_mode_snapshot,addition_count,IF(business_date IS NULL,'',DATE_FORMAT(business_date,'%Y-%m-%d')),pickup_sequence,pickup_code,fast_food_plate_id,fast_food_plate_public_id_snapshot,fast_food_plate_name_snapshot,fast_food_plate_code_snapshot,table_id,table_public_id_snapshot,table_area_name_snapshot,table_name_snapshot,table_code_snapshot,status,payment_status,total_cents,paid_cents,refunded_cents,
		IF(paid_at IS NULL,NULL,DATE_FORMAT(paid_at,'%Y-%m-%d %H:%i:%s')),DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM orders WHERE tenant_id=?`
	args := []any{tenantID}
	if id > 0 {
		query += " AND id=?"
		args = append(args, id)
	} else {
		query += " AND order_no=?"
		args = append(args, orderNo)
	}
	var item orderDTO
	var paidAt sql.NullString
	var tableID sql.NullInt64
	var pickupSequence, fastFoodPlateID sql.NullInt64
	var fastFoodPublicID, fastFoodName, fastFoodCode string
	var tablePublicID, tableArea, tableName, tableCode string
	err := queryer.QueryRowContext(ctx, query, args...).Scan(&item.ID, &item.TenantID, &item.StoreID, &item.StoreName, &item.OrderNo, &item.CustomerName, &item.CustomerPhone, &item.Remark, &item.Source, &item.Fulfillment, &item.OrderType, &item.SettlementMode, &item.AdditionCount, &item.BusinessDate, &pickupSequence, &item.PickupCode, &fastFoodPlateID, &fastFoodPublicID, &fastFoodName, &fastFoodCode, &tableID, &tablePublicID, &tableArea, &tableName, &tableCode, &item.Status, &item.PaymentStatus, &item.TotalCents, &item.PaidCents, &item.RefundedCents, &paidAt, &item.CreatedAt)
	if err != nil {
		return item, err
	}
	if paidAt.Valid {
		item.PaidAt = &paidAt.String
	}
	setOrderTable(&item, tableID, tablePublicID, tableArea, tableName, tableCode)
	setOrderPickupContext(&item, pickupSequence, fastFoodPlateID, fastFoodPublicID, fastFoodName, fastFoodCode)
	item.AvailableSteps = orderTransitions[item.Status]
	rows, err := queryer.QueryContext(ctx, `SELECT id,product_id,sku_id,product_name,sku_name,attributes_json,COALESCE(configuration_json,'{}'),item_remark,base_price_cents,modifier_price_cents,unit_price_cents,quantity,subtotal_cents FROM order_items WHERE tenant_id=? AND order_id=? ORDER BY id`, tenantID, item.ID)
	if err != nil {
		return item, err
	}
	item.Items = []orderItemDTO{}
	for rows.Next() {
		var row orderItemDTO
		var attrs, configuration string
		if err := rows.Scan(&row.ID, &row.ProductID, &row.SKUID, &row.ProductName, &row.SKUName, &attrs, &configuration, &row.ItemRemark, &row.BasePriceCents, &row.ModifierCents, &row.UnitPriceCents, &row.Quantity, &row.SubtotalCents); err != nil {
			return item, err
		}
		_ = json.Unmarshal([]byte(attrs), &row.Attributes)
		_ = json.Unmarshal([]byte(configuration), &row.Configuration)
		if row.Configuration == nil {
			row.Configuration = map[string]any{}
		}
		item.Items = append(item.Items, row)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return item, err
	}
	rows.Close()
	var providerName, providerNo, paymentStatus string
	var paymentID, amount int64
	err = queryer.QueryRowContext(ctx, "SELECT id,provider,provider_order_no,amount_cents,status FROM payment_transactions WHERE tenant_id=? AND order_id=? ORDER BY id DESC LIMIT 1", tenantID, item.ID).Scan(&paymentID, &providerName, &providerNo, &amount, &paymentStatus)
	if err == nil {
		item.Payment = map[string]any{"id": paymentID, "provider": providerName, "provider_order_no": providerNo, "amount_cents": amount, "status": paymentStatus}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return item, err
	}
	return item, nil
}

func setOrderTable(order *orderDTO, tableID sql.NullInt64, publicID, areaName, name, tableCode string) {
	if !tableID.Valid {
		return
	}
	order.Table = &orderTableDTO{ID: tableID.Int64, PublicID: publicID, AreaName: areaName, Name: name, TableCode: tableCode}
}

func setOrderPickupContext(order *orderDTO, pickupSequence, plateID sql.NullInt64, publicID, name, plateCode string) {
	if pickupSequence.Valid {
		order.PickupSequence = pickupSequence.Int64
	}
	if plateID.Valid {
		order.FastFoodPlate = &orderFastFoodPlateDTO{ID: plateID.Int64, PublicID: publicID, Name: name, PlateCode: plateCode}
	}
}

var orderTransitions = map[string][]string{
	"PENDING_PAYMENT": {"CLOSED"},
	"PAID":            {"ACCEPTED", "PREPARING", "CLOSED"},
	"ACCEPTED":        {"PREPARING", "READY", "CLOSED"},
	"PREPARING":       {"READY", "COMPLETED"},
	"READY":           {"COMPLETED"},
}

var (
	errInsufficientStock  = errors.New("insufficient stock")
	errPaymentAlreadyPaid = errors.New("payment already succeeded")
)

type sqlExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func reserveStock(ctx context.Context, executor sqlExecer, tenantID, skuID int64, quantity int) error {
	result, err := executor.ExecContext(ctx, "UPDATE inventory SET stock=stock-? WHERE sku_id=? AND tenant_id=? AND stock>=?", quantity, skuID, tenantID, quantity)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errInsufficientStock
	}
	return nil
}

func releaseOrderCoupon(ctx context.Context, executor sqlExecer, tenantID, orderID int64) error {
	_, err := executor.ExecContext(ctx, `UPDATE customer_coupons SET status='PROVISIONAL',order_id=NULL
		WHERE tenant_id=? AND order_id=? AND status='RESERVED'`, tenantID, orderID)
	return err
}

func useOrderCoupon(ctx context.Context, executor sqlExecer, tenantID, orderID int64) error {
	_, err := executor.ExecContext(ctx, `UPDATE customer_coupons SET status='USED',used_at=NOW(3)
		WHERE tenant_id=? AND order_id=? AND status='RESERVED'`, tenantID, orderID)
	return err
}

type transitionInput struct {
	Status string `json:"status"`
}

type settlePayAfterInput struct {
	Method string `json:"method"`
	Remark string `json:"remark"`
}

func (s *Server) settlePayAfterOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input settlePayAfterInput
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
	identity := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var storeID, totalCents int64
	var status, paymentStatus, settlementMode string
	if err = tx.QueryRowContext(r.Context(), `SELECT store_id,total_cents,status,payment_status,settlement_mode_snapshot,inventory_reserved
		FROM orders WHERE id=? AND tenant_id=? FOR UPDATE`, id, identity.TenantID).
		Scan(&storeID, &totalCents, &status, &paymentStatus, &settlementMode, new(int)); err != nil {
		handleSQLError(w, err)
		return
	}
	if paymentStatus == "PAID" {
		if err = tx.Commit(); err != nil {
			handleSQLError(w, err)
			return
		}
		s.getOrderByID(w, r, identity.TenantID, id)
		return
	}
	if settlementMode != "PAY_AFTER" || !validStatus(status, "PAID", "ACCEPTED", "PREPARING", "READY") || paymentStatus != "UNPAID" {
		writeError(w, http.StatusConflict, "ORDER_NOT_SETTLEABLE", "当前订单不是可结账的后付账堂食订单")
		return
	}
	var pendingPayments int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_transactions
		WHERE tenant_id=? AND order_id=? AND status IN ('CREATING','PENDING')`, identity.TenantID, id).Scan(&pendingPayments); err != nil {
		handleSQLError(w, err)
		return
	}
	if pendingPayments > 0 {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", "顾客正在支付，请确认支付结果后再操作线下结账")
		return
	}
	reference := newBusinessNo("OF")
	providerName := "external"
	if input.Method == "CASH" {
		providerName = "offline_cash"
	}
	raw, _ := json.Marshal(map[string]any{"method": input.Method, "remark": input.Remark, "confirmedBy": identity.UserID})
	paymentResult, err := tx.ExecContext(r.Context(), `INSERT INTO payment_transactions(
		tenant_id,store_id,order_id,provider,provider_request_no,provider_order_no,amount_cents,status,raw_response,paid_at
	) VALUES(?,?,?,?,?,?,?,'SUCCESS',?,NOW(3))`, identity.TenantID, storeID, id, providerName, reference, reference, totalCents, string(raw))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	paymentID, _ := paymentResult.LastInsertId()
	if _, err = tx.ExecContext(r.Context(), `UPDATE orders SET status='COMPLETED',payment_status='PAID',paid_cents=total_cents,
		inventory_reserved=0,stock_reserved_at=NULL,paid_at=NOW(3),completed_at=NOW(3)
		WHERE id=? AND tenant_id=? AND payment_status='UNPAID'`, id, identity.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = useOrderCoupon(r.Context(), tx, identity.TenantID, id); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = enqueuePrintOutboxWith(r.Context(), tx, identity.TenantID, storeID, id, "PAYMENT_SUCCESS", paymentPrintDedupeKey(paymentID), identity.UserID, "线下结账："+input.Method); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "order.offline_settle", "order", int64String(id), map[string]any{"method": input.Method, "remark": input.Remark}, r)
	s.getOrderByID(w, r, identity.TenantID, id)
}

func (s *Server) transitionOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input transitionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Status = strings.ToUpper(input.Status)
	identity := currentIdentity(r.Context())
	var paymentConn *sql.Conn
	var releasePaymentLock func()
	var err error
	if input.Status == "CLOSED" {
		paymentConn, releasePaymentLock, err = s.acquirePaymentOrderLock(r.Context(), identity.TenantID, id)
		if err != nil {
			writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", err.Error())
			return
		}
		defer releasePaymentLock()
		if err = s.closePendingPaymentLocked(r.Context(), paymentConn, identity.TenantID, id); err != nil {
			if errors.Is(err, errPaymentAlreadyPaid) {
				writeError(w, http.StatusConflict, "ORDER_ALREADY_PAID", "a successful payment exists; the order cannot be closed")
			} else {
				writeError(w, http.StatusBadGateway, "PAYMENT_CLOSE_FAILED", err.Error())
			}
			return
		}
	}
	var tx *sql.Tx
	if paymentConn != nil {
		tx, err = paymentConn.BeginTx(r.Context(), nil)
	} else {
		tx, err = s.DB.BeginTx(r.Context(), nil)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var current, paymentStatus string
	var inventoryReserved int
	if err := tx.QueryRowContext(r.Context(), "SELECT status,payment_status,inventory_reserved FROM orders WHERE id=? AND tenant_id=? FOR UPDATE", id, identity.TenantID).Scan(&current, &paymentStatus, &inventoryReserved); err != nil {
		handleSQLError(w, err)
		return
	}
	allowed := false
	for _, target := range orderTransitions[current] {
		if target == input.Status {
			allowed = true
		}
	}
	if !allowed || (input.Status == "CLOSED" && paymentStatus != "UNPAID") {
		writeError(w, http.StatusConflict, "INVALID_TRANSITION", "order cannot transition from "+current+" to "+input.Status)
		return
	}
	completedExpr, closedExpr := "completed_at", "closed_at"
	if input.Status == "COMPLETED" {
		completedExpr = "NOW(3)"
	}
	if input.Status == "CLOSED" {
		var pendingPayments int
		if countErr := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM payment_transactions WHERE order_id=? AND status IN ('CREATING','PENDING')", id).Scan(&pendingPayments); countErr != nil {
			handleSQLError(w, countErr)
			return
		}
		if pendingPayments > 0 {
			writeError(w, http.StatusConflict, "PAYMENT_CLOSE_REQUIRED", "provider payment must be closed before closing this order")
			return
		}
		closedExpr = "NOW(3)"
		if inventoryReserved == 1 {
			if restoreErr := restoreOrderInventory(r.Context(), tx, identity.TenantID, id); restoreErr != nil {
				handleSQLError(w, restoreErr)
				return
			}
		}
		if couponErr := releaseOrderCoupon(r.Context(), tx, identity.TenantID, id); couponErr != nil {
			handleSQLError(w, couponErr)
			return
		}
	}
	reservationUpdate := ""
	if input.Status == "CLOSED" {
		reservationUpdate = ",inventory_reserved=0,stock_reserved_at=NULL"
	}
	query := fmt.Sprintf("UPDATE orders SET status=?,completed_at=%s,closed_at=%s%s WHERE id=? AND tenant_id=? AND status=?", completedExpr, closedExpr, reservationUpdate)
	result, err := tx.ExecContext(r.Context(), query, input.Status, id, identity.TenantID, current)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusConflict, "CONCURRENT_UPDATE", "order status changed; refresh and retry")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "order.transition", "order", int64String(id), map[string]string{"from": current, "to": input.Status}, r)
	s.getOrderByID(w, r, identity.TenantID, id)
}

type paymentInput struct {
	OpenID   string `json:"openid"`
	SubAppID string `json:"sub_appid"`
}

const paymentStatusCreating = "CREATING"

type paymentCreationIntent struct {
	ID, TenantID, StoreID, OrderID, Amount int64
	BusinessOrderNo, ProviderRequestNo     string
	MerchantNo, SubAppID, OpenID           string
}

func (s *Server) acquirePaymentOrderLock(ctx context.Context, tenantID, orderID int64) (*sql.Conn, func(), error) {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	lockKey := fmt.Sprintf("tanban:payment:%d:%d", tenantID, orderID)
	var acquired int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,5)", lockKey).Scan(&acquired); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if acquired != 1 {
		conn.Close()
		return nil, nil, errors.New("another payment operation is in progress; retry shortly")
	}
	release := func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var released sql.NullInt64
		_ = conn.QueryRowContext(releaseCtx, "SELECT RELEASE_LOCK(?)", lockKey).Scan(&released)
		_ = conn.Close()
	}
	return conn, release, nil
}

func localPaymentReference(providerRequestNo string) string {
	return "LOCAL-" + providerRequestNo
}

func (s *Server) paymentAcceptanceEnabled(ctx context.Context) (bool, error) {
	var body string
	err := s.DB.QueryRowContext(ctx, "SELECT value_text FROM platform_settings WHERE setting_key='payment'").Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	settings := paymentSettings{Enabled: true}
	if err = json.Unmarshal([]byte(body), &settings); err != nil {
		return false, err
	}
	return settings.Enabled, nil
}

func (s *Server) loadPaymentCreationIntent(ctx context.Context, queryer sqlQueryer, paymentID int64) (paymentCreationIntent, error) {
	var intent paymentCreationIntent
	err := queryer.QueryRowContext(ctx, `SELECT p.id,p.tenant_id,p.store_id,p.order_id,p.amount_cents,o.order_no,p.provider_request_no,
		p.merchant_no,p.sub_appid,p.customer_openid
		FROM payment_transactions p
		JOIN orders o ON o.id=p.order_id AND o.tenant_id=p.tenant_id
		WHERE p.id=?`, paymentID).
		Scan(&intent.ID, &intent.TenantID, &intent.StoreID, &intent.OrderID, &intent.Amount, &intent.BusinessOrderNo, &intent.ProviderRequestNo, &intent.MerchantNo, &intent.SubAppID, &intent.OpenID)
	return intent, err
}

// submitPaymentIntent is only called while holding the per-order MySQL named
// lock. The local CREATING record exists before the provider call, so a crash
// cannot leave a completely orphaned provider order. Provider adapters must
// treat OrderNo as an idempotency key; the reconciler safely resubmits the same
// intent after an ambiguous timeout or process restart.
func (s *Server) submitPaymentIntent(ctx context.Context, conn *sql.Conn, intent paymentCreationIntent) (provider.CreatePaymentResult, error) {
	providerCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	result, err := s.Payment.Create(providerCtx, provider.CreatePaymentRequest{
		MerchantNo: intent.MerchantNo,
		OrderNo:    intent.ProviderRequestNo,
		Amount:     intent.Amount,
		OpenID:     intent.OpenID,
		SubAppID:   intent.SubAppID,
		NotifyURL:  s.paymentNotifyURL(),
	})
	if err == nil && strings.TrimSpace(result.ProviderOrderNo) == "" {
		err = errors.New("provider returned an empty payment number")
	}
	if err != nil {
		raw, _ := json.Marshal(map[string]string{"phase": "creating", "last_error": truncateError(err)})
		if errors.Is(err, provider.ErrNotConfigured) {
			_, _ = conn.ExecContext(ctx, "UPDATE payment_transactions SET status='FAILED',raw_response=?,updated_at=NOW(3) WHERE id=? AND tenant_id=? AND status='CREATING'", string(raw), intent.ID, intent.TenantID)
		} else {
			_, _ = conn.ExecContext(ctx, "UPDATE payment_transactions SET raw_response=?,updated_at=NOW(3) WHERE id=? AND tenant_id=? AND status='CREATING'", string(raw), intent.ID, intent.TenantID)
		}
		return result, err
	}
	rawResponse, _ := json.Marshal(result.PayParams)
	update, err := conn.ExecContext(ctx, `UPDATE payment_transactions SET provider_order_no=?,status=?,raw_response=?,paid_at=NULL
		WHERE id=? AND tenant_id=? AND status='CREATING'`, result.ProviderOrderNo, string(result.Status), string(rawResponse), intent.ID, intent.TenantID)
	if err != nil {
		return result, err
	}
	if changed, _ := update.RowsAffected(); changed != 1 {
		return result, errors.New("payment intent changed while provider creation was in progress")
	}
	if result.Status == provider.PaymentSuccess {
		if err = s.markPaymentPaidLocked(ctx, conn, s.Payment.Name(), result.ProviderOrderNo, time.Now()); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *Server) paymentNotifyURL() string {
	if s.Payment.Name() == "wechat_partner" {
		return s.Config.WeChatPayPartner.NotifyURL
	}
	return s.Config.TianQue.NotifyURL
}

func (s *Server) createPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var input paymentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	s.createPaymentForOrder(w, r, identity.TenantID, id, input)
}

func (s *Server) createPaymentForOrder(w http.ResponseWriter, r *http.Request, tenantID, orderID int64, input paymentInput) {
	enabled, err := s.paymentAcceptanceEnabled(r.Context())
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if !enabled {
		writeError(w, http.StatusServiceUnavailable, "PAYMENTS_DISABLED", "payment acceptance is disabled by the platform")
		return
	}
	conn, release, err := s.acquirePaymentOrderLock(r.Context(), tenantID, orderID)
	if err != nil {
		writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", err.Error())
		return
	}
	defer release()
	expired, allowLate, err := s.expireOrderReservationLocked(r.Context(), conn, tenantID, orderID)
	if errors.Is(err, errPaymentAlreadyPaid) {
		writeError(w, http.StatusConflict, "ORDER_ALREADY_PAID", "a successful payment already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "PAYMENT_CLOSE_FAILED", err.Error())
		return
	}
	if expired && !allowLate {
		writeError(w, http.StatusConflict, "ORDER_PAYMENT_EXPIRED", "the payment window expired and the order was closed")
		return
	}

	var orderNo, orderStatus, orderPaymentStatus, settlementMode, merchantNo, subAppID, storedOpenID, tenantPaymentProvider, onboardingStatus, productAuthorizationStatus string
	var storeID, amount int64
	err = conn.QueryRowContext(r.Context(), `SELECT o.order_no,o.store_id,o.total_cents,o.status,o.payment_status,o.settlement_mode_snapshot,t.payment_provider,t.payment_merchant_no,t.payment_sub_appid,o.customer_openid,
		t.payment_onboarding_status,t.payment_product_authorization_status
		FROM orders o
		JOIN tenants t ON t.id=o.tenant_id AND t.status='ACTIVE'
			AND (t.service_expires_at IS NULL OR t.service_expires_at >= CURRENT_DATE) AND t.deleted_at IS NULL
		JOIN stores st ON st.id=o.store_id AND st.tenant_id=o.tenant_id AND st.status='ACTIVE' AND st.deleted_at IS NULL
		WHERE o.id=? AND o.tenant_id=?`, orderID, tenantID).
		Scan(&orderNo, &storeID, &amount, &orderStatus, &orderPaymentStatus, &settlementMode, &tenantPaymentProvider, &merchantNo, &subAppID, &storedOpenID, &onboardingStatus, &productAuthorizationStatus)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	postPay := settlementMode == "PAY_AFTER"
	payableStatus := orderStatus == "PENDING_PAYMENT"
	if postPay {
		payableStatus = validStatus(orderStatus, "PAID", "ACCEPTED", "PREPARING", "READY")
	}
	if !payableStatus || orderPaymentStatus != "UNPAID" {
		writeError(w, http.StatusConflict, "ORDER_NOT_PAYABLE", "order is not pending payment")
		return
	}
	if tenantPaymentProvider != s.Payment.Name() {
		writeError(w, http.StatusServiceUnavailable, "PAYMENT_PROVIDER_UNAVAILABLE", "merchant payment provider is not active on the platform")
		return
	}
	if tenantPaymentProvider == "wechat_partner" && (merchantNo == "" || onboardingStatus != "ACTIVE" || productAuthorizationStatus != "AUTHORIZED") {
		writeError(w, http.StatusConflict, "WECHAT_PAY_MERCHANT_NOT_READY", "WeChat Pay sub-merchant onboarding or product authorization is incomplete")
		return
	}
	newReservation := false
	if !postPay {
		var reserveErr error
		newReservation, reserveErr = ensureOrderStockReservationLocked(r.Context(), conn, tenantID, orderID)
		if errors.Is(reserveErr, errInsufficientStock) {
			writeError(w, http.StatusConflict, "ITEM_UNAVAILABLE", "inventory changed while this order was waiting for late payment")
			return
		}
		if errors.Is(reserveErr, errOrderNotPayable) {
			writeError(w, http.StatusConflict, "ORDER_NOT_PAYABLE", "order is not pending payment")
			return
		}
		if reserveErr != nil {
			handleSQLError(w, reserveErr)
			return
		}
	}
	if input.OpenID == "" {
		input.OpenID = storedOpenID
	} else if storedOpenID == "" {
		if _, err = conn.ExecContext(r.Context(), "UPDATE orders SET customer_openid=? WHERE id=? AND tenant_id=? AND customer_openid=''", input.OpenID, orderID, tenantID); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	var existingID int64
	var existingProvider, existingNo, existingStatus, raw string
	var intent paymentCreationIntent
	renewReservation := newReservation && !postPay
	err = conn.QueryRowContext(r.Context(), "SELECT id,provider,provider_order_no,status,COALESCE(raw_response,'') FROM payment_transactions WHERE tenant_id=? AND order_id=? ORDER BY id DESC LIMIT 1", tenantID, orderID).Scan(&existingID, &existingProvider, &existingNo, &existingStatus, &raw)
	createAttempt := errors.Is(err, sql.ErrNoRows)
	if err == nil {
		switch existingStatus {
		case string(provider.PaymentSuccess):
			if err = s.markPaymentPaidLocked(r.Context(), conn, existingProvider, existingNo, time.Now()); err != nil {
				handleSQLError(w, err)
				return
			}
			var payParams map[string]string
			_ = json.Unmarshal([]byte(raw), &payParams)
			writePaymentResponse(w, http.StatusOK, existingID, existingProvider, existingNo, existingStatus, payParams)
			return
		case string(provider.PaymentPending), string(provider.PaymentRefunded):
			var payParams map[string]string
			_ = json.Unmarshal([]byte(raw), &payParams)
			writePaymentResponse(w, http.StatusOK, existingID, existingProvider, existingNo, existingStatus, payParams)
			return
		case paymentStatusCreating:
			// Resume exactly the durable routing snapshot. Tenant bindings and the
			// caller's OpenID may have changed since the first provider attempt.
			intent, err = s.loadPaymentCreationIntent(r.Context(), conn, existingID)
		case string(provider.PaymentFailed), string(provider.PaymentClosed):
			// Payment attempts are append-only. A fresh provider request number is
			// essential because providers retain CLOSED/FAILED idempotency keys,
			// and preserving the prior row keeps delayed callbacks auditable.
			createAttempt = true
			intent = paymentCreationIntent{}
			renewReservation = !postPay
		default:
			writeError(w, http.StatusConflict, "PAYMENT_STATE_UNSUPPORTED", "payment is in an unsupported state")
			return
		}
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	if createAttempt {
		renewReservation = !postPay
		providerRequestNo := newBusinessNo("PY")
		existingNo = localPaymentReference(providerRequestNo)
		var dbResult sql.Result
		dbResult, err = conn.ExecContext(r.Context(), `INSERT INTO payment_transactions(tenant_id,store_id,order_id,provider,merchant_no,sub_appid,customer_openid,provider_request_no,provider_order_no,amount_cents,status,raw_response)
			VALUES(?,?,?,?,?,?,?,?,?,?,'CREATING','{}')`, tenantID, storeID, orderID, s.Payment.Name(), merchantNo, subAppID, input.OpenID, providerRequestNo, existingNo, amount)
		if err == nil {
			existingID, _ = dbResult.LastInsertId()
		}
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if intent.ID == 0 {
		intent, err = s.loadPaymentCreationIntent(r.Context(), conn, existingID)
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if renewReservation && !newReservation {
		if _, err = conn.ExecContext(r.Context(), `UPDATE orders SET stock_reserved_at=NOW(3)
			WHERE id=? AND tenant_id=? AND status='PENDING_PAYMENT' AND payment_status='UNPAID' AND inventory_reserved=1`, orderID, tenantID); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	result, submitErr := s.submitPaymentIntent(r.Context(), conn, intent)
	if submitErr != nil {
		if errors.Is(submitErr, provider.ErrNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "PAYMENT_PROVIDER_NOT_CONFIGURED", "the selected payment provider is not ready for transactions")
			return
		}
		s.Logger.Warn("payment creation outcome is pending reconciliation", "payment_id", existingID, "order_id", orderID, "error", submitErr)
		writePaymentResponse(w, http.StatusAccepted, existingID, s.Payment.Name(), localPaymentReference(intent.ProviderRequestNo), paymentStatusCreating, map[string]string{"mode": "processing"})
		return
	}
	statusCode := http.StatusOK
	if createAttempt {
		statusCode = http.StatusCreated
	}
	writePaymentResponse(w, statusCode, existingID, s.Payment.Name(), result.ProviderOrderNo, string(result.Status), result.PayParams)
}

func writePaymentResponse(w http.ResponseWriter, statusCode int, id int64, providerName, providerNo, status string, payParams map[string]string) {
	writeData(w, statusCode, map[string]any{"id": id, "paymentId": id, "provider": providerName, "provider_order_no": providerNo, "providerOrderNo": providerNo, "status": status, "pay_params": payParams, "wxPayParams": payParams})
}

func (s *Server) closePendingPaymentLocked(ctx context.Context, conn *sql.Conn, tenantID, orderID int64) error {
	var id int64
	var providerName, providerNo, status string
	err := conn.QueryRowContext(ctx, "SELECT id,provider,provider_order_no,status FROM payment_transactions WHERE tenant_id=? AND order_id=? ORDER BY id DESC LIMIT 1", tenantID, orderID).Scan(&id, &providerName, &providerNo, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status == string(provider.PaymentSuccess) || status == string(provider.PaymentRefunded) {
		return errPaymentAlreadyPaid
	}
	if status == string(provider.PaymentClosed) || status == string(provider.PaymentFailed) {
		return nil
	}
	if providerName != s.Payment.Name() {
		return fmt.Errorf("payment provider %s is not active", providerName)
	}
	if status == paymentStatusCreating {
		intent, loadErr := s.loadPaymentCreationIntent(ctx, conn, id)
		if loadErr != nil {
			return loadErr
		}
		creation, submitErr := s.submitPaymentIntent(ctx, conn, intent)
		if submitErr != nil {
			return fmt.Errorf("resolve creating payment before close: %w", submitErr)
		}
		providerNo = creation.ProviderOrderNo
		status = string(creation.Status)
		if status == string(provider.PaymentSuccess) {
			return errPaymentAlreadyPaid
		}
		if status == string(provider.PaymentFailed) || status == string(provider.PaymentClosed) {
			return nil
		}
	}
	if status != string(provider.PaymentPending) {
		return fmt.Errorf("payment cannot be closed from status %s", status)
	}
	providerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err = s.Payment.Close(providerCtx, providerNo); err != nil {
		return err
	}
	result, err := conn.ExecContext(ctx, "UPDATE payment_transactions SET status='CLOSED' WHERE id=? AND tenant_id=? AND status='PENDING'", id, tenantID)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		var current string
		if queryErr := conn.QueryRowContext(ctx, "SELECT status FROM payment_transactions WHERE id=? AND tenant_id=?", id, tenantID).Scan(&current); queryErr != nil {
			return queryErr
		}
		if current == string(provider.PaymentSuccess) {
			return errPaymentAlreadyPaid
		}
	}
	return nil
}

func (s *Server) mockConfirm(w http.ResponseWriter, r *http.Request) {
	if !s.AllowMockConfirmation || s.Payment.Name() != "mock" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "mock confirmation endpoint is disabled")
		return
	}
	providerNo := chi.URLParam(r, "providerOrderNo")
	var currentStatus string
	if err := s.DB.QueryRowContext(r.Context(), "SELECT status FROM payment_transactions WHERE provider='mock' AND provider_order_no=?", providerNo).Scan(&currentStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if currentStatus == string(provider.PaymentClosed) {
		writeError(w, http.StatusConflict, "PAYMENT_CLOSED", "closed payment cannot be confirmed")
		return
	}
	if !s.MockPayment.Confirm(providerNo) {
		writeError(w, http.StatusConflict, "MOCK_PAYMENT_NOT_PENDING", "mock payment is missing, closed, or already confirmed")
		return
	}
	if err := s.markPaymentPaid(r.Context(), "mock", providerNo, time.Now()); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"provider_order_no": providerNo, "status": "SUCCESS"})
}

func (s *Server) tianQueCallback(w http.ResponseWriter, r *http.Request) {
	if s.Payment.Name() != "tianque" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "tianque provider is not active")
		return
	}
	// Production implementation must verify TianQue's RSA signature before
	// reading these fields. The route is retained now so DNS/contract remain stable.
	writeError(w, http.StatusNotImplemented, "TIANQUE_NOT_CONFIGURED", "callback signature verification awaits partner credentials")
}

func (s *Server) wechatPayCallback(w http.ResponseWriter, r *http.Request) {
	if s.Payment.Name() != "wechat_partner" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "WeChat Pay provider is not active")
		return
	}
	// Never acknowledge a payment notification before API v3 signature
	// verification, resource decryption and payment identity checks exist.
	writeError(w, http.StatusNotImplemented, "WECHAT_PAY_NOT_CONFIGURED", "WeChat Pay notification verification is not implemented")
}

func (s *Server) wechatPayRefundCallback(w http.ResponseWriter, r *http.Request) {
	if s.Payment.Name() != "wechat_partner" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "WeChat Pay provider is not active")
		return
	}
	// Refund notifications have the same fail-closed boundary as payment
	// notifications. Returning a non-success response prevents false success.
	writeError(w, http.StatusNotImplemented, "WECHAT_PAY_REFUND_NOT_CONFIGURED", "WeChat Pay refund notification verification is not implemented")
}

func (s *Server) markPaymentPaid(ctx context.Context, providerName, providerNo string, paidAt time.Time) error {
	var tenantID, orderID int64
	if err := s.DB.QueryRowContext(ctx, "SELECT tenant_id,order_id FROM payment_transactions WHERE provider=? AND provider_order_no=?", providerName, providerNo).Scan(&tenantID, &orderID); err != nil {
		return err
	}
	conn, release, err := s.acquirePaymentOrderLock(ctx, tenantID, orderID)
	if err != nil {
		return err
	}
	defer release()
	return s.markPaymentPaidLocked(ctx, conn, providerName, providerNo, paidAt)
}

// markPaymentPaidLocked must only be called while the per-order named lock is
// held. Payment creation/renewal and callbacks consequently share one order
// state machine instead of racing through separate database transactions.
func (s *Server) markPaymentPaidLocked(ctx context.Context, conn *sql.Conn, providerName, providerNo string, paidAt time.Time) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var paymentID, tenantID, storeID, orderID int64
	var paymentStatus, orderStatus, orderPaymentStatus, settlementMode string
	var inventoryReserved int
	err = tx.QueryRowContext(ctx, `SELECT p.id,p.tenant_id,p.store_id,p.order_id,p.status,o.status,o.payment_status,o.settlement_mode_snapshot,o.inventory_reserved
		FROM payment_transactions p JOIN orders o ON o.id=p.order_id
		WHERE p.provider=? AND p.provider_order_no=? FOR UPDATE`, providerName, providerNo).
		Scan(&paymentID, &tenantID, &storeID, &orderID, &paymentStatus, &orderStatus, &orderPaymentStatus, &settlementMode, &inventoryReserved)
	if err != nil {
		return err
	}
	newlySucceeded := paymentStatus != string(provider.PaymentSuccess)
	var newerActiveID int64
	var newerActiveProvider, newerActiveNo, newerActiveStatus, newerCloseError string
	if newlySucceeded {
		queryErr := tx.QueryRowContext(ctx, `SELECT id,provider,provider_order_no,status FROM payment_transactions
			WHERE tenant_id=? AND order_id=? AND id>? AND status IN ('CREATING','PENDING')
			ORDER BY id DESC LIMIT 1 FOR UPDATE`, tenantID, orderID, paymentID).
			Scan(&newerActiveID, &newerActiveProvider, &newerActiveNo, &newerActiveStatus)
		if queryErr != nil && !errors.Is(queryErr, sql.ErrNoRows) {
			return queryErr
		}
		if queryErr == nil {
			switch {
			case newerActiveStatus != string(provider.PaymentPending):
				newerCloseError = "newer payment creation is still unresolved"
			case newerActiveProvider != s.Payment.Name():
				newerCloseError = "newer payment belongs to an inactive provider"
			default:
				closeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				closeErr := s.Payment.Close(closeCtx, newerActiveNo)
				cancel()
				if closeErr != nil {
					newerCloseError = truncateError(closeErr)
				} else {
					result, updateErr := tx.ExecContext(ctx, "UPDATE payment_transactions SET status='CLOSED' WHERE id=? AND tenant_id=? AND status='PENDING'", newerActiveID, tenantID)
					if updateErr != nil {
						return updateErr
					}
					if changed, _ := result.RowsAffected(); changed != 1 {
						newerCloseError = "newer payment changed while it was being closed"
					}
				}
			}
		}
	}
	if newlySucceeded {
		if _, err = tx.ExecContext(ctx, "UPDATE payment_transactions SET status='SUCCESS',paid_at=? WHERE id=?", paidAt, paymentID); err != nil {
			return err
		}
	}
	if orderPaymentStatus == "UNPAID" {
		targetStatus := "PAID"
		if settlementMode == "PAY_AFTER" {
			targetStatus = "COMPLETED"
		}
		if orderStatus == "CLOSED" || inventoryReserved != 1 || newerActiveID != 0 {
			targetStatus = "PAYMENT_EXCEPTION"
		}
		if _, err = tx.ExecContext(ctx, "UPDATE orders SET status=?,payment_status='PAID',inventory_reserved=0,stock_reserved_at=NULL,paid_cents=total_cents,paid_at=?,completed_at=IF(?='COMPLETED',?,completed_at) WHERE id=?", targetStatus, paidAt, targetStatus, paidAt, orderID); err != nil {
			return err
		}
		if err = useOrderCoupon(ctx, tx, tenantID, orderID); err != nil {
			return err
		}
		if targetStatus == "PAYMENT_EXCEPTION" {
			s.Logger.Error("payment succeeded on an exceptional order path", "order_id", orderID, "provider_order_no", providerNo, "previous_order_status", orderStatus, "newer_payment_id", newerActiveID, "newer_payment_close_error", newerCloseError)
		} else if err = enqueuePrintOutboxWith(ctx, tx, tenantID, storeID, orderID, "PAYMENT_SUCCESS", paymentPrintDedupeKey(paymentID), 0, ""); err != nil {
			return err
		}
	} else if newlySucceeded {
		// A different attempt already paid this order. Preserve both provider
		// transactions, surface an exception, and require an operator to refund
		// the duplicate receipt instead of silently treating it as idempotent.
		if _, err = tx.ExecContext(ctx, "UPDATE orders SET status='PAYMENT_EXCEPTION' WHERE id=?", orderID); err != nil {
			return err
		}
		s.Logger.Error("additional payment attempt succeeded for an already paid order", "order_id", orderID, "payment_id", paymentID, "provider_order_no", providerNo, "order_payment_status", orderPaymentStatus)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

type refundInput struct {
	OrderID     int64  `json:"order_id"`
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
}

func (s *Server) createRefund(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input refundInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.OrderID <= 0 || input.AmountCents <= 0 || input.AmountCents > maxBusinessAmountCents {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "order_id and amount_cents inside the supported range are required")
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required and must not exceed 128 characters")
		return
	}
	fingerprint := requestFingerprint(input)
	var existingID int64
	var existingFingerprint string
	err := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM refunds WHERE tenant_id=? AND idempotency_key=?", identity.TenantID, idempotencyKey).Scan(&existingID, &existingFingerprint)
	if err == nil {
		if existingFingerprint != "" && existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different refund request")
			return
		}
		s.writeRefund(w, r, http.StatusOK, identity.TenantID, existingID)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var storeID, totalCents, paymentID int64
	var providerNo, merchantNo, paymentStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT o.store_id,o.paid_cents,p.id,p.provider_order_no,
		p.merchant_no,o.payment_status
		FROM orders o JOIN payment_transactions p ON p.order_id=o.id AND p.status='SUCCESS'
		WHERE o.id=? AND o.tenant_id=? ORDER BY p.id DESC LIMIT 1 FOR UPDATE`, input.OrderID, identity.TenantID).
		Scan(&storeID, &totalCents, &paymentID, &providerNo, &merchantNo, &paymentStatus)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if paymentStatus != "PAID" && paymentStatus != "PARTIALLY_REFUNDED" {
		writeError(w, http.StatusConflict, "ORDER_NOT_REFUNDABLE", "order payment is not refundable")
		return
	}
	var reserved int64
	if err = tx.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(amount_cents),0) FROM refunds WHERE order_id=? AND status IN ('PENDING','SUCCESS')", input.OrderID).Scan(&reserved); err != nil {
		handleSQLError(w, err)
		return
	}
	if totalCents < 0 || reserved < 0 || reserved > totalCents || input.AmountCents > totalCents-reserved {
		writeError(w, http.StatusConflict, "REFUND_EXCEEDS_PAID_AMOUNT", "cumulative refund exceeds paid amount")
		return
	}
	refundNo := newBusinessNo("RF")
	result, err := tx.ExecContext(r.Context(), `INSERT INTO refunds(tenant_id,store_id,order_id,payment_id,refund_no,idempotency_key,request_fingerprint,amount_cents,reason,status,created_by) VALUES(?,?,?,?,?,?,?,?,?, 'PENDING',?)`, identity.TenantID, storeID, input.OrderID, paymentID, refundNo, idempotencyKey, fingerprint, input.AmountCents, input.Reason, identity.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			_ = tx.Rollback()
			if duplicateErr := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM refunds WHERE tenant_id=? AND idempotency_key=?", identity.TenantID, idempotencyKey).Scan(&existingID, &existingFingerprint); duplicateErr == nil {
				if existingFingerprint != "" && existingFingerprint != fingerprint {
					writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different refund request")
					return
				}
				s.writeRefund(w, r, http.StatusOK, identity.TenantID, existingID)
				return
			}
		}
		handleSQLError(w, err)
		return
	}
	refundID, _ := result.LastInsertId()
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	providerCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	providerResult, err := s.Payment.Refund(providerCtx, provider.RefundRequest{MerchantNo: merchantNo, ProviderOrderNo: providerNo, RefundNo: refundNo, Amount: input.AmountCents})
	if err != nil {
		// A transport error is an unknown outcome, not proof that the provider
		// rejected the refund. Keep it pending for query-based reconciliation.
		_, _ = s.DB.ExecContext(r.Context(), "UPDATE refunds SET last_error=? WHERE id=? AND status='PENDING'", truncateError(err), refundID)
		s.audit(r.Context(), identity, "refund.create.pending", "refund", int64String(refundID), input, r)
		s.writeRefund(w, r, http.StatusAccepted, identity.TenantID, refundID)
		return
	}
	if providerResult.Status == provider.PaymentSuccess {
		if strings.TrimSpace(providerResult.ProviderRefundNo) == "" {
			_, _ = s.DB.ExecContext(r.Context(), "UPDATE refunds SET last_error='provider returned an empty refund number',updated_at=NOW(3) WHERE id=? AND status='PENDING'", refundID)
			s.writeRefund(w, r, http.StatusAccepted, identity.TenantID, refundID)
			return
		}
		if err = s.finalizeRefund(r.Context(), refundID, providerResult.ProviderRefundNo); err != nil {
			// Provider success is authoritative. Leave the record pending so the
			// reconciliation worker can finish local accounting without re-refunding.
			_, _ = s.DB.ExecContext(r.Context(), "UPDATE refunds SET provider_refund_no=?,last_error=? WHERE id=? AND status='PENDING'", providerResult.ProviderRefundNo, truncateError(err), refundID)
			handleSQLError(w, err)
			return
		}
	} else if providerResult.Status == provider.PaymentFailed || providerResult.Status == provider.PaymentClosed {
		_, _ = s.DB.ExecContext(r.Context(), "UPDATE refunds SET status='FAILED',provider_refund_no=?,last_error='provider rejected refund' WHERE id=? AND status='PENDING'", providerResult.ProviderRefundNo, refundID)
	}
	s.audit(r.Context(), identity, "refund.create", "refund", int64String(refundID), input, r)
	statusCode := http.StatusCreated
	if providerResult.Status == provider.PaymentPending {
		statusCode = http.StatusAccepted
	}
	s.writeRefund(w, r, statusCode, identity.TenantID, refundID)
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 500 {
		return message[:500]
	}
	return message
}

func (s *Server) finalizeRefund(ctx context.Context, refundID int64, providerRefundNo string) error {
	if strings.TrimSpace(providerRefundNo) == "" {
		return errors.New("provider refund number is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var tenantID, storeID, orderID, amount, actorID, paid, refunded int64
	var reason, status string
	err = tx.QueryRowContext(ctx, `SELECT r.tenant_id,r.store_id,r.order_id,r.amount_cents,r.reason,r.status,r.created_by,o.paid_cents,o.refunded_cents
		FROM refunds r JOIN orders o ON o.id=r.order_id WHERE r.id=? FOR UPDATE`, refundID).
		Scan(&tenantID, &storeID, &orderID, &amount, &reason, &status, &actorID, &paid, &refunded)
	if err != nil {
		return err
	}
	if status == "SUCCESS" {
		return nil
	}
	if status != "PENDING" {
		return fmt.Errorf("refund %d is not pending", refundID)
	}
	if paid < 0 || refunded < 0 || refunded > paid || amount <= 0 || amount > paid-refunded {
		return fmt.Errorf("refund %d exceeds paid amount", refundID)
	}
	newRefunded := refunded + amount
	if _, err = tx.ExecContext(ctx, "UPDATE refunds SET status='SUCCESS',provider_refund_no=?,last_error='' WHERE id=? AND status='PENDING'", providerRefundNo, refundID); err != nil {
		return err
	}
	paymentStatus := "PARTIALLY_REFUNDED"
	if newRefunded == paid {
		paymentStatus = "REFUNDED"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE orders SET refunded_cents=?,payment_status=?,status=IF(?='REFUNDED','REFUNDED',status) WHERE id=? AND tenant_id=?`, newRefunded, paymentStatus, paymentStatus, orderID, tenantID); err != nil {
		return err
	}
	if err = enqueuePrintOutboxWith(ctx, tx, tenantID, storeID, orderID, "REFUND", refundPrintDedupeKey(refundID), actorID, fmt.Sprintf("退款 %d 分：%s", amount, reason)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) writeRefund(w http.ResponseWriter, r *http.Request, statusCode int, tenantID, refundID int64) {
	var id, orderID, amount, createdBy int64
	var refundNo, providerNo, reason, status, lastError, created string
	err := s.DB.QueryRowContext(r.Context(), `SELECT id,order_id,refund_no,provider_refund_no,amount_cents,reason,status,last_error,created_by,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM refunds WHERE id=? AND tenant_id=?`, refundID, tenantID).
		Scan(&id, &orderID, &refundNo, &providerNo, &amount, &reason, &status, &lastError, &createdBy, &created)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, statusCode, map[string]any{"id": id, "order_id": orderID, "refund_no": refundNo, "provider_refund_no": providerNo, "amount_cents": amount, "reason": reason, "status": status, "last_error": lastError, "created_by": createdBy, "created_at": created})
}

func (s *Server) listRefunds(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM refunds WHERE tenant_id=?", identity.TenantID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,order_id,refund_no,provider_refund_no,amount_cents,reason,status,last_error,created_by,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM refunds WHERE tenant_id=? ORDER BY id DESC LIMIT ? OFFSET ?`, identity.TenantID, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, orderID, amount, createdBy int64
		var refundNo, providerNo, reason, status, lastError, created string
		if err := rows.Scan(&id, &orderID, &refundNo, &providerNo, &amount, &reason, &status, &lastError, &createdBy, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "order_id": orderID, "refund_no": refundNo, "provider_refund_no": providerNo, "amount_cents": amount, "reason": reason, "status": status, "last_error": lastError, "created_by": createdBy, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func newBusinessNo(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s%s%s", prefix, time.Now().Format("20060102150405"), strings.ToUpper(hex.EncodeToString(b)))
}
