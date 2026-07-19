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
	writeData(w, http.StatusOK, publicStoreView(store))
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
		publicProducts = append(publicProducts, map[string]any{"id": product.ID, "categoryId": product.CategoryID, "name": product.Name, "description": product.Description, "imageUrl": product.ImageURL, "price": minPrice, "stock": stock, "soldOut": len(product.SKUs) == 0 || stock <= 0, "skus": publicSKUs})
	}
	writeData(w, http.StatusOK, map[string]any{"store": publicStoreView(store), "categories": publicCategories, "products": publicProducts})
}

func publicStoreView(store storeDTO) map[string]any {
	return map[string]any{"id": store.ID, "code": store.Code, "name": store.Name, "logoUrl": store.LogoURL, "address": store.Address, "businessStatus": "OPEN", "theme": map[string]any{"bannerUrl": store.BannerURL, "announcement": store.Notice}}
}

func (s *Server) findPublicStore(ctx context.Context, code string) (storeDTO, error) {
	var store storeDTO
	err := scanStore(s.DB.QueryRowContext(ctx, `SELECT s.id,s.tenant_id,s.code,s.name,s.logo_url,s.banner_url,s.address,s.phone,s.business_hours,s.notice,s.status,DATE_FORMAT(s.created_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM stores s JOIN tenants t ON t.id=s.tenant_id WHERE s.code=? AND s.status='ACTIVE' AND s.deleted_at IS NULL AND t.status='ACTIVE' AND t.deleted_at IS NULL`, code), &store)
	return store, err
}

type publicOrderInput struct {
	OpenID        string `json:"openid"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	Fulfillment   string `json:"fulfillmentType"`
	Remark        string `json:"remark"`
	Items         []struct {
		ProductID int64 `json:"productId"`
		SKUID     int64 `json:"skuId"`
		LegacySKU int64 `json:"sku_id"`
		Quantity  int   `json:"quantity"`
	} `json:"items"`
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
	if input.Fulfillment != "" && input.Fulfillment != "PICKUP" && input.Fulfillment != "DINE_IN" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "fulfillmentType must be PICKUP or DINE_IN")
		return
	}
	if input.Fulfillment == "" {
		input.Fulfillment = "PICKUP"
	}
	fingerprint := requestFingerprint(input)
	var existingID int64
	var existingFingerprint string
	err = s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM orders WHERE tenant_id=? AND store_id=? AND idempotency_key=?", store.TenantID, store.ID, idempotencyKey).Scan(&existingID, &existingFingerprint)
	if err == nil {
		if existingFingerprint != "" && existingFingerprint != fingerprint {
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
	type resolvedItem struct {
		productID, skuID, unitPrice int64
		productName, skuName, attrs string
		quantity                    int
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
		err = tx.QueryRowContext(r.Context(), `SELECT p.id,s.id,p.name,s.name,s.attributes_json,s.price_cents,i.stock FROM skus s JOIN products p ON p.id=s.product_id JOIN inventory i ON i.sku_id=s.id WHERE s.id=? AND s.tenant_id=? AND s.store_id=? AND s.status='ACTIVE' AND p.status='ACTIVE' AND s.deleted_at IS NULL AND p.deleted_at IS NULL FOR UPDATE`, requested.SKUID, store.TenantID, store.ID).
			Scan(&row.productID, &row.skuID, &row.productName, &row.skuName, &row.attrs, &row.unitPrice, &stock)
		if err != nil || stock < requested.Quantity {
			writeError(w, http.StatusConflict, "ITEM_UNAVAILABLE", "an item is sold out or unavailable")
			return
		}
		if stockErr := reserveStock(r.Context(), tx, store.TenantID, requested.SKUID, requested.Quantity); errors.Is(stockErr, errInsufficientStock) {
			writeError(w, http.StatusConflict, "ITEM_UNAVAILABLE", "an item was just sold out")
			return
		} else if stockErr != nil {
			handleSQLError(w, stockErr)
			return
		}
		row.quantity = requested.Quantity
		total += row.unitPrice * int64(row.quantity)
		resolved = append(resolved, row)
	}
	orderNo := newBusinessNo("TB")
	result, err := tx.ExecContext(r.Context(), `INSERT INTO orders(tenant_id,store_id,order_no,idempotency_key,request_fingerprint,customer_openid,customer_name,customer_phone,remark,fulfillment_type,total_cents) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, store.TenantID, store.ID, orderNo, idempotencyKey, fingerprint, input.OpenID, input.CustomerName, input.CustomerPhone, input.Remark, input.Fulfillment, total)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			_ = tx.Rollback()
			if e := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM orders WHERE tenant_id=? AND store_id=? AND idempotency_key=?", store.TenantID, store.ID, idempotencyKey).Scan(&existingID, &existingFingerprint); e == nil {
				if existingFingerprint != "" && existingFingerprint != fingerprint {
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
		_, err = tx.ExecContext(r.Context(), `INSERT INTO order_items(tenant_id,order_id,product_id,sku_id,product_name,sku_name,attributes_json,unit_price_cents,quantity,subtotal_cents) VALUES(?,?,?,?,?,?,?,?,?,?)`, store.TenantID, orderID, row.productID, row.skuID, row.productName, row.skuName, row.attrs, row.unitPrice, row.quantity, row.unitPrice*int64(row.quantity))
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = s.enqueueOrderPrintsWith(r.Context(), tx, store.TenantID, store.ID, orderID, "ORDER_CREATED", false, 0, ""); err != nil {
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
		items = append(items, map[string]any{"productId": item.ProductID, "skuId": item.SKUID, "name": item.ProductName, "skuName": item.SKUName, "price": item.UnitPriceCents, "quantity": item.Quantity, "amount": item.SubtotalCents})
	}
	paymentStatus := order.PaymentStatus
	if paymentStatus == "PAID" {
		paymentStatus = "SUCCEEDED"
	}
	return map[string]any{"id": order.ID, "orderNo": order.OrderNo, "pickupCode": fmt.Sprintf("%04d", order.ID%10000), "status": order.Status, "paymentStatus": paymentStatus, "fulfillmentType": order.Fulfillment, "amount": order.TotalCents, "refundedAmount": order.RefundedCents, "createdAt": order.CreatedAt, "items": items}
}
