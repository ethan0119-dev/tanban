package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
	"github.com/go-chi/chi/v5"
)

func (s *Server) publicRoutes(r chi.Router) {
	r.Get("/table-codes/{code}", s.publicResolveTableCode)
	r.Get("/stores/{storeCode}", s.publicStore)
	r.Get("/stores/{storeCode}/catalog", s.publicCatalog)
	r.Post("/stores/{storeCode}/orders", s.publicCreateOrder)
	r.Get("/orders/{orderNo}", s.publicGetOrder)
	r.Post("/orders/{orderNo}/pay", s.publicPayOrder)
	r.Post("/orders/{orderNo}/payments", s.publicPayOrder)
	r.Post("/payments/{paymentID}/mock-confirm", s.publicMockConfirm)
	r.Get("/customer/orders", s.publicCustomerOrders)
}

func (s *Server) publicStore(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, s.publicStoreView(r.Context(), store))
}

func (s *Server) publicCatalog(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	products, err := s.loadProducts(r.Context(), store.TenantID, store.ID, true, 0)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT id,store_id,name,sort_order,status FROM categories WHERE tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL ORDER BY sort_order,id", store.TenantID, store.ID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	categories := []categoryDTO{}
	for rows.Next() {
		var item categoryDTO
		if err := rows.Scan(&item.ID, &item.StoreID, &item.Name, &item.SortOrder, &item.Status); err != nil {
			handleSQLError(w, err)
			return
		}
		categories = append(categories, item)
	}
	publicCategories := make([]map[string]any, 0, len(categories))
	for _, category := range categories {
		publicCategories = append(publicCategories, map[string]any{"id": category.ID, "name": category.Name, "sortOrder": category.SortOrder})
	}
	publicProducts := make([]map[string]any, 0, len(products))
	for _, product := range products {
		var stock int
		var minPrice int64
		publicSKUs := make([]map[string]any, 0, len(product.SKUs))
		for index, sku := range product.SKUs {
			stock += sku.Stock
			if index == 0 || sku.PriceCents < minPrice {
				minPrice = sku.PriceCents
			}
			publicSKUs = append(publicSKUs, map[string]any{"id": sku.ID, "name": sku.Name, "price": sku.PriceCents, "stock": sku.Stock, "soldOut": sku.Stock <= 0})
		}
		configuration, configErr := s.loadProductConfiguration(r.Context(), store.TenantID, store.ID, product.ID, true)
		if configErr != nil {
			handleSQLError(w, configErr)
			return
		}
		publicProducts = append(publicProducts, map[string]any{"id": product.ID, "categoryId": product.CategoryID, "name": product.Name, "description": product.Description, "imageUrl": product.ImageURL, "price": minPrice, "stock": stock, "soldOut": len(product.SKUs) == 0 || stock <= 0, "skus": publicSKUs, "optionGroups": publicOptionGroups(configuration.OptionGroups), "modifierGroups": publicModifierGroups(configuration.ModifierGroups)})
	}
	writeData(w, http.StatusOK, map[string]any{"store": s.publicStoreView(r.Context(), store), "categories": publicCategories, "products": publicProducts})
}

func publicOptionGroups(groups []productOptionGroupDTO) []map[string]any {
	result := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		values := make([]map[string]any, 0, len(group.Values))
		for _, value := range group.Values {
			values = append(values, map[string]any{"id": value.ID, "name": value.Name, "priceDeltaCents": value.PriceDeltaCents, "isDefault": value.IsDefault})
		}
		result = append(result, map[string]any{"id": group.ID, "name": group.Name, "selectionMode": group.SelectionMode, "minSelect": group.MinSelect, "maxSelect": group.MaxSelect, "values": values})
	}
	return result
}

func publicModifierGroups(groups []modifierGroupDTO) []map[string]any {
	result := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		items := make([]map[string]any, 0, len(group.Items))
		for _, item := range group.Items {
			items = append(items, map[string]any{"id": item.ModifierItemID, "name": item.Name, "priceCents": item.PriceCents, "isDefault": item.IsDefault})
		}
		result = append(result, map[string]any{"id": group.ID, "name": group.Name, "minSelect": group.MinSelect, "maxSelect": group.MaxSelect, "items": items})
	}
	return result
}

func publicStoreView(store storeDTO) map[string]any {
	return map[string]any{"id": store.ID, "code": store.Code, "name": store.Name, "logoUrl": store.LogoURL, "address": store.Address, "businessStatus": "OPEN", "theme": map[string]any{"bannerUrl": store.BannerURL, "announcement": store.Notice}}
}

func (s *Server) publicStoreView(ctx context.Context, store storeDTO) map[string]any {
	view := publicStoreView(store)
	decoration, version := s.publicDecorationConfig(ctx, store)
	view["decoration"] = decoration
	view["decorationVersion"] = version
	return view
}

func (s *Server) findPublicStore(ctx context.Context, code string) (storeDTO, error) {
	var store storeDTO
	err := scanStore(s.DB.QueryRowContext(ctx, `SELECT s.id,s.tenant_id,s.code,s.name,s.logo_url,s.banner_url,s.address,s.phone,s.business_hours,s.notice,s.status,DATE_FORMAT(s.created_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM stores s JOIN tenants t ON t.id=s.tenant_id WHERE s.code=? AND s.status='ACTIVE' AND s.deleted_at IS NULL AND t.status='ACTIVE' AND t.deleted_at IS NULL`, code), &store)
	return store, err
}

type publicOrderItemInput struct {
	ProductID         int64                   `json:"productId"`
	SKUID             int64                   `json:"skuId"`
	LegacySKU         int64                   `json:"sku_id"`
	Quantity          int                     `json:"quantity"`
	OptionValueIDs    []int64                 `json:"optionValueIds"`
	AttributeValueIDs []int64                 `json:"attributeValueIds"`
	Modifiers         []selectedModifierInput `json:"modifiers"`
	ItemRemark        string                  `json:"itemRemark"`
}

type publicOrderInput struct {
	OpenID        string                 `json:"openid"`
	CustomerKey   string                 `json:"customerKey"`
	CustomerName  string                 `json:"customer_name"`
	CustomerPhone string                 `json:"customer_phone"`
	Fulfillment   string                 `json:"fulfillmentType"`
	Remark        string                 `json:"remark"`
	Items         []publicOrderItemInput `json:"items"`
	OrderType     string                 `json:"orderType"`
	OrderScene    string                 `json:"order_scene"`
	TablePublicID string                 `json:"table_public_id"`
	TableScene    string                 `json:"tableScene"`
}

func legacyPublicOrderFingerprint(input publicOrderInput) string {
	return requestFingerprint(struct {
		OpenID        string                 `json:"openid"`
		CustomerKey   string                 `json:"customerKey"`
		CustomerName  string                 `json:"customer_name"`
		CustomerPhone string                 `json:"customer_phone"`
		Fulfillment   string                 `json:"fulfillmentType"`
		Remark        string                 `json:"remark"`
		Items         []publicOrderItemInput `json:"items"`
	}{input.OpenID, input.CustomerKey, input.CustomerName, input.CustomerPhone, input.Fulfillment, input.Remark, input.Items})
}

func (s *Server) publicCreateOrder(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	// V1 runs a single API process, so the shared in-memory cache is sufficient
	// for a lightweight abuse guard. The cache interface can move this counter
	// to Redis when the API is scaled horizontally.
	s.publicRateMu.Lock()
	rateKey := "public-order:" + store.Code + ":" + r.RemoteAddr
	attempts := 0
	if raw, cacheErr := s.Cache.Get(r.Context(), rateKey); cacheErr == nil {
		attempts, _ = strconv.Atoi(string(raw))
	}
	if attempts >= 30 {
		s.publicRateMu.Unlock()
		writeError(w, http.StatusTooManyRequests, "ORDER_RATE_LIMITED", "too many order attempts; retry in one minute")
		return
	}
	_ = s.Cache.Set(r.Context(), rateKey, []byte(strconv.Itoa(attempts+1)), time.Minute)
	s.publicRateMu.Unlock()
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required and must not exceed 128 characters")
		return
	}
	var input publicOrderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if len(input.Items) == 0 || len(input.Items) > 100 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "between 1 and 100 items are required")
		return
	}
	orderType, typeErr := normalizeOrderType(input.OrderType, input.OrderScene, input.Fulfillment)
	if typeErr != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", typeErr.Error())
		return
	}
	if orderType == orderTypeDelivery {
		writeError(w, http.StatusBadRequest, "DELIVERY_NOT_AVAILABLE", "delivery ordering is not available in this release")
		return
	}
	input.OrderType = orderType
	input.OrderScene = ""
	input.Fulfillment = legacyFulfillmentType(orderType)
	if input.TablePublicID == "" {
		input.TablePublicID = input.TableScene
	}
	input.TablePublicID = strings.TrimSpace(strings.TrimPrefix(input.TablePublicID, "tc="))
	input.TableScene = ""
	if orderType == orderTypeDineIn && input.TablePublicID == "" {
		writeError(w, http.StatusBadRequest, "TABLE_CODE_REQUIRED", "table_public_id is required for a dine-in order")
		return
	}
	if orderType != orderTypeDineIn && input.TablePublicID != "" {
		writeError(w, http.StatusBadRequest, "TABLE_CODE_NOT_ALLOWED", "table_public_id is only valid for a dine-in order")
		return
	}
	input.OpenID = strings.TrimSpace(input.OpenID)
	input.CustomerKey = strings.TrimSpace(input.CustomerKey)
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.CustomerPhone = strings.TrimSpace(input.CustomerPhone)
	if len(input.OpenID) > 128 || len(input.CustomerKey) > 128 || len([]rune(input.CustomerName)) > 80 || len(input.CustomerPhone) > 32 || len([]rune(input.Remark)) > 500 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "customer identity or order remark is too long")
		return
	}
	fingerprint := requestFingerprint(input)
	legacyFingerprint := legacyPublicOrderFingerprint(input)
	var existingID int64
	var existingFingerprint string
	err = s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM orders WHERE tenant_id=? AND store_id=? AND idempotency_key=?", store.TenantID, store.ID, idempotencyKey).Scan(&existingID, &existingFingerprint)
	if err == nil {
		if existingFingerprint != "" && existingFingerprint != fingerprint && existingFingerprint != legacyFingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different order request")
			return
		}
		item, loadErr := s.loadOrder(r.Context(), store.TenantID, existingID, "")
		if loadErr != nil {
			handleSQLError(w, loadErr)
			return
		}
		writeData(w, http.StatusOK, publicOrderView(item))
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
	var table orderTableReference
	if input.OrderType == orderTypeDineIn {
		table, err = resolveOrderTable(r.Context(), tx, store.TenantID, store.ID, input.TablePublicID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusBadRequest, "INVALID_TABLE_CODE", "table code does not belong to this store or is disabled")
				return
			}
			handleSQLError(w, err)
			return
		}
	}
	type resolvedItem struct {
		productID, skuID, basePrice, modifierPrice, unitPrice               int64
		productName, skuName, productType, attrs, configuration, itemRemark string
		quantity                                                            int
	}
	resolved := make([]resolvedItem, 0, len(input.Items))
	var total int64
	for _, requested := range input.Items {
		if requested.SKUID == 0 {
			requested.SKUID = requested.LegacySKU
		}
		if requested.SKUID <= 0 || requested.Quantity <= 0 || requested.Quantity > 99 {
			writeError(w, http.StatusBadRequest, "INVALID_ITEM", "sku_id and quantity between 1 and 99 are required")
			return
		}
		var row resolvedItem
		var stock int
		err = tx.QueryRowContext(r.Context(), `SELECT p.id,s.id,p.name,s.name,p.product_type,s.attributes_json,s.price_cents,i.stock FROM skus s JOIN products p ON p.id=s.product_id JOIN categories c ON c.id=p.category_id AND c.tenant_id=p.tenant_id AND c.store_id=p.store_id JOIN inventory i ON i.sku_id=s.id WHERE s.id=? AND s.tenant_id=? AND s.store_id=? AND s.status='ACTIVE' AND p.status='ACTIVE' AND c.status='ACTIVE' AND s.deleted_at IS NULL AND p.deleted_at IS NULL AND c.deleted_at IS NULL FOR UPDATE`, requested.SKUID, store.TenantID, store.ID).
			Scan(&row.productID, &row.skuID, &row.productName, &row.skuName, &row.productType, &row.attrs, &row.basePrice, &stock)
		if err != nil || stock < requested.Quantity {
			writeError(w, http.StatusConflict, "ITEM_UNAVAILABLE", "an item is sold out or unavailable")
			return
		}
		if requested.ProductID > 0 && requested.ProductID != row.productID {
			writeError(w, http.StatusBadRequest, "INVALID_ITEM", "sku does not belong to the requested product")
			return
		}
		selectedOptions := requested.OptionValueIDs
		if len(selectedOptions) == 0 {
			selectedOptions = requested.AttributeValueIDs
		}
		resolvedConfiguration, configurationErr := resolveProductConfiguration(r.Context(), tx, store.TenantID, store.ID, row.productID, selectedOptions, requested.Modifiers, requested.ItemRemark)
		if configurationErr != nil {
			writeError(w, http.StatusBadRequest, "INVALID_CONFIGURATION", configurationErr.Error())
			return
		}
		row.modifierPrice = resolvedConfiguration.PriceDeltaCents
		if row.basePrice < 0 || row.basePrice > maxCatalogUnitPriceCents || row.modifierPrice > maxCatalogUnitPriceCents-row.basePrice {
			writeError(w, http.StatusConflict, "INVALID_PRICE", "configured unit price is outside the allowed range")
			return
		}
		row.unitPrice = row.basePrice + row.modifierPrice
		row.configuration = resolvedConfiguration.SnapshotJSON
		row.itemRemark = requested.ItemRemark
		if stockErr := reserveStock(r.Context(), tx, store.TenantID, requested.SKUID, requested.Quantity); errors.Is(stockErr, errInsufficientStock) {
			writeError(w, http.StatusConflict, "ITEM_UNAVAILABLE", "an item was just sold out")
			return
		} else if stockErr != nil {
			handleSQLError(w, stockErr)
			return
		}
		row.quantity = requested.Quantity
		if row.unitPrice > maxCatalogOrderCents/int64(row.quantity) {
			writeError(w, http.StatusConflict, "ORDER_AMOUNT_LIMIT", "order amount exceeds the allowed range")
			return
		}
		subtotal := row.unitPrice * int64(row.quantity)
		if subtotal > maxCatalogOrderCents-total {
			writeError(w, http.StatusConflict, "ORDER_AMOUNT_LIMIT", "order amount exceeds the allowed range")
			return
		}
		total += subtotal
		resolved = append(resolved, row)
	}
	customerID, err := upsertPublicOrderCustomer(r.Context(), tx, store, input)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	orderNo := newBusinessNo("TB")
	result, err := tx.ExecContext(r.Context(), `INSERT INTO orders(tenant_id,store_id,order_no,idempotency_key,request_fingerprint,customer_openid,customer_id,customer_name,customer_phone,remark,fulfillment_type,order_type,table_id,table_public_id_snapshot,table_area_name_snapshot,table_name_snapshot,table_code_snapshot,inventory_reserved,stock_reserved_at,total_cents)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,NOW(3),?)`, store.TenantID, store.ID, orderNo, idempotencyKey, fingerprint, input.OpenID, nullableID(customerID), input.CustomerName, input.CustomerPhone, input.Remark, input.Fulfillment, input.OrderType, nullableID(table.ID), table.PublicID, table.AreaName, table.Name, table.TableCode, total)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			_ = tx.Rollback()
			if e := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM orders WHERE tenant_id=? AND store_id=? AND idempotency_key=?", store.TenantID, store.ID, idempotencyKey).Scan(&existingID, &existingFingerprint); e == nil {
				if existingFingerprint != "" && existingFingerprint != fingerprint && existingFingerprint != legacyFingerprint {
					writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different order request")
					return
				}
				item, loadErr := s.loadOrder(r.Context(), store.TenantID, existingID, "")
				if loadErr != nil {
					handleSQLError(w, loadErr)
					return
				}
				writeData(w, http.StatusOK, publicOrderView(item))
				return
			}
		}
		handleSQLError(w, err)
		return
	}
	orderID, _ := result.LastInsertId()
	for _, row := range resolved {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO order_items(tenant_id,order_id,product_id,sku_id,product_name,sku_name,product_type,base_price_cents,modifier_price_cents,attributes_json,configuration_json,item_remark,unit_price_cents,quantity,subtotal_cents) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, store.TenantID, orderID, row.productID, row.skuID, row.productName, row.skuName, row.productType, row.basePrice, row.modifierPrice, row.attrs, row.configuration, row.itemRemark, row.unitPrice, row.quantity, row.unitPrice*int64(row.quantity))
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = enqueuePrintOutboxWith(r.Context(), tx, store.TenantID, store.ID, orderID, "ORDER_CREATED", orderCreatedPrintDedupeKey(orderID), 0, ""); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	item, err := s.loadOrder(r.Context(), store.TenantID, orderID, "")
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, publicOrderView(item))
}

// upsertPublicOrderCustomer links a checkout to a stable CRM customer without
// treating a client-provided identifier as authentication. A verified WeChat
// OpenID wins when present; otherwise an install-scoped guest key groups repeat
// anonymous orders until the real wx.login flow is connected.
func upsertPublicOrderCustomer(ctx context.Context, tx *sql.Tx, store storeDTO, input publicOrderInput) (int64, error) {
	identityColumn := "guest_key"
	identityValue := input.CustomerKey
	source := "MINIPROGRAM_GUEST"
	if input.OpenID != "" {
		identityColumn = "wechat_openid"
		identityValue = input.OpenID
		source = "MINIPROGRAM"
	}
	if identityValue == "" {
		return 0, nil
	}
	publicID := "CU" + strings.ToUpper(requestFingerprint(map[string]any{"tenantId": store.TenantID, "identityType": identityColumn, "identityValue": identityValue})[:32])
	name := input.CustomerName
	if name == "" {
		suffix := identityValue
		if len(suffix) > 6 {
			suffix = suffix[len(suffix)-6:]
		}
		name = "小程序顾客 " + suffix
	}
	query := `INSERT INTO customers(tenant_id,source_store_id,public_id,` + identityColumn + `,name,phone,source,status,last_seen_at)
		VALUES(?,?,?,?,?,?,?,'ACTIVE',NOW(3))
		ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id),source_store_id=COALESCE(source_store_id,VALUES(source_store_id)),
		name=IF(name='',VALUES(name),name),phone=IF(phone='',VALUES(phone),phone),
		status=IF(deleted_at IS NOT NULL,'ACTIVE',status),deleted_at=NULL,last_seen_at=NOW(3)`
	result, err := tx.ExecContext(ctx, query, store.TenantID, store.ID, publicID, identityValue, name, input.CustomerPhone, source)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Server) publicGetOrder(w http.ResponseWriter, r *http.Request) {
	orderNo := chi.URLParam(r, "orderNo")
	var tenantID int64
	if err := s.DB.QueryRowContext(r.Context(), "SELECT tenant_id FROM orders WHERE order_no=?", orderNo).Scan(&tenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	item, err := s.loadOrder(r.Context(), tenantID, 0, orderNo)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, publicOrderView(item))
}

type publicPaymentInput struct {
	Provider string `json:"provider"`
	OpenID   string `json:"openid"`
	SubAppID string `json:"subAppId"`
}

func (s *Server) publicPayOrder(w http.ResponseWriter, r *http.Request) {
	orderNo := chi.URLParam(r, "orderNo")
	var tenantID, id int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT o.tenant_id,o.id FROM orders o
		JOIN tenants t ON t.id=o.tenant_id AND t.status='ACTIVE' AND t.deleted_at IS NULL
		JOIN stores st ON st.id=o.store_id AND st.status='ACTIVE' AND st.deleted_at IS NULL
		WHERE o.order_no=?`, orderNo).Scan(&tenantID, &id); err != nil {
		handleSQLError(w, err)
		return
	}
	var requested publicPaymentInput
	if !decodeJSON(w, r, &requested) {
		return
	}
	if requested.Provider != "" && strings.ToLower(requested.Provider) != s.Payment.Name() {
		writeError(w, http.StatusBadRequest, "PAYMENT_PROVIDER_UNAVAILABLE", "requested payment provider is not active")
		return
	}
	s.createPaymentForOrder(w, r, tenantID, id, paymentInput{OpenID: requested.OpenID, SubAppID: requested.SubAppID})
}

func (s *Server) publicMockConfirm(w http.ResponseWriter, r *http.Request) {
	if !s.AllowMockConfirmation || s.Payment.Name() != "mock" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "mock confirmation endpoint is disabled")
		return
	}
	paymentID, ok := pathID(w, r, "paymentID")
	if !ok {
		return
	}
	var providerNo, currentStatus string
	if err := s.DB.QueryRowContext(r.Context(), "SELECT provider_order_no,status FROM payment_transactions WHERE id=? AND provider='mock'", paymentID).Scan(&providerNo, &currentStatus); err != nil {
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
	writeData(w, http.StatusOK, map[string]any{"id": paymentID, "provider": "mock", "status": "SUCCEEDED"})
}

func (s *Server) publicCustomerOrders(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, []any{})
}

func publicOrderView(order orderDTO) map[string]any {
	items := make([]map[string]any, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, map[string]any{"productId": item.ProductID, "skuId": item.SKUID, "name": item.ProductName, "skuName": item.SKUName, "configuration": item.Configuration, "itemRemark": item.ItemRemark, "price": item.UnitPriceCents, "quantity": item.Quantity, "amount": item.SubtotalCents})
	}
	paymentStatus := order.PaymentStatus
	if paymentStatus == "PAID" {
		paymentStatus = "SUCCEEDED"
	}
	view := map[string]any{"id": order.ID, "orderNo": order.OrderNo, "pickupCode": fmt.Sprintf("%04d", order.ID%10000), "status": order.Status, "paymentStatus": paymentStatus, "fulfillmentType": order.Fulfillment, "orderType": order.OrderType, "orderScene": order.OrderType, "order_scene": order.OrderType, "remark": order.Remark, "amount": order.TotalCents, "refundedAmount": order.RefundedCents, "createdAt": order.CreatedAt, "items": items}
	if order.Table != nil {
		view["table"] = map[string]any{"publicId": order.Table.PublicID, "name": order.Table.Name, "areaName": order.Table.AreaName, "tableCode": order.Table.TableCode}
		view["tablePublicId"] = order.Table.PublicID
		view["tableName"] = order.Table.Name
		view["tableCode"] = order.Table.TableCode
		view["tableAreaName"] = order.Table.AreaName
	}
	return view
}
