package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
)

type storeOperationSettings struct {
	StoreID                      int64    `json:"storeId"`
	SettlementMode               string   `json:"settlementMode"`
	OrderingMode                 string   `json:"orderingMode"`
	DistanceCheckEnabled         bool     `json:"distanceCheckEnabled"`
	DistanceLimitM               int      `json:"distanceLimitM"`
	StoreLatitude                *float64 `json:"storeLatitude"`
	StoreLongitude               *float64 `json:"storeLongitude"`
	RequireCustomerPhone         bool     `json:"requireCustomerPhone"`
	AllowOrderRemark             bool     `json:"allowOrderRemark"`
	AllowItemRemark              bool     `json:"allowItemRemark"`
	OrderReminderEnabled         bool     `json:"orderReminderEnabled"`
	OrderReminderIntervalMinutes int      `json:"orderReminderIntervalMinutes"`
	TakeawayVerificationEnabled  bool     `json:"takeawayVerificationEnabled"`
	ReviewsEnabled               bool     `json:"reviewsEnabled"`
	CustomerServicePhone         string   `json:"customerServicePhone"`
	CustomerServiceWechat        string   `json:"customerServiceWechat"`
	CustomerServiceQRURL         string   `json:"customerServiceQrUrl"`
	PrivacyPolicyText            string   `json:"privacyPolicyText"`
	UserAgreementText            string   `json:"userAgreementText"`
	OfficialAccountNotifyEnabled bool     `json:"officialAccountNotifyEnabled"`
	OfficialAccountEvents        []string `json:"officialAccountEvents"`
	NotificationRecipientLabel   string   `json:"notificationRecipientLabel"`
}

func (s *Server) loadStoreOperationSettings(r *http.Request, tenantID, storeID int64) (storeOperationSettings, error) {
	if _, err := s.DB.ExecContext(r.Context(), `INSERT IGNORE INTO store_operation_settings(
		store_id,tenant_id,settlement_mode,ordering_mode,privacy_policy_text,user_agreement_text,official_account_events_json
	) VALUES(?,?,'PAY_BEFORE','MULTI_PERSON','','','["ORDER_PAID","REFUND_CREATED","PRINT_FAILED"]')`, storeID, tenantID); err != nil {
		return storeOperationSettings{}, err
	}
	var item storeOperationSettings
	var latitude, longitude sql.NullFloat64
	var eventsJSON string
	err := s.DB.QueryRowContext(r.Context(), `SELECT store_id,settlement_mode,ordering_mode,distance_check_enabled,distance_limit_m,
		store_latitude,store_longitude,require_customer_phone,allow_order_remark,allow_item_remark,order_reminder_enabled,
		order_reminder_interval_minutes,takeaway_verification_enabled,reviews_enabled,customer_service_phone,
		customer_service_wechat,customer_service_qr_url,privacy_policy_text,user_agreement_text,official_account_notify_enabled,
		official_account_events_json,notification_recipient_label
		FROM store_operation_settings WHERE tenant_id=? AND store_id=?`, tenantID, storeID).
		Scan(&item.StoreID, &item.SettlementMode, &item.OrderingMode, &item.DistanceCheckEnabled, &item.DistanceLimitM,
			&latitude, &longitude, &item.RequireCustomerPhone, &item.AllowOrderRemark, &item.AllowItemRemark,
			&item.OrderReminderEnabled, &item.OrderReminderIntervalMinutes, &item.TakeawayVerificationEnabled,
			&item.ReviewsEnabled, &item.CustomerServicePhone, &item.CustomerServiceWechat, &item.CustomerServiceQRURL,
			&item.PrivacyPolicyText, &item.UserAgreementText, &item.OfficialAccountNotifyEnabled, &eventsJSON,
			&item.NotificationRecipientLabel)
	if err != nil {
		return item, err
	}
	if latitude.Valid {
		value := latitude.Float64
		item.StoreLatitude = &value
	}
	if longitude.Valid {
		value := longitude.Float64
		item.StoreLongitude = &value
	}
	if json.Unmarshal([]byte(eventsJSON), &item.OfficialAccountEvents) != nil {
		item.OfficialAccountEvents = []string{}
	}
	return item, nil
}

func validCoordinate(latitude, longitude *float64) bool {
	return latitude != nil && longitude != nil && *latitude >= -90 && *latitude <= 90 && *longitude >= -180 && *longitude <= 180
}

func validateOperationSettings(input storeOperationSettings) error {
	input.SettlementMode = strings.ToUpper(strings.TrimSpace(input.SettlementMode))
	input.OrderingMode = strings.ToUpper(strings.TrimSpace(input.OrderingMode))
	if !validStatus(input.SettlementMode, "PAY_BEFORE", "PAY_AFTER") {
		return errors.New("settlementMode must be PAY_BEFORE or PAY_AFTER")
	}
	if !validStatus(input.OrderingMode, "SINGLE_PERSON", "MULTI_PERSON") {
		return errors.New("orderingMode must be SINGLE_PERSON or MULTI_PERSON")
	}
	if input.DistanceLimitM < 100 || input.DistanceLimitM > 100000 {
		return errors.New("distanceLimitM must be between 100 and 100000")
	}
	if input.DistanceCheckEnabled && !validCoordinate(input.StoreLatitude, input.StoreLongitude) {
		return errors.New("store coordinates are required when distance validation is enabled")
	}
	if input.OrderReminderIntervalMinutes < 1 || input.OrderReminderIntervalMinutes > 120 {
		return errors.New("orderReminderIntervalMinutes must be between 1 and 120")
	}
	if len([]rune(input.CustomerServicePhone)) > 32 || len([]rune(input.CustomerServiceWechat)) > 80 || len(input.CustomerServiceQRURL) > 1024 {
		return errors.New("customer service contact is too long")
	}
	if len([]rune(input.PrivacyPolicyText)) > 20000 || len([]rune(input.UserAgreementText)) > 20000 {
		return errors.New("policy text must not exceed 20000 characters")
	}
	if len([]rune(input.NotificationRecipientLabel)) > 120 {
		return errors.New("notification recipient label is too long")
	}
	allowedEvents := map[string]bool{"ORDER_PAID": true, "REFUND_CREATED": true, "PRINT_FAILED": true, "STORE_EXCEPTION": true}
	seen := map[string]bool{}
	for _, event := range input.OfficialAccountEvents {
		if !allowedEvents[event] || seen[event] {
			return errors.New("officialAccountEvents contains an unsupported or duplicate event")
		}
		seen[event] = true
	}
	return nil
}

func applyOperationSettingsDefaults(input *storeOperationSettings) {
	if !input.DistanceCheckEnabled && input.DistanceLimitM == 0 {
		input.DistanceLimitM = 5000
	}
}

func (s *Server) getMerchantOperationSettings(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	item, err := s.loadStoreOperationSettings(r, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"settings": item,
		"officialAccount": map[string]any{
			"platformConfigured":     s.Config.WeChatOfficialAccount.AppID != "" && s.Config.WeChatOfficialAccount.AppSecret != "",
			"merchantRecipientBound": false,
			"deliveryActive":         false,
		},
		"safetyPolicies": map[string]any{
			"cancelledPaidOrderQuarantined": true,
			"duplicatePaymentQuarantined":   true,
			"stockDeductTiming":             "ORDER_CREATED_RESERVE_PAYMENT_SUCCESS_CONFIRM",
		},
		"activeCapabilities":   []string{"PAY_AFTER_MEAL"},
		"reservedCapabilities": []string{"UNACCEPTED_TIMEOUT_REFUND", "CUSTOMER_REVIEW", "TAKEAWAY_VERIFICATION"},
	})
}

func (s *Server) updateMerchantOperationSettings(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input storeOperationSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	input.StoreID = storeID
	applyOperationSettingsDefaults(&input)
	input.SettlementMode = strings.ToUpper(strings.TrimSpace(input.SettlementMode))
	input.OrderingMode = strings.ToUpper(strings.TrimSpace(input.OrderingMode))
	if err = validateOperationSettings(input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	eventsJSON, _ := json.Marshal(input.OfficialAccountEvents)
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var currentSettlementMode string
	if err = tx.QueryRowContext(r.Context(), `SELECT settlement_mode FROM store_operation_settings
		WHERE tenant_id=? AND store_id=? FOR UPDATE`, actor.TenantID, storeID).Scan(&currentSettlementMode); err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	if currentSettlementMode != "" && currentSettlementMode != input.SettlementMode {
		var openBills int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM orders
			WHERE tenant_id=? AND store_id=? AND order_type='DINE_IN' AND payment_status='UNPAID'
			  AND status IN ('PAID','ACCEPTED','PREPARING','READY')`, actor.TenantID, storeID).Scan(&openBills); err != nil {
			handleSQLError(w, err)
			return
		}
		if openBills > 0 {
			writeError(w, http.StatusConflict, "OPEN_TABLE_BILLS_EXIST", "请先结清所有桌台未结账订单，再切换堂食结算模式")
			return
		}
	}
	if err = s.validateManagedMediaURL(r.Context(), tx, actor.TenantID, storeID, input.CustomerServiceQRURL); err != nil {
		writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO store_operation_settings(
		store_id,tenant_id,settlement_mode,ordering_mode,distance_check_enabled,distance_limit_m,store_latitude,store_longitude,
		require_customer_phone,allow_order_remark,allow_item_remark,order_reminder_enabled,order_reminder_interval_minutes,
		takeaway_verification_enabled,reviews_enabled,customer_service_phone,customer_service_wechat,customer_service_qr_url,
		privacy_policy_text,user_agreement_text,official_account_notify_enabled,official_account_events_json,notification_recipient_label
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON DUPLICATE KEY UPDATE settlement_mode=VALUES(settlement_mode),ordering_mode=VALUES(ordering_mode),
		distance_check_enabled=VALUES(distance_check_enabled),distance_limit_m=VALUES(distance_limit_m),store_latitude=VALUES(store_latitude),
		store_longitude=VALUES(store_longitude),require_customer_phone=VALUES(require_customer_phone),allow_order_remark=VALUES(allow_order_remark),
		allow_item_remark=VALUES(allow_item_remark),order_reminder_enabled=VALUES(order_reminder_enabled),
		order_reminder_interval_minutes=VALUES(order_reminder_interval_minutes),takeaway_verification_enabled=VALUES(takeaway_verification_enabled),
		reviews_enabled=VALUES(reviews_enabled),customer_service_phone=VALUES(customer_service_phone),customer_service_wechat=VALUES(customer_service_wechat),
		customer_service_qr_url=VALUES(customer_service_qr_url),privacy_policy_text=VALUES(privacy_policy_text),user_agreement_text=VALUES(user_agreement_text),
		official_account_notify_enabled=VALUES(official_account_notify_enabled),official_account_events_json=VALUES(official_account_events_json),
		notification_recipient_label=VALUES(notification_recipient_label)`,
		storeID, actor.TenantID, input.SettlementMode, input.OrderingMode, input.DistanceCheckEnabled, input.DistanceLimitM,
		nullableFloat64(input.StoreLatitude), nullableFloat64(input.StoreLongitude), input.RequireCustomerPhone, input.AllowOrderRemark,
		input.AllowItemRemark, input.OrderReminderEnabled, input.OrderReminderIntervalMinutes, input.TakeawayVerificationEnabled,
		input.ReviewsEnabled, strings.TrimSpace(input.CustomerServicePhone), strings.TrimSpace(input.CustomerServiceWechat),
		strings.TrimSpace(input.CustomerServiceQRURL), input.PrivacyPolicyText, input.UserAgreementText, input.OfficialAccountNotifyEnabled,
		string(eventsJSON), strings.TrimSpace(input.NotificationRecipientLabel))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	printTrigger := settlementPrintTrigger(input.SettlementMode)
	if _, err = tx.ExecContext(r.Context(), `UPDATE stores SET default_print_trigger=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, printTrigger, storeID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE print_templates SET trigger_event=?,updated_at=NOW(3)
		WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL`, printTrigger, actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE printer_devices SET print_trigger=?,updated_at=NOW(3)
		WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL`, printTrigger, actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "merchant.operation_settings.update", "store", int64String(storeID), input, r)
	s.getMerchantOperationSettings(w, r)
}

func settlementPrintTrigger(settlementMode string) string {
	if strings.EqualFold(strings.TrimSpace(settlementMode), "PAY_AFTER") {
		return "ORDER_CREATED"
	}
	return "PAYMENT_SUCCESS"
}

func nullableFloat64(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func maskedMerchantNo(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 6 {
		if value == "" {
			return ""
		}
		return "***"
	}
	return value[:3] + strings.Repeat("*", len(value)-6) + value[len(value)-3:]
}

func (s *Server) getMerchantPaymentSettings(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var providerName, merchantNo, subAppID, settlementCycle, onboardingStatus, productAuthorizationStatus string
	var feeBPS int
	var refundAuthorized bool
	err := s.DB.QueryRowContext(r.Context(), `SELECT payment_provider,payment_merchant_no,payment_sub_appid,payment_fee_bps,payment_settlement_cycle,
		payment_onboarding_status,payment_product_authorization_status,payment_refund_authorized
		FROM tenants WHERE id=? AND status='ACTIVE' AND deleted_at IS NULL`, actor.TenantID).
		Scan(&providerName, &merchantNo, &subAppID, &feeBPS, &settlementCycle, &onboardingStatus, &productAuthorizationStatus, &refundAuthorized)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	presentation := describePaymentProvider(providerName)
	bindingStatus := "DEVELOPMENT"
	if providerName == "tianque" {
		bindingStatus = "PENDING_BINDING"
		if merchantNo != "" {
			bindingStatus = "BOUND"
		}
	} else if providerName == "wechat_partner" {
		bindingStatus = onboardingStatus
	}
	acceptanceEnabled, acceptanceErr := s.paymentAcceptanceEnabled(r.Context())
	if acceptanceErr != nil {
		handleSQLError(w, acceptanceErr)
		return
	}
	effectiveProvider := s.Payment.Name()
	providerActive := acceptanceEnabled && providerName == effectiveProvider
	onboardingReady := providerName == "mock" || (onboardingStatus == "ACTIVE" && productAuthorizationStatus == "AUTHORIZED" && merchantNo != "")
	writeData(w, http.StatusOK, map[string]any{
		"provider": providerName, "providerDisplayName": presentation.DisplayName, "bindingStatus": bindingStatus,
		"merchantNoMasked": maskedMerchantNo(merchantNo), "subAppIdConfigured": subAppID != "",
		"sharedServiceProviderApp": providerName == "wechat_partner" && subAppID == "",
		"onboardingStatus":         onboardingStatus, "productAuthorizationStatus": productAuthorizationStatus,
		"refundAuthorized": refundAuthorized, "onboardingReady": onboardingReady, "adapterImplemented": presentation.AdapterImplemented,
		"acceptanceEnabled": acceptanceEnabled, "effectiveProvider": effectiveProvider, "providerActive": providerActive,
		"feeRatePercent": float64(feeBPS) / 100, "settlementCycle": settlementCycle,
		"checkoutMode":          presentation.CheckoutMode,
		"fundsFlow":             "ACQUIRER_TO_MERCHANT_SETTLEMENT_ACCOUNT",
		"platformReceivesFunds": false, "confirmationMode": "PROVIDER_CALLBACK_WITH_ACTIVE_QUERY_RECONCILIATION",
		"supportsPartialRefund": providerName != "wechat_partner" || refundAuthorized, "sensitiveConfigurationManagedByPlatform": true,
	})
}

type tableBoardTable struct {
	ID           int64  `json:"id"`
	AreaID       int64  `json:"areaId"`
	AreaName     string `json:"areaName"`
	Name         string `json:"name"`
	TableCode    string `json:"tableCode"`
	Capacity     int    `json:"capacity"`
	State        string `json:"state"`
	OrderID      int64  `json:"orderId,omitempty"`
	OrderNo      string `json:"orderNo,omitempty"`
	OrderStatus  string `json:"orderStatus,omitempty"`
	CustomerName string `json:"customerName,omitempty"`
	TotalCents   int64  `json:"totalCents,omitempty"`
	OpenedAt     string `json:"openedAt,omitempty"`
}

func (s *Server) getMerchantTableBoard(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT t.id,t.area_id,a.name,t.name,t.table_code,t.capacity,
		COALESCE(o.id,0),COALESCE(o.order_no,''),COALESCE(o.status,''),COALESCE(o.customer_name,''),COALESCE(o.total_cents,0),
		COALESCE(DATE_FORMAT(o.created_at,'%Y-%m-%d %H:%i:%s'),'')
		FROM table_codes t JOIN table_areas a ON a.id=t.area_id AND a.tenant_id=t.tenant_id AND a.store_id=t.store_id
		LEFT JOIN orders o ON o.id=(SELECT o2.id FROM orders o2 WHERE o2.tenant_id=t.tenant_id AND o2.store_id=t.store_id
			AND o2.table_id=t.id AND o2.order_type='DINE_IN' AND o2.status IN ('PENDING_PAYMENT','PAID','ACCEPTED','PREPARING','READY')
			ORDER BY o2.id DESC LIMIT 1)
		WHERE t.tenant_id=? AND t.store_id=? AND t.status='ACTIVE' AND t.deleted_at IS NULL
			AND a.status='ACTIVE' AND a.deleted_at IS NULL
		ORDER BY a.sort_order,a.id,t.sort_order,t.id`, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	areas := []map[string]any{}
	areaIndex := map[int64]int{}
	for rows.Next() {
		var item tableBoardTable
		if err = rows.Scan(&item.ID, &item.AreaID, &item.AreaName, &item.Name, &item.TableCode, &item.Capacity,
			&item.OrderID, &item.OrderNo, &item.OrderStatus, &item.CustomerName, &item.TotalCents, &item.OpenedAt); err != nil {
			handleSQLError(w, err)
			return
		}
		item.State = "UNOPENED"
		if item.OrderStatus == "PENDING_PAYMENT" || item.OrderStatus == "PAID" {
			item.State = "OPENED"
		} else if item.OrderStatus != "" {
			item.State = "DINING"
		}
		index, exists := areaIndex[item.AreaID]
		if !exists {
			index = len(areas)
			areaIndex[item.AreaID] = index
			areas = append(areas, map[string]any{"id": item.AreaID, "name": item.AreaName, "tables": []tableBoardTable{}})
		}
		areas[index]["tables"] = append(areas[index]["tables"].([]tableBoardTable), item)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	settings, settingsErr := s.loadStoreOperationSettings(r, actor.TenantID, storeID)
	if settingsErr != nil {
		handleSQLError(w, settingsErr)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"areas": areas, "settlementMode": settings.SettlementMode, "orderingMode": settings.OrderingMode})
}

func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000
	toRadians := math.Pi / 180
	dLat := (lat2 - lat1) * toRadians
	dLon := (lon2 - lon1) * toRadians
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*toRadians)*math.Cos(lat2*toRadians)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
