package app

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) merchantRoutes(r chi.Router) {
	r.Use(requireRoles(RoleMerchantOwner, RoleMerchantManager, RoleMerchantStaff))

	// Store staff can operate the live order queue and recover printing, but
	// financial, catalog, device and account administration are manager-only.
	r.Get("/dashboard", s.merchantDashboard)
	r.Get("/stores", s.listMerchantStores)
	r.Get("/orders", s.listOrders)
	r.Get("/orders/{orderID}", s.getOrder)
	r.Post("/orders/{orderID}/status", s.transitionOrder)
	r.Post("/orders/{orderID}/reprint", s.reprintOrder)
	r.Get("/print-jobs", s.listPrintJobs)
	r.Post("/print-jobs/{jobID}/retry", s.retryPrintJob)
	r.Get("/notifications", s.listMerchantNotifications)
	r.Get("/notifications/unread-count", s.merchantNotificationUnreadCount)
	r.Post("/notifications/read-all", s.markAllMerchantNotificationsRead)
	r.Get("/notifications/{notificationID}", s.getMerchantNotification)
	r.Post("/notifications/{notificationID}/read", s.markMerchantNotificationRead)

	r.Group(func(managers chi.Router) {
		managers.Use(requireRoles(RoleMerchantOwner, RoleMerchantManager))
		managers.Get("/settings", s.getMerchantSettings)
		managers.Put("/settings", s.updateMerchantSettings)
		managers.Get("/business-hours", s.getStoreBusinessHours)
		managers.Put("/business-hours", s.updateStoreBusinessHours)
		managers.Put("/business-status", s.updateStoreBusinessOverride)
		managers.Get("/operation-settings", s.getMerchantOperationSettings)
		managers.Put("/operation-settings", s.updateMerchantOperationSettings)
		managers.Get("/payment-settings", s.getMerchantPaymentSettings)
		managers.Get("/table-board", s.getMerchantTableBoard)
		managers.Get("/categories", s.listCategories)
		managers.Post("/categories", s.createCategory)
		managers.Get("/categories/{categoryID}", s.getCategory)
		managers.Put("/categories/{categoryID}", s.updateCategory)
		managers.Delete("/categories/{categoryID}", s.deleteCategory)
		managers.Get("/products", s.listProducts)
		managers.Post("/products", s.createProduct)
		managers.Get("/products/{productID}", s.getProduct)
		managers.Put("/products/{productID}", s.updateProduct)
		managers.Delete("/products/{productID}", s.deleteProduct)
		managers.Post("/products/{productID}/actions", s.performProductAction)
		managers.Get("/products/{productID}/statistics", s.getProductStatistics)
		managers.Get("/products/{productID}/configuration", s.getProductConfiguration)
		managers.Put("/products/{productID}/configuration", s.updateProductConfiguration)
		managers.Put("/skus/{skuID}/inventory", s.updateInventory)
		managers.Get("/catalog-resources", s.listCatalogResources)
		managers.Post("/catalog-resources", s.createCatalogResource)
		managers.Put("/catalog-resources/{resourceID}", s.updateCatalogResource)
		managers.Delete("/catalog-resources/{resourceID}", s.deleteCatalogResource)
		managers.Get("/modifier-items", s.listModifierItems)
		managers.Post("/modifier-items", s.createModifierItem)
		managers.Put("/modifier-items/{itemID}", s.updateModifierItem)
		managers.Delete("/modifier-items/{itemID}", s.deleteModifierItem)
		managers.Get("/modifier-groups", s.listModifierGroups)
		managers.Post("/modifier-groups", s.createModifierGroup)
		managers.Put("/modifier-groups/{groupID}", s.updateModifierGroup)
		managers.Delete("/modifier-groups/{groupID}", s.deleteModifierGroup)
		managers.Get("/decoration", s.getDecoration)
		managers.Put("/decoration/draft", s.saveDecorationDraft)
		managers.Post("/decoration/publish", s.publishDecoration)
		managers.Get("/decoration/versions", s.listDecorationVersions)
		managers.Get("/decoration/versions/{versionID}", s.getDecorationVersion)
		managers.Post("/decoration/versions/{versionID}/rollback", s.rollbackDecoration)
		managers.Get("/decoration/templates", s.getDecorationTemplates)
		managers.Get("/media-groups", s.listMediaGroups)
		managers.Post("/media-groups", s.createMediaGroup)
		managers.Put("/media-groups/{groupID}", s.updateMediaGroup)
		managers.Delete("/media-groups/{groupID}", s.deleteMediaGroup)
		managers.Get("/media-assets", s.listMediaAssets)
		managers.Post("/media-assets", s.createMediaAsset)
		managers.Post("/media-assets/upload", s.uploadMediaAsset)
		managers.Put("/media-assets/{assetID}", s.updateMediaAsset)
		managers.Delete("/media-assets/{assetID}", s.deleteMediaAsset)
		managers.Post("/orders/{orderID}/pay", s.createPayment)
		managers.Post("/refunds", s.createRefund)
		managers.Get("/refunds", s.listRefunds)
		managers.Get("/payments", s.listPayments)
		managers.Get("/printers", s.listPrinters)
		managers.Post("/printers", s.createPrinter)
		managers.Get("/printers/{printerID}", s.getPrinter)
		managers.Put("/printers/{printerID}", s.updatePrinter)
		managers.Delete("/printers/{printerID}", s.deletePrinter)
		managers.Post("/printers/{printerID}/test", s.testPrinter)
		managers.Get("/table-areas", s.listTableAreas)
		managers.Post("/table-areas", s.createTableArea)
		managers.Put("/table-areas/{areaID}", s.updateTableArea)
		managers.Delete("/table-areas/{areaID}", s.deleteTableArea)
		managers.Get("/table-codes", s.listTableCodes)
		managers.Post("/table-codes", s.createTableCode)
		managers.Get("/table-codes/{tableID}", s.getTableCode)
		managers.Put("/table-codes/{tableID}", s.updateTableCode)
		managers.Delete("/table-codes/{tableID}", s.deleteTableCode)
		managers.Get("/fast-food-plates", s.listFastFoodPlates)
		managers.Post("/fast-food-plates", s.createFastFoodPlate)
		managers.Get("/fast-food-plates/{plateID}", s.getFastFoodPlate)
		managers.Put("/fast-food-plates/{plateID}", s.updateFastFoodPlate)
		managers.Delete("/fast-food-plates/{plateID}", s.deleteFastFoodPlate)
		managers.Get("/print-templates", s.listPrintTemplates)
		managers.Post("/print-templates", s.createPrintTemplate)
		managers.Get("/print-templates/{templateID}", s.getPrintTemplate)
		managers.Put("/print-templates/{templateID}", s.updatePrintTemplate)
		managers.Delete("/print-templates/{templateID}", s.deletePrintTemplate)
		managers.Get("/staff", s.listStaff)
		managers.Post("/staff", s.createStaff)
		managers.Get("/staff/{userID}", s.getStaff)
		managers.Put("/staff/{userID}", s.updateStaff)
		managers.Delete("/staff/{userID}", s.deleteStaff)
	})

	// Customer, membership and stored-value administration has its own route
	// group so financial write permissions remain explicit and fail closed.
	s.memberRoutes(r)

	// Marketing applications are mounted as a separate manager-only module so
	// staff order permissions never leak into campaign or coupon administration.
	s.registerMarketingMerchantRoutes(r)
}

func (s *Server) merchantDashboard(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if identity.Role == RoleMerchantStaff {
		var todayOrders, pendingOrders int
		err = s.DB.QueryRowContext(r.Context(), `SELECT
			COALESCE(SUM(DATE(created_at)=CURDATE()),0),
			COALESCE(SUM(status IN ('PAID','ACCEPTED','PREPARING','READY')),0)
			FROM orders WHERE tenant_id=? AND store_id=?`, identity.TenantID, storeID).Scan(&todayOrders, &pendingOrders)
		if err != nil {
			handleSQLError(w, err)
			return
		}
		writeData(w, http.StatusOK, map[string]any{"store_id": storeID, "today_orders": todayOrders, "active_orders": pendingOrders, "financials_visible": false})
		return
	}
	var todayOrders, pendingOrders, paidOrders int
	var todayRevenue, refunded, yesterdayRevenue int64
	err = s.DB.QueryRowContext(r.Context(), `SELECT
		COALESCE(SUM(CASE WHEN DATE(created_at)=CURDATE() THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN DATE(paid_at)=CURDATE() THEN paid_cents ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status IN ('PAID','ACCEPTED','PREPARING','READY') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN DATE(updated_at)=CURDATE() THEN refunded_cents ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN DATE(paid_at)=CURDATE()-INTERVAL 1 DAY THEN paid_cents-refunded_cents ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN DATE(paid_at)=CURDATE() THEN 1 ELSE 0 END),0)
		FROM orders WHERE tenant_id=? AND store_id=?`, identity.TenantID, storeID).Scan(&todayOrders, &todayRevenue, &pendingOrders, &refunded, &yesterdayRevenue, &paidOrders)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	trendRows, err := s.DB.QueryContext(r.Context(), `SELECT DATE_FORMAT(days.day,'%m-%d'),COALESCE(SUM(o.paid_cents-o.refunded_cents),0) FROM (
		SELECT CURDATE() day UNION ALL SELECT CURDATE()-INTERVAL 1 DAY UNION ALL SELECT CURDATE()-INTERVAL 2 DAY UNION ALL SELECT CURDATE()-INTERVAL 3 DAY UNION ALL SELECT CURDATE()-INTERVAL 4 DAY UNION ALL SELECT CURDATE()-INTERVAL 5 DAY UNION ALL SELECT CURDATE()-INTERVAL 6 DAY
	) days LEFT JOIN orders o ON DATE(o.paid_at)=days.day AND o.tenant_id=? AND o.store_id=? GROUP BY days.day ORDER BY days.day`, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	revenueTrend := []map[string]any{}
	for trendRows.Next() {
		var label string
		var cents int64
		if err = trendRows.Scan(&label, &cents); err != nil {
			trendRows.Close()
			handleSQLError(w, err)
			return
		}
		revenueTrend = append(revenueTrend, map[string]any{"label": label, "value": float64(cents) / 100})
	}
	trendRows.Close()
	popularRows, err := s.DB.QueryContext(r.Context(), `SELECT oi.product_name,SUM(oi.quantity) FROM order_items oi JOIN orders o ON o.id=oi.order_id
		WHERE o.tenant_id=? AND o.store_id=? AND o.paid_at>=DATE_SUB(CURDATE(),INTERVAL 6 DAY)
		GROUP BY oi.product_name ORDER BY SUM(oi.quantity) DESC LIMIT 5`, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	popularProducts := []map[string]any{}
	for popularRows.Next() {
		var name string
		var count int
		if err = popularRows.Scan(&name, &count); err != nil {
			popularRows.Close()
			handleSQLError(w, err)
			return
		}
		popularProducts = append(popularProducts, map[string]any{"name": name, "count": count})
	}
	popularRows.Close()
	recentRows, err := s.DB.QueryContext(r.Context(), `SELECT id,order_no,total_cents,status,order_type,pickup_code,fast_food_plate_name_snapshot,fast_food_plate_code_snapshot,table_area_name_snapshot,table_name_snapshot,table_code_snapshot,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM orders WHERE tenant_id=? AND store_id=? ORDER BY id DESC LIMIT 5`, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	recentOrders := []map[string]any{}
	for recentRows.Next() {
		var orderID, totalCents int64
		var orderNo, status, orderType, pickupCode, plateName, plateCode, tableArea, tableName, tableCode, createdAt string
		if err = recentRows.Scan(&orderID, &orderNo, &totalCents, &status, &orderType, &pickupCode, &plateName, &plateCode, &tableArea, &tableName, &tableCode, &createdAt); err != nil {
			recentRows.Close()
			handleSQLError(w, err)
			return
		}
		row := map[string]any{"id": orderID, "pickupNo": pickupCode, "pickupCode": pickupCode, "orderNo": orderNo, "orderType": orderType, "order_type": orderType, "amount": float64(totalCents) / 100, "status": status, "createdAt": createdAt, "items": []any{}}
		if orderType == orderTypeDineIn {
			row["table"] = map[string]any{"areaName": tableArea, "name": tableName, "tableCode": tableCode}
		} else if plateCode != "" {
			row["fastFoodPlate"] = map[string]any{"plateName": plateName, "plateCode": plateCode}
		}
		recentOrders = append(recentOrders, row)
	}
	recentRows.Close()
	averageOrderValue := 0.0
	if paidOrders > 0 {
		averageOrderValue = float64(todayRevenue) / 100 / float64(paidOrders)
	}
	writeData(w, http.StatusOK, map[string]any{"store_id": storeID, "today_orders": todayOrders, "today_revenue_cents": todayRevenue, "active_orders": pendingOrders, "today_refunded_cents": refunded, "yesterdayRevenue": float64(yesterdayRevenue) / 100, "averageOrderValue": averageOrderValue, "revenueTrend": revenueTrend, "popularProducts": popularProducts, "recentOrders": recentOrders, "financials_visible": true})
}

func (s *Server) tenantStoreID(r *http.Request, tenantID int64) (int64, error) {
	if raw := r.URL.Query().Get("store_id"); raw != "" {
		id, err := parseInt64(raw)
		if err != nil {
			return 0, err
		}
		var found int64
		err = s.DB.QueryRowContext(r.Context(), "SELECT id FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, tenantID).Scan(&found)
		return found, err
	}
	var id int64
	err := s.DB.QueryRowContext(r.Context(), "SELECT id FROM stores WHERE tenant_id=? AND deleted_at IS NULL ORDER BY id LIMIT 1", tenantID).Scan(&id)
	return id, err
}

func (s *Server) listMerchantStores(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("tenantID", int64String(identity.TenantID))
	s.listPlatformStores(w, r.WithContext(contextWithRoute(r.Context(), ctx)))
}

func contextWithRoute(ctx context.Context, route *chi.Context) context.Context {
	return context.WithValue(ctx, chi.RouteCtxKey, route)
}

func (s *Server) getMerchantSettings(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var storeCode, storeName, logo, phone, address, announcement, businessHours, trigger string
	var businessLicenseURL, foodBusinessLicenseURL string
	var autoAccept, voice, printReceipt, printLabel, pickup, latePayment bool
	var timeout int
	err = s.DB.QueryRowContext(r.Context(), `SELECT s.code,s.name,s.logo_url,s.phone,s.address,s.notice,s.business_hours,s.auto_accept_orders,s.voice_reminder,s.default_print_trigger,s.auto_print_receipt,s.auto_print_label,s.pickup_mode,s.allow_late_payment,s.payment_timeout_minutes,
		COALESCE((SELECT a.url FROM tenants t JOIN media_assets a ON a.id=t.business_license_media_id AND a.tenant_id=t.id AND a.kind='TENANT_DOCUMENT' AND a.status='ACTIVE' AND a.deleted_at IS NULL WHERE t.id=s.tenant_id AND t.deleted_at IS NULL),''),
		COALESCE((SELECT a.url FROM tenants t JOIN media_assets a ON a.id=t.food_business_license_media_id AND a.tenant_id=t.id AND a.kind='TENANT_DOCUMENT' AND a.status='ACTIVE' AND a.deleted_at IS NULL WHERE t.id=s.tenant_id AND t.deleted_at IS NULL),'')
		FROM stores s WHERE s.id=? AND s.tenant_id=? AND s.deleted_at IS NULL`, storeID, identity.TenantID).
		Scan(&storeCode, &storeName, &logo, &phone, &address, &announcement, &businessHours, &autoAccept, &voice, &trigger, &printReceipt, &printLabel, &pickup, &latePayment, &timeout, &businessLicenseURL, &foodBusinessLicenseURL)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	hours := strings.Split(businessHours, "-")
	if len(hours) != 2 {
		hours = []string{}
	}
	writeData(w, http.StatusOK, map[string]any{"storeId": storeID, "storeCode": storeCode, "storeName": storeName, "logo": logo, "phone": phone, "address": address, "announcement": announcement, "businessHours": hours, "autoAcceptOrder": autoAccept, "orderVoiceReminder": voice, "printTrigger": trigger, "autoPrintReceipt": printReceipt, "autoPrintLabel": printLabel, "pickupMode": pickup, "allowLatePayment": latePayment, "paymentTimeoutMinutes": timeout, "businessLicenseUrl": businessLicenseURL, "foodBusinessLicenseUrl": foodBusinessLicenseURL})
}

type merchantSettingsInput struct {
	StoreID               int64    `json:"storeId"`
	StoreName             string   `json:"storeName"`
	Logo                  string   `json:"logo"`
	Address               string   `json:"address"`
	Phone                 string   `json:"phone"`
	Announcement          string   `json:"announcement"`
	BusinessHours         []string `json:"businessHours"`
	AutoAcceptOrder       *bool    `json:"autoAcceptOrder"`
	OrderVoiceReminder    *bool    `json:"orderVoiceReminder"`
	PrintTrigger          string   `json:"printTrigger"`
	AutoPrintReceipt      *bool    `json:"autoPrintReceipt"`
	AutoPrintLabel        *bool    `json:"autoPrintLabel"`
	PickupMode            *bool    `json:"pickupMode"`
	AllowLatePayment      *bool    `json:"allowLatePayment"`
	PaymentTimeoutMinutes *int     `json:"paymentTimeoutMinutes"`
	LegacyStoreID         int64    `json:"store_id"`
	LegacyStoreName       string   `json:"store_name"`
	LegacyLogo            string   `json:"logo_url"`
	LegacyBanner          string   `json:"banner_url"`
	LegacyBusinessHours   string   `json:"business_hours"`
	LegacyNotice          string   `json:"notice"`
	LegacyTenantName      string   `json:"name"`
	LegacyContactName     string   `json:"contact_name"`
	LegacyContactPhone    string   `json:"contact_phone"`
}

func (s *Server) updateMerchantSettings(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	if identity.Role == RoleMerchantStaff {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "manager role is required")
		return
	}
	var input merchantSettingsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	storeID := input.StoreID
	if storeID == 0 {
		storeID = input.LegacyStoreID
	}
	if storeID == 0 {
		var err error
		storeID, err = s.tenantStoreID(r, identity.TenantID)
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var currentName, currentLogo, currentAddress, currentPhone, currentHours, currentNotice, currentTrigger string
	var currentAutoAccept, currentVoice, currentReceipt, currentLabel, currentPickup, currentLate bool
	var currentTimeout int
	if err = tx.QueryRowContext(r.Context(), `SELECT name,logo_url,address,phone,business_hours,notice,auto_accept_orders,voice_reminder,default_print_trigger,auto_print_receipt,auto_print_label,pickup_mode,allow_late_payment,payment_timeout_minutes FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE`, storeID, identity.TenantID).
		Scan(&currentName, &currentLogo, &currentAddress, &currentPhone, &currentHours, &currentNotice, &currentAutoAccept, &currentVoice, &currentTrigger, &currentReceipt, &currentLabel, &currentPickup, &currentLate, &currentTimeout); err != nil {
		handleSQLError(w, err)
		return
	}
	legacy := input.LegacyStoreID != 0 || input.LegacyStoreName != "" || input.LegacyBusinessHours != "" || input.LegacyTenantName != ""
	if legacy {
		currentName, currentLogo, currentAddress, currentPhone, currentHours, currentNotice = input.LegacyStoreName, input.LegacyLogo, input.Address, input.Phone, input.LegacyBusinessHours, input.LegacyNotice
	} else {
		currentName, currentLogo, currentAddress, currentPhone, currentNotice = input.StoreName, input.Logo, input.Address, input.Phone, input.Announcement
		if len(input.BusinessHours) == 2 {
			currentHours = strings.Join(input.BusinessHours, "-")
		}
	}
	if input.AutoAcceptOrder != nil {
		currentAutoAccept = *input.AutoAcceptOrder
	}
	if input.OrderVoiceReminder != nil {
		currentVoice = *input.OrderVoiceReminder
	}
	if input.AutoPrintReceipt != nil {
		currentReceipt = *input.AutoPrintReceipt
	}
	if input.AutoPrintLabel != nil {
		currentLabel = *input.AutoPrintLabel
	}
	if input.PickupMode != nil {
		currentPickup = *input.PickupMode
	}
	if input.AllowLatePayment != nil {
		currentLate = *input.AllowLatePayment
	}
	if input.PaymentTimeoutMinutes != nil {
		currentTimeout = *input.PaymentTimeoutMinutes
	}
	if input.PrintTrigger != "" {
		currentTrigger = input.PrintTrigger
	}
	if currentTrigger != "ORDER_CREATED" && currentTrigger != "PAYMENT_SUCCESS" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "printTrigger must be ORDER_CREATED or PAYMENT_SUCCESS")
		return
	}
	if currentTimeout < 1 || currentTimeout > 1440 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "paymentTimeoutMinutes must be between 1 and 1440")
		return
	}
	if err = s.validateManagedMediaURL(r.Context(), tx, identity.TenantID, storeID, currentLogo); err != nil {
		writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE stores SET name=?,logo_url=?,address=?,phone=?,business_hours=?,notice=?,auto_accept_orders=?,voice_reminder=?,default_print_trigger=?,auto_print_receipt=?,auto_print_label=?,pickup_mode=?,allow_late_payment=?,payment_timeout_minutes=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, currentName, currentLogo, currentAddress, currentPhone, currentHours, currentNotice, currentAutoAccept, currentVoice, currentTrigger, currentReceipt, currentLabel, currentPickup, currentLate, currentTimeout, storeID, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if commitErr := tx.Commit(); commitErr != nil {
		handleSQLError(w, commitErr)
		return
	}
	s.audit(r.Context(), identity, "merchant.settings.update", "store", int64String(storeID), input, r)
	s.getMerchantSettings(w, r)
}

type categoryDTO struct {
	ID              int64  `json:"id"`
	StoreID         int64  `json:"store_id"`
	Name            string `json:"name"`
	SortOrder       int    `json:"sort_order"`
	InStoreEnabled  bool   `json:"in_store_enabled"`
	DeliveryEnabled bool   `json:"delivery_enabled"`
	Status          string `json:"status"`
}

type categoryInput struct {
	StoreID         int64  `json:"store_id"`
	Name            string `json:"name"`
	SortOrder       int    `json:"sort_order"`
	InStoreEnabled  *bool  `json:"in_store_enabled"`
	DeliveryEnabled *bool  `json:"delivery_enabled"`
	Status          string `json:"status"`
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT id,store_id,name,sort_order,in_store_enabled,delivery_enabled,status FROM categories WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY sort_order,id", identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []categoryDTO{}
	for rows.Next() {
		var item categoryDTO
		if err := rows.Scan(&item.ID, &item.StoreID, &item.Name, &item.SortOrder, &item.InStoreEnabled, &item.DeliveryEnabled, &item.Status); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input categoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return
	}
	storeID := input.StoreID
	if storeID == 0 {
		var err error
		storeID, err = s.tenantStoreID(r, identity.TenantID)
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	inStoreEnabled, deliveryEnabled, channelErr := catalogChannelFlags(input.InStoreEnabled, input.DeliveryEnabled, true, false)
	if channelErr != nil {
		writeError(w, http.StatusConflict, "DELIVERY_NOT_AVAILABLE", channelErr.Error())
		return
	}
	input.Status = strings.ToUpper(input.Status)
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE or DISABLED")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "INSERT INTO categories(tenant_id,store_id,name,sort_order,in_store_enabled,delivery_enabled,status) SELECT ?,id,?,?,?,?,? FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL", identity.TenantID, input.Name, input.SortOrder, inStoreEnabled, deliveryEnabled, strings.ToUpper(input.Status), storeID, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "category.create", "category", int64String(id), input, r)
	s.getCategoryByID(w, r, identity.TenantID, id)
}

func (s *Server) getCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "categoryID")
	if ok {
		s.getCategoryByID(w, r, currentIdentity(r.Context()).TenantID, id)
	}
}

func (s *Server) getCategoryByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var item categoryDTO
	err := s.DB.QueryRowContext(r.Context(), "SELECT id,store_id,name,sort_order,in_store_enabled,delivery_enabled,status FROM categories WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, tenantID).Scan(&item.ID, &item.StoreID, &item.Name, &item.SortOrder, &item.InStoreEnabled, &item.DeliveryEnabled, &item.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "categoryID")
	if !ok {
		return
	}
	var input categoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	identity := currentIdentity(r.Context())
	var currentInStore, currentDelivery bool
	if err := s.DB.QueryRowContext(r.Context(), "SELECT in_store_enabled,delivery_enabled FROM categories WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID).Scan(&currentInStore, &currentDelivery); err != nil {
		handleSQLError(w, err)
		return
	}
	inStoreEnabled, deliveryEnabled, channelErr := catalogChannelFlags(input.InStoreEnabled, input.DeliveryEnabled, currentInStore, currentDelivery)
	if channelErr != nil {
		writeError(w, http.StatusConflict, "DELIVERY_NOT_AVAILABLE", channelErr.Error())
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE categories SET name=?,sort_order=?,in_store_enabled=?,delivery_enabled=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.Name, input.SortOrder, inStoreEnabled, deliveryEnabled, strings.ToUpper(input.Status), id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "category not found")
		return
	}
	s.audit(r.Context(), identity, "category.update", "category", int64String(id), input, r)
	s.getCategoryByID(w, r, identity.TenantID, id)
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "categoryID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var productCount int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM products WHERE category_id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID).Scan(&productCount); err != nil {
		handleSQLError(w, err)
		return
	}
	if productCount > 0 {
		writeError(w, http.StatusConflict, "CATEGORY_IN_USE", "move or delete products in this category first")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE categories SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "category not found")
		return
	}
	s.audit(r.Context(), identity, "category.delete", "category", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) listStaff(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,tenant_id,username,display_name,role,status,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ') FROM users WHERE tenant_id=? AND deleted_at IS NULL ORDER BY id DESC`, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []userDTO{}
	for rows.Next() {
		var item userDTO
		if err := scanUser(rows, &item); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createStaff(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input userInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.TenantID = identity.TenantID
	input.Role = strings.ToUpper(input.Role)
	if input.Role == "" {
		input.Role = RoleMerchantStaff
	}
	if identity.Role == RoleMerchantManager && input.Role != RoleMerchantStaff {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "merchant managers can only create staff accounts")
		return
	}
	if input.Username == "" || len(input.Password) < 8 || !validRole(input.Role, input.TenantID) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "username and password (at least 8 characters) are required")
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	input.Status = strings.ToUpper(input.Status)
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE or DISABLED")
		return
	}
	hash, hashErr := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if hashErr != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PASSWORD", "password must not exceed 72 bytes")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "INSERT INTO users(tenant_id,username,password_hash,display_name,role,status) VALUES(?,?,?,?,?,?)", identity.TenantID, input.Username, string(hash), input.DisplayName, input.Role, input.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "staff.create", "user", int64String(id), map[string]any{"username": input.Username, "role": input.Role}, r)
	s.getUserByScope(w, r, identity.TenantID, id, false)
}

func (s *Server) getStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "userID")
	if ok {
		identity := currentIdentity(r.Context())
		s.getUserByScope(w, r, identity.TenantID, id, false)
	}
}

func (s *Server) updateStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var input userInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Role = strings.ToUpper(input.Role)
	if input.Role == "" {
		input.Role = RoleMerchantStaff
	}
	if !validRole(input.Role, identity.TenantID) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid merchant role")
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	input.Status = strings.ToUpper(input.Status)
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE or DISABLED")
		return
	}
	var passwordHash string
	if input.Password != "" {
		if len(input.Password) < 8 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "password must contain at least 8 characters")
			return
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PASSWORD", "password must not exceed 72 bytes")
			return
		}
		passwordHash = string(hash)
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var targetRole, targetStatus string
	if err = tx.QueryRowContext(r.Context(), "SELECT role,status FROM users WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", id, identity.TenantID).Scan(&targetRole, &targetStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if identity.Role == RoleMerchantManager && (targetRole != RoleMerchantStaff || input.Role != RoleMerchantStaff) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "merchant managers can only update staff accounts")
		return
	}
	if identity.UserID == id && (input.Role != targetRole || input.Status != targetStatus) {
		writeError(w, http.StatusConflict, "CANNOT_CHANGE_OWN_ACCESS", "current merchant user cannot change its own role or status")
		return
	}
	if targetRole == RoleMerchantOwner && targetStatus == "ACTIVE" && (input.Role != RoleMerchantOwner || input.Status != "ACTIVE") {
		rows, countErr := tx.QueryContext(r.Context(), "SELECT id FROM users WHERE tenant_id=? AND role=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE", identity.TenantID, RoleMerchantOwner)
		if countErr != nil {
			handleSQLError(w, countErr)
			return
		}
		activeOwners := 0
		for rows.Next() {
			activeOwners++
		}
		rows.Close()
		if activeOwners <= 1 {
			writeError(w, http.StatusConflict, "LAST_MERCHANT_OWNER", "at least one active merchant owner is required")
			return
		}
	}
	var result sql.Result
	if passwordHash != "" {
		result, err = tx.ExecContext(r.Context(), "UPDATE users SET username=?,password_hash=?,display_name=?,role=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.Username, passwordHash, input.DisplayName, input.Role, input.Status, id, identity.TenantID)
	} else {
		result, err = tx.ExecContext(r.Context(), "UPDATE users SET username=?,display_name=?,role=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.Username, input.DisplayName, input.Role, input.Status, id, identity.TenantID)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "staff not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "staff.update", "user", int64String(id), map[string]any{"username": input.Username, "role": input.Role}, r)
	s.getUserByScope(w, r, identity.TenantID, id, false)
}

func (s *Server) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	if identity.UserID == id {
		writeError(w, http.StatusConflict, "CANNOT_DELETE_SELF", "current user cannot delete itself")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var targetRole, targetStatus string
	if err = tx.QueryRowContext(r.Context(), "SELECT role,status FROM users WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", id, identity.TenantID).Scan(&targetRole, &targetStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if identity.Role == RoleMerchantManager && targetRole != RoleMerchantStaff {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "merchant managers can only delete staff accounts")
		return
	}
	if targetRole == RoleMerchantOwner && targetStatus == "ACTIVE" {
		rows, countErr := tx.QueryContext(r.Context(), "SELECT id FROM users WHERE tenant_id=? AND role=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE", identity.TenantID, RoleMerchantOwner)
		if countErr != nil {
			handleSQLError(w, countErr)
			return
		}
		activeOwners := 0
		for rows.Next() {
			activeOwners++
		}
		rows.Close()
		if activeOwners <= 1 {
			writeError(w, http.StatusConflict, "LAST_MERCHANT_OWNER", "at least one active merchant owner is required")
			return
		}
	}
	result, err := tx.ExecContext(r.Context(), "UPDATE users SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "staff not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "staff.delete", "user", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

var _ = sql.ErrNoRows
