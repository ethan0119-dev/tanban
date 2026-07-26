package app

import (
	"context"
	"database/sql"
	"encoding/json"
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
	r.Post("/customer/session", s.publicCreateCustomerSession)
	r.Get("/table-codes/{code}", s.publicResolveTableCode)
	r.Get("/fast-food-plates/{code}", s.publicResolveFastFoodPlate)
	r.Get("/stores/{storeCode}", s.publicStore)
	r.Get("/stores/{storeCode}/catalog", s.publicCatalog)
	r.Get("/stores/{storeCode}/membership", s.publicMembership)
	r.Get("/stores/{storeCode}/stored-value", s.publicStoredValue)
	r.Post("/stores/{storeCode}/membership/orders", s.publicCreateMembershipOrder)
	r.Post("/stores/{storeCode}/stored-value/orders", s.publicCreateStoredValueOrder)
	r.Get("/account-payments/{paymentID}", s.publicGetAccountPayment)
	r.Post("/account-payments/{paymentID}/mock-confirm", s.publicAccountPaymentMockConfirm)
	r.Post("/stores/{storeCode}/orders", s.publicCreateOrder)
	r.Get("/orders/{orderNo}", s.publicGetOrder)
	r.Post("/orders/{orderNo}/pay", s.publicPayOrder)
	r.Post("/orders/{orderNo}/payments", s.publicPayOrder)
	r.Post("/payments/{paymentID}/mock-confirm", s.publicMockConfirm)
	r.Get("/customer/orders", s.publicCustomerOrders)
	s.registerPublicMarketingRoutes(r)
}

func (s *Server) publicMembership(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var enabled, showBalance bool
	var cardName, cardColor, cardImageURL, agreementURL string
	err = s.DB.QueryRowContext(r.Context(), `SELECT enabled,card_name,card_color,card_image_url,agreement_url,show_balance
		FROM membership_settings WHERE tenant_id=?`, store.TenantID).
		Scan(&enabled, &cardName, &cardColor, &cardImageURL, &agreementURL, &showBalance)
	if errors.Is(err, sql.ErrNoRows) {
		writeData(w, http.StatusOK, map[string]any{"available": false, "levels": []any{}})
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	levels := []map[string]any{}
	if enabled {
		rows, queryErr := s.DB.QueryContext(r.Context(), `SELECT id,name,rank_no,acquire_type,price_cents,valid_days,benefits_json,is_default
			FROM member_levels WHERE tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL ORDER BY rank_no,id`, store.TenantID)
		if queryErr != nil {
			handleSQLError(w, queryErr)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, price int64
			var rank, validDays int
			var name, acquireType, benefitsJSON string
			var isDefault bool
			if scanErr := rows.Scan(&id, &name, &rank, &acquireType, &price, &validDays, &benefitsJSON, &isDefault); scanErr != nil {
				handleSQLError(w, scanErr)
				return
			}
			levels = append(levels, map[string]any{"id": id, "name": name, "rank": rank, "acquireType": acquireType, "rechargeCents": price, "validDays": validDays, "discountPercent": decodeMemberDiscount(benefitsJSON), "isDefault": isDefault})
		}
	}
	view := map[string]any{
		"available": enabled,
		"card":      map[string]any{"name": cardName, "color": cardColor, "imageUrl": cardImageURL, "agreementUrl": agreementURL, "showBalance": showBalance},
		"levels":    levels,
	}
	session, hasSession := s.optionalPublicCustomerSession(r.Context(), r, store.TenantID)
	if enabled && hasSession {
		var memberID sql.NullInt64
		var memberNo, levelName string
		var levelID sql.NullInt64
		var principal, bonus int64
		memberErr := s.DB.QueryRowContext(r.Context(), `SELECT m.id,COALESCE(m.member_no,''),m.current_level_id,COALESCE(l.name,''),
			COALESCE(ba.principal_cents,0),COALESCE(ba.bonus_cents,0)
			FROM customers c
			LEFT JOIN members m ON m.tenant_id=c.tenant_id AND m.customer_id=c.id AND m.status='ACTIVE'
			LEFT JOIN member_levels l ON l.tenant_id=m.tenant_id AND l.id=m.current_level_id AND l.status='ACTIVE' AND l.deleted_at IS NULL
			LEFT JOIN balance_accounts ba ON ba.tenant_id=c.tenant_id AND ba.customer_id=c.id
			WHERE c.tenant_id=? AND c.id=? AND c.status='ACTIVE' AND c.deleted_at IS NULL`,
			store.TenantID, session.CustomerID).Scan(&memberID, &memberNo, &levelID, &levelName, &principal, &bonus)
		if memberErr == nil {
			view["member"] = map[string]any{"memberId": memberID.Int64, "memberNo": memberNo, "levelId": levelID.Int64, "levelName": levelName, "principalCents": principal, "bonusCents": bonus, "balanceCents": principal + bonus}
		} else if !errors.Is(memberErr, sql.ErrNoRows) {
			handleSQLError(w, memberErr)
			return
		}
	}
	writeData(w, http.StatusOK, view)
}

func (s *Server) publicStoredValue(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	settings := storedValueSettingsInput{
		MinRechargeCents: 100,
		MaxRechargeCents: 1000000,
		MaxBalanceCents:  1000000,
		DeductionOrder:   "BONUS_FIRST",
		RefundPolicy:     "MANUAL_REVIEW",
	}
	err = s.DB.QueryRowContext(r.Context(), `SELECT enabled,min_recharge_cents,max_recharge_cents,max_balance_cents,deduction_order,refund_policy,agreement_url,show_in_miniapp FROM stored_value_settings WHERE tenant_id=?`, store.TenantID).
		Scan(&settings.Enabled, &settings.MinRechargeCents, &settings.MaxRechargeCents, &settings.MaxBalanceCents, &settings.DeductionOrder, &settings.RefundPolicy, &settings.AgreementURL, &settings.ShowInMiniapp)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	available := settings.Enabled && settings.ShowInMiniapp
	rules := []map[string]any{}
	if available {
		rows, queryErr := s.DB.QueryContext(r.Context(), `SELECT id,name,recharge_cents,gift_cents,gift_growth,per_customer_limit,IF(starts_at IS NULL,NULL,DATE_FORMAT(starts_at,'%Y-%m-%d %H:%i:%s')),IF(ends_at IS NULL,NULL,DATE_FORMAT(ends_at,'%Y-%m-%d %H:%i:%s')) FROM stored_value_rules WHERE tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL AND recharge_cents BETWEEN ? AND ? AND (starts_at IS NULL OR starts_at<=NOW(3)) AND (ends_at IS NULL OR ends_at>=NOW(3)) ORDER BY recharge_cents,id`,
			store.TenantID, settings.MinRechargeCents, settings.MaxRechargeCents)
		if queryErr != nil {
			handleSQLError(w, queryErr)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, recharge, gift, growth int64
			var limit int
			var name string
			var starts, ends sql.NullString
			if scanErr := rows.Scan(&id, &name, &recharge, &gift, &growth, &limit, &starts, &ends); scanErr != nil {
				handleSQLError(w, scanErr)
				return
			}
			item := map[string]any{"id": id, "name": name, "rechargeCents": recharge, "giftCents": gift, "giftGrowth": growth, "perCustomerLimit": limit}
			if starts.Valid {
				item["startsAt"] = starts.String
			}
			if ends.Valid {
				item["endsAt"] = ends.String
			}
			rules = append(rules, item)
		}
	}
	message := "储值支付暂未开放"
	if available {
		message = "请选择储值金额"
	}
	balance := map[string]any{"principalCents": int64(0), "bonusCents": int64(0), "balanceCents": int64(0)}
	session, hasSession := s.optionalPublicCustomerSession(r.Context(), r, store.TenantID)
	if hasSession {
		var principal, bonus int64
		balanceErr := s.DB.QueryRowContext(r.Context(), `SELECT COALESCE(ba.principal_cents,0),COALESCE(ba.bonus_cents,0)
			FROM customers c LEFT JOIN balance_accounts ba ON ba.tenant_id=c.tenant_id AND ba.customer_id=c.id
			WHERE c.tenant_id=? AND c.id=? AND c.status='ACTIVE' AND c.deleted_at IS NULL`,
			store.TenantID, session.CustomerID).Scan(&principal, &bonus)
		if balanceErr == nil {
			balance = map[string]any{"principalCents": principal, "bonusCents": bonus, "balanceCents": principal + bonus}
		} else if !errors.Is(balanceErr, sql.ErrNoRows) {
			handleSQLError(w, balanceErr)
			return
		}
	}
	writeData(w, http.StatusOK, map[string]any{
		"available": available,
		"message":   message,
		"balance":   balance,
		"settings": map[string]any{
			"minRechargeCents": settings.MinRechargeCents,
			"maxRechargeCents": settings.MaxRechargeCents,
			"maxBalanceCents":  settings.MaxBalanceCents,
			"agreementUrl":     settings.AgreementURL,
		},
		"rules": rules,
	})
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
	memberBenefit := publicMemberBenefit{DiscountPercent: 100}
	if session, ok := s.optionalPublicCustomerSession(r.Context(), r, store.TenantID); ok {
		memberBenefit, err = publicMemberBenefitByCustomer(r.Context(), s.DB, store.TenantID, session.CustomerID)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	products, err := s.loadProducts(r.Context(), store.TenantID, store.ID, true, 0)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT id,store_id,name,sort_order,in_store_enabled,delivery_enabled,status FROM categories WHERE tenant_id=? AND store_id=? AND status='ACTIVE' AND in_store_enabled=1 AND deleted_at IS NULL ORDER BY sort_order,id", store.TenantID, store.ID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	categories := []categoryDTO{}
	for rows.Next() {
		var item categoryDTO
		if err := rows.Scan(&item.ID, &item.StoreID, &item.Name, &item.SortOrder, &item.InStoreEnabled, &item.DeliveryEnabled, &item.Status); err != nil {
			handleSQLError(w, err)
			return
		}
		categories = append(categories, item)
	}
	publicCategories := make([]map[string]any, 0, len(categories))
	for _, category := range categories {
		publicCategories = append(publicCategories, map[string]any{"id": category.ID, "name": category.Name, "sortOrder": category.SortOrder, "inStoreEnabled": category.InStoreEnabled, "deliveryEnabled": category.DeliveryEnabled})
	}
	publicProducts := make([]map[string]any, 0, len(products))
	for _, product := range products {
		var stock int
		var minPrice int64
		publicImages := make([]map[string]any, 0, len(product.Images))
		for _, image := range product.Images {
			publicImages = append(publicImages, map[string]any{"url": image.URL, "isPrimary": image.IsPrimary, "sortOrder": image.SortOrder})
		}
		publicSKUs := make([]map[string]any, 0, len(product.SKUs))
		for index, sku := range product.SKUs {
			stock += sku.Stock
			if index == 0 || sku.PriceCents < minPrice {
				minPrice = sku.PriceCents
			}
			skuView := map[string]any{"id": sku.ID, "name": sku.Name, "price": sku.PriceCents, "stock": sku.Stock, "soldOut": sku.Stock <= 0}
			if product.MemberDiscount && memberBenefit.DiscountPercent < 100 {
				skuView["memberPrice"] = discountedMemberPrice(sku.PriceCents, memberBenefit.DiscountPercent)
			}
			publicSKUs = append(publicSKUs, skuView)
		}
		configuration, configErr := s.loadProductConfiguration(r.Context(), store.TenantID, store.ID, product.ID, true)
		if configErr != nil {
			handleSQLError(w, configErr)
			return
		}
		productView := map[string]any{"id": product.ID, "categoryId": product.CategoryID, "name": product.Name, "description": product.Description, "imageUrl": product.ImageURL, "images": publicImages, "price": minPrice, "stock": stock, "soldOut": len(product.SKUs) == 0 || stock <= 0, "recommended": product.Recommended, "memberDiscountEnabled": product.MemberDiscount, "inStoreEnabled": product.InStoreEnabled, "deliveryEnabled": product.DeliveryEnabled, "skus": publicSKUs, "optionGroups": publicOptionGroups(configuration.OptionGroups), "modifierGroups": publicModifierGroups(configuration.ModifierGroups)}
		if product.MemberDiscount && memberBenefit.DiscountPercent < 100 {
			productView["memberPrice"] = discountedMemberPrice(minPrice, memberBenefit.DiscountPercent)
			productView["memberDiscountPercent"] = memberBenefit.DiscountPercent
		}
		publicProducts = append(publicProducts, productView)
	}
	catalogView := map[string]any{"store": s.publicStoreView(r.Context(), store), "categories": publicCategories, "products": publicProducts}
	if memberBenefit.MemberID > 0 {
		catalogView["membership"] = map[string]any{"memberId": memberBenefit.MemberID, "levelId": memberBenefit.LevelID, "levelName": memberBenefit.LevelName, "discountPercent": memberBenefit.DiscountPercent}
	}
	writeData(w, http.StatusOK, catalogView)
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
	return map[string]any{"id": store.ID, "code": store.Code, "name": store.Name, "logoUrl": store.LogoURL, "address": store.Address, "phone": store.Phone, "businessHours": store.BusinessHours, "theme": map[string]any{"bannerUrl": store.BannerURL, "announcement": store.Notice}}
}

func (s *Server) publicStoreView(ctx context.Context, store storeDTO) map[string]any {
	view := publicStoreView(store)
	state, err := s.currentStoreBusinessState(ctx, s.DB, store.TenantID, store.ID)
	if err != nil {
		// A configuration read failure must fail closed at the public boundary.
		s.Logger.Error("load public store business status", "store_id", store.ID, "error", err)
		state = storeBusinessState{Open: false, Reason: "SCHEDULE_UNAVAILABLE", Message: "暂时无法接单", Timezone: defaultStoreTimezone, BusinessDate: time.Now().Format("2006-01-02")}
	}
	for key, value := range businessStateView(state) {
		view[key] = value
	}
	decoration, version := s.publicDecorationConfig(ctx, store)
	view["decoration"] = decoration
	view["decorationVersion"] = version
	var settlementMode, orderingMode, customerServicePhone, customerServiceWechat, customerServiceQRURL, privacyPolicyText, userAgreementText string
	var distanceCheckEnabled, requireCustomerPhone, allowOrderRemark, allowItemRemark, payAfterOnlinePaymentEnabled bool
	var distanceLimitM int
	var storeLatitude, storeLongitude sql.NullFloat64
	err = s.DB.QueryRowContext(ctx, `SELECT settlement_mode,ordering_mode,pay_after_online_payment_enabled,distance_check_enabled,distance_limit_m,store_latitude,store_longitude,require_customer_phone,
		allow_order_remark,allow_item_remark,customer_service_phone,customer_service_wechat,customer_service_qr_url,
		privacy_policy_text,user_agreement_text FROM store_operation_settings WHERE tenant_id=? AND store_id=?`, store.TenantID, store.ID).
		Scan(&settlementMode, &orderingMode, &payAfterOnlinePaymentEnabled, &distanceCheckEnabled, &distanceLimitM, &storeLatitude, &storeLongitude, &requireCustomerPhone, &allowOrderRemark, &allowItemRemark,
			&customerServicePhone, &customerServiceWechat, &customerServiceQRURL, &privacyPolicyText, &userAgreementText)
	if err == nil {
		view["orderingSettings"] = map[string]any{"settlementMode": settlementMode, "orderingMode": orderingMode, "distanceCheckEnabled": distanceCheckEnabled,
			"distanceLimitM": distanceLimitM, "requireCustomerPhone": requireCustomerPhone, "allowOrderRemark": allowOrderRemark,
			"allowItemRemark": allowItemRemark, "payAfterOnlinePaymentEnabled": payAfterOnlinePaymentEnabled}
		if storeLatitude.Valid && storeLongitude.Valid {
			view["location"] = map[string]any{"latitude": storeLatitude.Float64, "longitude": storeLongitude.Float64}
		}
		view["customerService"] = map[string]any{"phone": customerServicePhone, "wechat": customerServiceWechat, "qrUrl": customerServiceQRURL}
		view["legal"] = map[string]any{"privacyPolicy": privacyPolicyText, "userAgreement": userAgreementText}
	} else if !errors.Is(err, sql.ErrNoRows) {
		s.Logger.Error("load public store operation settings", "store_id", store.ID, "error", err)
	}
	branding := systemSettings{
		PlatformName:      "摊伴餐饮系统",
		MarketingTitle:    "让每一家小店，都能轻松拥有自己的数字化点餐系统",
		MarketingSubtitle: "点餐、营销、会员与门店经营，一套系统顺畅连接。",
	}
	var settingsJSON string
	if settingErr := s.DB.QueryRowContext(ctx, "SELECT value_text FROM platform_settings WHERE setting_key='system'").Scan(&settingsJSON); settingErr == nil {
		if unmarshalErr := json.Unmarshal([]byte(settingsJSON), &branding); unmarshalErr != nil {
			s.Logger.Error("decode public platform branding", "error", unmarshalErr)
		}
	} else if !errors.Is(settingErr, sql.ErrNoRows) {
		s.Logger.Error("load public platform branding", "error", settingErr)
	}
	view["platformBranding"] = map[string]any{
		"platformName": branding.PlatformName, "marketingTitle": branding.MarketingTitle,
		"marketingSubtitle": branding.MarketingSubtitle, "contactWechat": branding.ContactWechat,
		"contactQrUrl": branding.ContactQRURL, "marketingPageUrl": branding.MarketingPageURL,
	}
	return view
}

func (s *Server) findPublicStore(ctx context.Context, code string) (storeDTO, error) {
	var store storeDTO
	err := scanStore(s.DB.QueryRowContext(ctx, `SELECT s.id,s.tenant_id,s.code,s.name,s.logo_url,s.banner_url,s.address,s.phone,s.business_hours,s.notice,s.status,DATE_FORMAT(s.created_at,'%Y-%m-%d %H:%i:%s')
		FROM stores s JOIN tenants t ON t.id=s.tenant_id LEFT JOIN store_profiles p ON p.store_id=s.id AND p.tenant_id=s.tenant_id
		WHERE s.code=? AND s.status='ACTIVE' AND COALESCE(p.visible_in_miniapp,1)=1 AND s.deleted_at IS NULL
		AND t.status='ACTIVE' AND (t.service_expires_at IS NULL OR t.service_expires_at >= CURRENT_DATE) AND t.deleted_at IS NULL`, code), &store)
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
	OpenID                      string                 `json:"openid"`
	CustomerKey                 string                 `json:"customerKey"`
	CustomerName                string                 `json:"customer_name"`
	CustomerPhone               string                 `json:"customer_phone"`
	Fulfillment                 string                 `json:"fulfillmentType"`
	Remark                      string                 `json:"remark"`
	Items                       []publicOrderItemInput `json:"items"`
	OrderType                   string                 `json:"orderType"`
	OrderScene                  string                 `json:"order_scene"`
	TablePublicID               string                 `json:"table_public_id"`
	TableScene                  string                 `json:"tableScene"`
	FastFoodPlatePublicID       string                 `json:"fastFoodPlatePublicId"`
	LegacyFastFoodPlatePublicID string                 `json:"fast_food_plate_public_id"`
	CustomerLatitude            *float64               `json:"customerLatitude"`
	CustomerLongitude           *float64               `json:"customerLongitude"`
	CouponCampaignID            int64                  `json:"couponCampaignId"`
	DisableStorePromotion       bool                   `json:"disableStorePromotion"`
	DinerCount                  int                    `json:"dinerCount"`
	VerifiedCustomerID          int64                  `json:"-"`
}

type publicMemberBenefit struct {
	CustomerID      int64
	MemberID        int64
	LevelID         int64
	LevelName       string
	DiscountPercent int
}

func discountedMemberPrice(amount int64, percent int) int64 {
	if percent <= 0 || percent >= 100 {
		return amount
	}
	return (amount*int64(percent) + 50) / 100
}

func decodeMemberDiscount(benefitsJSON string) int {
	benefits := map[string]any{}
	if json.Unmarshal([]byte(benefitsJSON), &benefits) != nil {
		return 100
	}
	raw, ok := benefits["discount"]
	if !ok {
		return 100
	}
	var discount int
	switch value := raw.(type) {
	case float64:
		discount = int(value)
	case string:
		discount, _ = strconv.Atoi(strings.TrimSpace(value))
	}
	if discount < 1 || discount > 100 {
		return 100
	}
	return discount
}

func publicMemberBenefitByCustomer(ctx context.Context, queryer sqlQueryer, tenantID, customerID int64) (publicMemberBenefit, error) {
	if customerID <= 0 {
		return publicMemberBenefit{DiscountPercent: 100}, nil
	}
	var result publicMemberBenefit
	var benefitsJSON string
	err := queryer.QueryRowContext(ctx, `SELECT c.id,m.id,l.id,l.name,l.benefits_json
		FROM customers c
		JOIN membership_settings ms ON ms.tenant_id=c.tenant_id AND ms.enabled=1
		JOIN members m ON m.tenant_id=c.tenant_id AND m.customer_id=c.id AND m.status='ACTIVE'
			AND (m.expires_at IS NULL OR m.expires_at>NOW(3))
		JOIN member_levels l ON l.tenant_id=m.tenant_id AND l.id=m.current_level_id
			AND l.status='ACTIVE' AND l.deleted_at IS NULL
		WHERE c.tenant_id=? AND c.id=? AND c.status='ACTIVE' AND c.deleted_at IS NULL`,
		tenantID, customerID).Scan(&result.CustomerID, &result.MemberID, &result.LevelID, &result.LevelName, &benefitsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return publicMemberBenefit{CustomerID: customerID, DiscountPercent: 100}, nil
	}
	if err != nil {
		return publicMemberBenefit{}, err
	}
	result.DiscountPercent = decodeMemberDiscount(benefitsJSON)
	return result, nil
}

type publicOrderCoupon struct {
	RecordID, CampaignID int64
	Name                 string
	Discount             int64
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
	cashierActor, cashierOrder := s.optionalCashierIdentity(r.Context(), r, store.TenantID)
	cashierPrintActorID := int64(0)
	if cashierOrder {
		cashierPrintActorID = cashierActor.UserID
	}
	// V1 runs a single API process, so the shared in-memory cache is sufficient
	// for a lightweight abuse guard. The cache interface can move this counter
	// to Redis when the API is scaled horizontally.
	if !cashierOrder {
		s.publicRateMu.Lock()
		rateKey := "public-order:" + store.Code + ":" + publicClientHost(r)
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
	}
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
	if input.FastFoodPlatePublicID == "" {
		input.FastFoodPlatePublicID = input.LegacyFastFoodPlatePublicID
	}
	input.FastFoodPlatePublicID = strings.TrimSpace(strings.TrimPrefix(input.FastFoodPlatePublicID, "fp="))
	input.LegacyFastFoodPlatePublicID = ""
	if orderType == orderTypeDineIn && input.TablePublicID == "" {
		writeError(w, http.StatusBadRequest, "TABLE_CODE_REQUIRED", "table_public_id is required for a dine-in order")
		return
	}
	if orderType != orderTypeDineIn && input.TablePublicID != "" {
		writeError(w, http.StatusBadRequest, "TABLE_CODE_NOT_ALLOWED", "table_public_id is only valid for a dine-in order")
		return
	}
	if orderType != orderTypeTakeout && input.FastFoodPlatePublicID != "" {
		writeError(w, http.StatusBadRequest, "FAST_FOOD_PLATE_NOT_ALLOWED", "fastFoodPlatePublicId is only valid for a takeout order")
		return
	}
	if input.OrderType == orderTypeDineIn {
		if input.DinerCount == 0 {
			input.DinerCount = 1
		}
		if input.DinerCount < 1 || input.DinerCount > 99 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "dinerCount must be between 1 and 99 for dine-in orders")
			return
		}
	} else {
		input.DinerCount = 1
	}
	// OpenID is accepted only from the signed customer session. A request-body
	// value is untrusted and must never unlock member pricing or stored value.
	input.OpenID = ""
	var customerSession publicCustomerSession
	if session, ok := s.optionalPublicCustomerSession(r.Context(), r, store.TenantID); ok {
		customerSession = session
		input.VerifiedCustomerID = session.CustomerID
		input.OpenID = session.OpenID
	}
	input.CustomerKey = strings.TrimSpace(input.CustomerKey)
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.CustomerPhone = strings.TrimSpace(input.CustomerPhone)
	if len(input.OpenID) > 128 || len(input.CustomerKey) > 128 || len([]rune(input.CustomerName)) > 80 || len(input.CustomerPhone) > 32 || len([]rune(input.Remark)) > 500 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "customer identity or order remark is too long")
		return
	}
	// Customer coordinates are a transient admission signal, not order content.
	// Excluding them keeps a safe idempotent retry from conflicting when a
	// second GPS reading differs by a few meters.
	fingerprintInput := input
	fingerprintInput.CustomerLatitude = nil
	fingerprintInput.CustomerLongitude = nil
	fingerprint := requestFingerprint(fingerprintInput)
	legacyFingerprint := ""
	if input.TablePublicID == "" && input.FastFoodPlatePublicID == "" {
		legacyFingerprint = legacyPublicOrderFingerprint(input)
	}
	var existingID int64
	var existingFingerprint string
	err = s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM orders WHERE tenant_id=? AND store_id=? AND idempotency_key=?", store.TenantID, store.ID, idempotencyKey).Scan(&existingID, &existingFingerprint)
	if err == nil {
		if existingFingerprint != "" && existingFingerprint != fingerprint && (legacyFingerprint == "" || existingFingerprint != legacyFingerprint) {
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
	err = s.DB.QueryRowContext(r.Context(), `SELECT order_id,request_fingerprint FROM order_addition_requests
		WHERE tenant_id=? AND store_id=? AND idempotency_key=?`, store.TenantID, store.ID, idempotencyKey).
		Scan(&existingID, &existingFingerprint)
	if err == nil {
		if existingFingerprint != fingerprint {
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
	businessState, err := s.ensureStoreAcceptingOrders(r.Context(), tx, store.TenantID, store.ID)
	if err != nil {
		if errors.Is(err, errStoreClosed) {
			writeError(w, http.StatusConflict, "STORE_CLOSED", businessState.Message)
			return
		}
		handleSQLError(w, err)
		return
	}
	policy := storeOperationSettings{SettlementMode: "PAY_BEFORE", OrderingMode: "MULTI_PERSON", PayBeforeClearMode: "AFTER_ORDER_COMPLETION", DistanceLimitM: 5000, AllowOrderRemark: true, AllowItemRemark: true}
	var storeLatitude, storeLongitude sql.NullFloat64
	err = tx.QueryRowContext(r.Context(), `SELECT settlement_mode,ordering_mode,pay_before_clear_mode,distance_check_enabled,distance_limit_m,store_latitude,store_longitude,
		require_customer_phone,allow_order_remark,allow_item_remark FROM store_operation_settings WHERE tenant_id=? AND store_id=?`, store.TenantID, store.ID).
		Scan(&policy.SettlementMode, &policy.OrderingMode, &policy.PayBeforeClearMode, &policy.DistanceCheckEnabled, &policy.DistanceLimitM, &storeLatitude, &storeLongitude,
			&policy.RequireCustomerPhone, &policy.AllowOrderRemark, &policy.AllowItemRemark)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	if policy.RequireCustomerPhone && input.CustomerPhone == "" {
		writeError(w, http.StatusBadRequest, "CUSTOMER_PHONE_REQUIRED", "customer phone is required by this store")
		return
	}
	if !policy.AllowOrderRemark && strings.TrimSpace(input.Remark) != "" {
		writeError(w, http.StatusBadRequest, "ORDER_REMARK_DISABLED", "order remarks are disabled by this store")
		return
	}
	if !policy.AllowItemRemark {
		for _, requested := range input.Items {
			if strings.TrimSpace(requested.ItemRemark) != "" {
				writeError(w, http.StatusBadRequest, "ITEM_REMARK_DISABLED", "item remarks are disabled by this store")
				return
			}
		}
	}
	if policy.DistanceCheckEnabled {
		if !storeLatitude.Valid || !storeLongitude.Valid || !validCoordinate(input.CustomerLatitude, input.CustomerLongitude) {
			writeError(w, http.StatusBadRequest, "CUSTOMER_LOCATION_REQUIRED", "a valid customer location is required by this store")
			return
		}
		distance := distanceMeters(storeLatitude.Float64, storeLongitude.Float64, *input.CustomerLatitude, *input.CustomerLongitude)
		if distance > float64(policy.DistanceLimitM) {
			writeError(w, http.StatusConflict, "CUSTOMER_OUT_OF_RANGE", "customer is outside the store ordering distance")
			return
		}
	}
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
	var additionOrderID, additionCurrentTotal int64
	var additionSequence int
	var additionSettlementMode, additionStatus string
	if input.OrderType == orderTypeDineIn {
		err = tx.QueryRowContext(r.Context(), `SELECT id,total_cents,addition_count+1,settlement_mode_snapshot,status FROM orders
			WHERE tenant_id=? AND store_id=? AND table_id=? AND order_type='DINE_IN'
			  AND payment_status='UNPAID' AND paid_cents=0
			  AND (
			    (settlement_mode_snapshot='PAY_AFTER' AND status IN ('PAID','ACCEPTED','PREPARING','READY'))
			    OR (settlement_mode_snapshot='PAY_BEFORE' AND status='PENDING_PAYMENT')
			  )
			  AND NOT EXISTS (SELECT 1 FROM payment_transactions p WHERE p.tenant_id=orders.tenant_id
			    AND p.order_id=orders.id AND p.status IN ('CREATING','PENDING','SUCCESS'))
			ORDER BY id DESC LIMIT 1 FOR UPDATE`, store.TenantID, store.ID, table.ID).
			Scan(&additionOrderID, &additionCurrentTotal, &additionSequence, &additionSettlementMode, &additionStatus)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			handleSQLError(w, err)
			return
		}
	}
	if input.OrderType == orderTypeDineIn && policy.OrderingMode == "SINGLE_PERSON" {
		var occupied int
		err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM orders o LEFT JOIN customers c ON c.id=o.customer_id AND c.tenant_id=o.tenant_id
			WHERE o.tenant_id=? AND o.store_id=? AND o.table_id=? AND o.order_type='DINE_IN'
			AND o.status IN ('PENDING_PAYMENT','PAID','ACCEPTED','PREPARING','READY')
			AND NOT (?='IMMEDIATE_AFTER_PAYMENT' AND o.settlement_mode_snapshot='PAY_BEFORE' AND o.payment_status='PAID')
			AND NOT ((?<>'' AND o.customer_openid=?) OR (?<>'' AND c.guest_key=?))`, store.TenantID, store.ID, table.ID,
			policy.PayBeforeClearMode, input.OpenID, input.OpenID, input.CustomerKey, input.CustomerKey).Scan(&occupied)
		if err != nil {
			handleSQLError(w, err)
			return
		}
		if occupied > 0 {
			writeError(w, http.StatusConflict, "TABLE_ORDERING_SESSION_OCCUPIED", "this table is currently limited to one ordering customer")
			return
		}
	}
	var fastFoodPlate fastFoodPlateReference
	if input.FastFoodPlatePublicID != "" {
		fastFoodPlate, err = resolveOrderFastFoodPlate(r.Context(), tx, store.TenantID, store.ID, input.FastFoodPlatePublicID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusBadRequest, "INVALID_FAST_FOOD_PLATE", "fast-food plate does not belong to this store or is disabled")
				return
			}
			handleSQLError(w, err)
			return
		}
	}
	customerID, err := upsertPublicOrderCustomer(r.Context(), tx, store, input)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	memberBenefit := publicMemberBenefit{CustomerID: customerID, DiscountPercent: 100}
	if input.VerifiedCustomerID > 0 {
		memberBenefit, err = publicMemberBenefitByCustomer(r.Context(), tx, store.TenantID, input.VerifiedCustomerID)
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	type resolvedItem struct {
		productID, skuID, basePrice, modifierPrice, unitPrice               int64
		originalPrice, memberDiscount                                       int64
		productName, skuName, productType, attrs, configuration, itemRemark string
		memberDiscountEnabled                                               bool
		quantity                                                            int
	}
	resolved := make([]resolvedItem, 0, len(input.Items))
	var total int64
	var memberDiscountTotal int64
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
		err = tx.QueryRowContext(r.Context(), `SELECT p.id,s.id,p.name,s.name,p.product_type,s.attributes_json,s.price_cents,p.member_discount_enabled,i.stock FROM skus s JOIN products p ON p.id=s.product_id JOIN categories c ON c.id=p.category_id AND c.tenant_id=p.tenant_id AND c.store_id=p.store_id JOIN inventory i ON i.sku_id=s.id WHERE s.id=? AND s.tenant_id=? AND s.store_id=? AND s.status='ACTIVE' AND p.status='ACTIVE' AND p.in_store_enabled=1 AND c.status='ACTIVE' AND c.in_store_enabled=1 AND s.deleted_at IS NULL AND p.deleted_at IS NULL AND c.deleted_at IS NULL FOR UPDATE`, requested.SKUID, store.TenantID, store.ID).
			Scan(&row.productID, &row.skuID, &row.productName, &row.skuName, &row.productType, &row.attrs, &row.basePrice, &row.memberDiscountEnabled, &stock)
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
		row.originalPrice = row.basePrice + row.modifierPrice
		row.unitPrice = row.originalPrice
		if row.memberDiscountEnabled && memberBenefit.MemberID > 0 && memberBenefit.DiscountPercent < 100 {
			row.unitPrice = discountedMemberPrice(row.originalPrice, memberBenefit.DiscountPercent)
			row.memberDiscount = row.originalPrice - row.unitPrice
		}
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
		memberDiscountTotal += row.memberDiscount * int64(row.quantity)
		resolved = append(resolved, row)
	}
	merchandiseSubtotal := total
	var appliedPromotion fullReductionRow
	if !input.DisableStorePromotion {
		now := time.Now().UTC()
		promotionRows, promotionErr := tx.QueryContext(r.Context(), fullReductionSelect+` WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL
			AND status='ACTIVE' AND threshold_cents<=? AND (active_from IS NULL OR active_from<=?) AND (active_to IS NULL OR active_to>?)
			ORDER BY discount_cents DESC,threshold_cents ASC,id ASC FOR UPDATE`, store.TenantID, store.ID, merchandiseSubtotal, now, now)
		if promotionErr != nil {
			handleSQLError(w, promotionErr)
			return
		}
		for promotionRows.Next() {
			candidate, scanErr := scanFullReduction(promotionRows)
			if scanErr != nil {
				promotionRows.Close()
				handleSQLError(w, scanErr)
				return
			}
			if marketingOrderTypesContain(decodeMarketingOrderTypes(candidate.OrderTypesJSON), input.OrderType) {
				appliedPromotion = candidate
				break
			}
		}
		if promotionErr = promotionRows.Close(); promotionErr != nil {
			handleSQLError(w, promotionErr)
			return
		}
		if appliedPromotion.ID > 0 {
			discount := appliedPromotion.DiscountCents
			if discount > total {
				discount = total
			}
			appliedPromotion.DiscountCents = discount
			total -= discount
		}
	}
	var appliedCoupon publicOrderCoupon
	if input.CouponCampaignID > 0 {
		subjectHash, hashErr := marketingSubjectHash(input.CustomerKey)
		if hashErr != nil {
			writeError(w, http.StatusBadRequest, "COUPON_CUSTOMER_REQUIRED", "customer identity is required to use a coupon")
			return
		}
		var couponType, orderTypesJSON string
		var threshold int64
		err = tx.QueryRowContext(r.Context(), `SELECT cc.id,c.name,c.coupon_type,c.threshold_cents,c.discount_cents,c.order_types_json
			FROM customer_coupons cc JOIN coupon_campaigns c ON c.id=cc.campaign_id AND c.tenant_id=cc.tenant_id
			WHERE cc.tenant_id=? AND cc.store_id=? AND cc.campaign_id=? AND cc.subject_key_hash=?
			  AND cc.status='PROVISIONAL' AND cc.valid_from<=NOW(3) AND cc.valid_to>NOW(3)
			ORDER BY cc.id LIMIT 1 FOR UPDATE`, store.TenantID, store.ID, input.CouponCampaignID, subjectHash).
			Scan(&appliedCoupon.RecordID, &appliedCoupon.Name, &couponType, &threshold, &appliedCoupon.Discount, &orderTypesJSON)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusConflict, "COUPON_NOT_AVAILABLE", "coupon is unavailable, expired, or already used")
			return
		}
		if err != nil {
			handleSQLError(w, err)
			return
		}
		if !marketingOrderTypesContain(decodeMarketingOrderTypes(orderTypesJSON), input.OrderType) {
			writeError(w, http.StatusConflict, "COUPON_ORDER_TYPE_MISMATCH", "coupon cannot be used for this order type")
			return
		}
		if couponType == "FULL_REDUCTION" && total < threshold {
			writeError(w, http.StatusConflict, "COUPON_THRESHOLD_NOT_MET", "order amount does not meet the coupon threshold")
			return
		}
		if appliedCoupon.Discount > total {
			appliedCoupon.Discount = total
		}
		appliedCoupon.CampaignID = input.CouponCampaignID
		total -= appliedCoupon.Discount
	}
	if additionOrderID > 0 {
		if total > maxCatalogOrderCents-additionCurrentTotal {
			writeError(w, http.StatusConflict, "ORDER_AMOUNT_LIMIT", "table bill amount exceeds the allowed range")
			return
		}
		additionResult, updateErr := tx.ExecContext(r.Context(), `UPDATE orders SET
			total_cents=total_cents+?,merchandise_subtotal_cents=merchandise_subtotal_cents+?,
			store_promotion_discount_cents=store_promotion_discount_cents+?,
			coupon_discount_cents=coupon_discount_cents+?,member_discount_cents=member_discount_cents+?,addition_count=addition_count+1,
			store_promotion_id=COALESCE(?,store_promotion_id),
			store_promotion_name=IF(?<>'',?,store_promotion_name),
			coupon_campaign_id=COALESCE(?,coupon_campaign_id),
			coupon_name=IF(?<>'',?,coupon_name),
			customer_id=COALESCE(customer_id,?),customer_openid=IF(customer_openid='',?,customer_openid),
			customer_name=IF(customer_name='',?,customer_name),customer_phone=IF(customer_phone='',?,customer_phone),
			status=IF(status='READY','PAID',status),updated_at=NOW(3)
			WHERE id=? AND tenant_id=? AND order_type='DINE_IN' AND payment_status='UNPAID' AND paid_cents=0
			  AND settlement_mode_snapshot=? AND status=? AND addition_count=?
			  AND NOT EXISTS (SELECT 1 FROM payment_transactions p WHERE p.tenant_id=orders.tenant_id
			    AND p.order_id=orders.id AND p.status IN ('CREATING','PENDING','SUCCESS'))`,
			total, merchandiseSubtotal, appliedPromotion.DiscountCents, appliedCoupon.Discount, memberDiscountTotal,
			nullableID(appliedPromotion.ID), appliedPromotion.Name, appliedPromotion.Name,
			nullableID(appliedCoupon.CampaignID), appliedCoupon.Name, appliedCoupon.Name,
			nullableID(customerID), input.OpenID, input.CustomerName, input.CustomerPhone,
			additionOrderID, store.TenantID, additionSettlementMode, additionStatus, additionSequence-1)
		if updateErr != nil {
			handleSQLError(w, updateErr)
			return
		}
		if changed, _ := additionResult.RowsAffected(); changed != 1 {
			writeError(w, http.StatusConflict, "TABLE_BILL_CHANGED", "桌台账单已更新，请刷新后重新加菜")
			return
		}
		requestResult, requestErr := tx.ExecContext(r.Context(), `INSERT INTO order_addition_requests(
			tenant_id,store_id,order_id,idempotency_key,request_fingerprint,addition_sequence
		) VALUES(?,?,?,?,?,?)`, store.TenantID, store.ID, additionOrderID, idempotencyKey, fingerprint, additionSequence)
		if requestErr != nil {
			handleSQLError(w, requestErr)
			return
		}
		requestID, _ := requestResult.LastInsertId()
		if appliedCoupon.RecordID > 0 {
			couponUpdate, couponErr := tx.ExecContext(r.Context(), `UPDATE customer_coupons SET status='RESERVED',order_id=?
				WHERE id=? AND tenant_id=? AND status='PROVISIONAL' AND order_id IS NULL`, additionOrderID, appliedCoupon.RecordID, store.TenantID)
			if couponErr != nil {
				handleSQLError(w, couponErr)
				return
			}
			if changed, _ := couponUpdate.RowsAffected(); changed != 1 {
				writeError(w, http.StatusConflict, "COUPON_NOT_AVAILABLE", "coupon was used by another order")
				return
			}
		}
		for _, row := range resolved {
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO order_items(tenant_id,order_id,addition_sequence,product_id,sku_id,product_name,sku_name,product_type,base_price_cents,modifier_price_cents,original_unit_price_cents,member_discount_cents,member_level_id_snapshot,member_level_name_snapshot,attributes_json,configuration_json,item_remark,unit_price_cents,quantity,subtotal_cents) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, store.TenantID, additionOrderID, additionSequence, row.productID, row.skuID, row.productName, row.skuName, row.productType, row.basePrice, row.modifierPrice, row.originalPrice, row.memberDiscount, nullableID(memberBenefit.LevelID), memberBenefit.LevelName, row.attrs, row.configuration, row.itemRemark, row.unitPrice, row.quantity, row.unitPrice*int64(row.quantity)); err != nil {
				handleSQLError(w, err)
				return
			}
		}
		if additionSettlementMode == "PAY_AFTER" {
			extra := fmt.Sprintf("【加单 #%d】", additionSequence)
			dedupeKey := fmt.Sprintf("ORDER:%d:ADDITION:%d", additionOrderID, requestID)
			if err = enqueuePrintOutboxWith(r.Context(), tx, store.TenantID, store.ID, additionOrderID, "ORDER_CREATED", dedupeKey, cashierPrintActorID, extra); err != nil {
				handleSQLError(w, err)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			handleSQLError(w, err)
			return
		}
		item, loadErr := s.loadOrder(r.Context(), store.TenantID, additionOrderID, "")
		if loadErr != nil {
			handleSQLError(w, loadErr)
			return
		}
		writeData(w, http.StatusCreated, publicOrderView(item))
		return
	}
	orderNo := newBusinessNo("TB")
	businessDate := businessState.BusinessDate
	var pickupSequence int64
	pickupCode := ""
	if input.OrderType == orderTypeTakeout {
		pickupSequence, pickupCode, err = allocatePickupCode(r.Context(), tx, store.TenantID, store.ID, businessDate)
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	initialStatus := "PENDING_PAYMENT"
	settlementModeSnapshot := settlementModeForOrder(input.OrderType, policy.SettlementMode)
	if settlementModeSnapshot == "PAY_AFTER" {
		initialStatus = "PAID"
	}
	sourceChannelKey := customerSession.MiniAppChannelKey
	if sourceChannelKey == "" {
		sourceChannelKey = publicMiniAppChannelKey
	}
	orderSource := "MINIPROGRAM"
	if cashierOrder {
		orderSource = "CASHIER"
		sourceChannelKey = "cashier-terminal"
		customerSession.MiniAppID = ""
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO orders(tenant_id,store_id,order_no,idempotency_key,request_fingerprint,source,source_miniapp_channel_key,source_miniapp_appid,customer_openid,customer_id,member_id_snapshot,member_level_id_snapshot,member_level_name_snapshot,customer_name,customer_phone,remark,fulfillment_type,order_type,settlement_mode_snapshot,addition_count,diner_count,status,business_date,pickup_sequence,pickup_code,fast_food_plate_id,fast_food_plate_public_id_snapshot,fast_food_plate_name_snapshot,fast_food_plate_code_snapshot,table_id,table_public_id_snapshot,table_area_name_snapshot,table_name_snapshot,table_code_snapshot,inventory_reserved,stock_reserved_at,total_cents,merchandise_subtotal_cents,member_discount_cents,store_promotion_id,store_promotion_name,store_promotion_discount_cents,coupon_campaign_id,coupon_name,coupon_discount_cents)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,NOW(3),?,?,?,?,?,?,?,?,?)`, store.TenantID, store.ID, orderNo, idempotencyKey, fingerprint, orderSource, sourceChannelKey, customerSession.MiniAppID, input.OpenID, nullableID(customerID), nullableID(memberBenefit.MemberID), nullableID(memberBenefit.LevelID), memberBenefit.LevelName, input.CustomerName, input.CustomerPhone, input.Remark, input.Fulfillment, input.OrderType, settlementModeSnapshot, input.DinerCount, initialStatus, businessDate, nullableID(pickupSequence), pickupCode, nullableID(fastFoodPlate.ID), fastFoodPlate.PublicID, fastFoodPlate.Name, fastFoodPlate.PlateCode, nullableID(table.ID), table.PublicID, table.AreaName, table.Name, table.TableCode, total, merchandiseSubtotal, memberDiscountTotal, nullableID(appliedPromotion.ID), appliedPromotion.Name, appliedPromotion.DiscountCents, nullableID(appliedCoupon.CampaignID), appliedCoupon.Name, appliedCoupon.Discount)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			_ = tx.Rollback()
			if e := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM orders WHERE tenant_id=? AND store_id=? AND idempotency_key=?", store.TenantID, store.ID, idempotencyKey).Scan(&existingID, &existingFingerprint); e == nil {
				if existingFingerprint != "" && existingFingerprint != fingerprint && (legacyFingerprint == "" || existingFingerprint != legacyFingerprint) {
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
	if appliedCoupon.RecordID > 0 {
		couponUpdate, updateErr := tx.ExecContext(r.Context(), `UPDATE customer_coupons SET status='RESERVED',order_id=?
			WHERE id=? AND tenant_id=? AND status='PROVISIONAL' AND order_id IS NULL`, orderID, appliedCoupon.RecordID, store.TenantID)
		if updateErr != nil {
			handleSQLError(w, updateErr)
			return
		}
		if changed, _ := couponUpdate.RowsAffected(); changed != 1 {
			writeError(w, http.StatusConflict, "COUPON_NOT_AVAILABLE", "coupon was used by another order")
			return
		}
	}
	for _, row := range resolved {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO order_items(tenant_id,order_id,addition_sequence,product_id,sku_id,product_name,sku_name,product_type,base_price_cents,modifier_price_cents,original_unit_price_cents,member_discount_cents,member_level_id_snapshot,member_level_name_snapshot,attributes_json,configuration_json,item_remark,unit_price_cents,quantity,subtotal_cents) VALUES(?,?,1,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, store.TenantID, orderID, row.productID, row.skuID, row.productName, row.skuName, row.productType, row.basePrice, row.modifierPrice, row.originalPrice, row.memberDiscount, nullableID(memberBenefit.LevelID), memberBenefit.LevelName, row.attrs, row.configuration, row.itemRemark, row.unitPrice, row.quantity, row.unitPrice*int64(row.quantity))
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = enqueuePrintOutboxWith(r.Context(), tx, store.TenantID, store.ID, orderID, "ORDER_CREATED", orderCreatedPrintDedupeKey(orderID), cashierPrintActorID, ""); err != nil {
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
// treating a client-provided identifier as authentication. A signed customer
// session wins when present; otherwise an install-scoped guest key groups
// anonymous orders but never grants member pricing or balance access.
func upsertPublicOrderCustomer(ctx context.Context, tx *sql.Tx, store storeDTO, input publicOrderInput) (int64, error) {
	if input.VerifiedCustomerID > 0 {
		var customerID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM customers WHERE tenant_id=? AND id=?
			AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE`, store.TenantID, input.VerifiedCustomerID).Scan(&customerID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE customers SET
			source_store_id=COALESCE(source_store_id,?),name=IF(?='',name,?),phone=IF(?='',phone,?),
			source='MINIPROGRAM',last_seen_at=NOW(3) WHERE tenant_id=? AND id=?`,
			store.ID, input.CustomerName, input.CustomerName, input.CustomerPhone, input.CustomerPhone,
			store.TenantID, customerID); err != nil {
			return 0, err
		}
		if err := ensureAutoMembershipTx(ctx, tx, store.TenantID, customerID); err != nil {
			return 0, err
		}
		return customerID, nil
	}
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
	customerID, err := result.LastInsertId()
	if err != nil || customerID <= 0 {
		return 0, err
	}
	if err = ensureAutoMembershipTx(ctx, tx, store.TenantID, customerID); err != nil {
		return 0, err
	}
	return customerID, nil
}

func ensureAutoMembershipTx(ctx context.Context, tx *sql.Tx, tenantID, customerID int64) error {
	var enabled, autoEnroll bool
	var levelID sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT enabled,auto_enroll,default_level_id
		FROM membership_settings WHERE tenant_id=?`, tenantID).Scan(&enabled, &autoEnroll, &levelID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !enabled || !autoEnroll || !levelID.Valid {
		return nil
	}
	var validDays int
	var acquireType string
	if err = tx.QueryRowContext(ctx, `SELECT valid_days,acquire_type FROM member_levels
		WHERE tenant_id=? AND id=? AND status='ACTIVE' AND deleted_at IS NULL`, tenantID, levelID.Int64).Scan(&validDays, &acquireType); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	var currentLevel any
	var expires any
	if acquireType == "FREE" {
		currentLevel = levelID.Int64
	}
	if currentLevel != nil && validDays > 0 {
		expires = time.Now().AddDate(0, 0, validDays)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO members(tenant_id,customer_id,member_no,current_level_id,expires_at)
		VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE customer_id=VALUES(customer_id)`,
		tenantID, customerID, newBusinessNo("MB"), currentLevel, expires)
	return err
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
	var settlementMode string
	var payAfterOnlinePaymentEnabled bool
	if err := s.DB.QueryRowContext(r.Context(), `SELECT o.tenant_id,o.id,o.settlement_mode_snapshot,
		COALESCE(os.pay_after_online_payment_enabled,1) FROM orders o
		JOIN tenants t ON t.id=o.tenant_id AND t.status='ACTIVE'
			AND (t.service_expires_at IS NULL OR t.service_expires_at >= CURRENT_DATE) AND t.deleted_at IS NULL
		JOIN stores st ON st.id=o.store_id AND st.status='ACTIVE' AND st.deleted_at IS NULL
		LEFT JOIN store_operation_settings os ON os.tenant_id=o.tenant_id AND os.store_id=o.store_id
		WHERE o.order_no=?`, orderNo).Scan(&tenantID, &id, &settlementMode, &payAfterOnlinePaymentEnabled); err != nil {
		handleSQLError(w, err)
		return
	}
	if settlementMode == "PAY_AFTER" && !payAfterOnlinePaymentEnabled {
		writeError(w, http.StatusConflict, "ONLINE_PAYMENT_DISABLED", "该门店已关闭餐后线上支付，请到收银台结账")
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
	var session publicCustomerSession
	if current, ok := s.optionalPublicCustomerSession(r.Context(), r, tenantID); ok {
		session = current
	}
	s.createPaymentForOrder(w, r, tenantID, id, paymentInput{
		OpenID: session.OpenID, SubAppID: requested.SubAppID, CustomerID: session.CustomerID,
	})
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

func (s *Server) publicCustomerOrders(w http.ResponseWriter, r *http.Request) {
	storeCode := strings.TrimSpace(r.URL.Query().Get("storeCode"))
	if storeCode == "" {
		writeError(w, http.StatusBadRequest, "STORE_CODE_REQUIRED", "storeCode is required")
		return
	}
	store, err := s.findPublicStore(r.Context(), storeCode)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	session, ok := s.requirePublicCustomerSession(w, r, store.TenantID)
	if !ok {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id FROM orders WHERE tenant_id=? AND store_id=? AND customer_id=?
		ORDER BY id DESC LIMIT 100`, store.TenantID, store.ID, session.CustomerID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	orderIDs := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			handleSQLError(w, err)
			return
		}
		orderIDs = append(orderIDs, id)
	}
	orders := make([]map[string]any, 0, len(orderIDs))
	for _, id := range orderIDs {
		order, loadErr := s.loadOrder(r.Context(), store.TenantID, id, "")
		if loadErr != nil {
			handleSQLError(w, loadErr)
			return
		}
		orders = append(orders, publicOrderView(order))
	}
	writeData(w, http.StatusOK, orders)
}

func publicOrderView(order orderDTO) map[string]any {
	items := make([]map[string]any, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, map[string]any{"id": item.ID, "productId": item.ProductID, "skuId": item.SKUID, "name": item.ProductName, "skuName": item.SKUName, "configuration": item.Configuration, "itemRemark": item.ItemRemark, "price": item.UnitPriceCents, "originalPrice": item.OriginalCents, "memberDiscount": item.MemberDiscount, "memberLevelName": item.MemberLevel, "additionSequence": item.Addition, "quantity": item.Quantity, "amount": item.SubtotalCents})
	}
	paymentStatus := order.PaymentStatus
	if paymentStatus == "PAID" {
		paymentStatus = "SUCCEEDED"
	}
	view := map[string]any{"id": order.ID, "orderNo": order.OrderNo, "pickupCode": order.PickupCode, "businessDate": order.BusinessDate, "status": order.Status, "paymentStatus": paymentStatus, "settlementMode": order.SettlementMode, "additionCount": order.AdditionCount, "dinerCount": order.DinerCount, "memberLevelName": order.MemberLevel, "memberDiscount": order.MemberDiscount, "canAddItems": canChangeDineInItems(order), "fulfillmentType": order.Fulfillment, "orderType": order.OrderType, "orderScene": order.OrderType, "order_scene": order.OrderType, "remark": order.Remark, "amount": order.TotalCents, "paidAmount": order.PaidCents, "remainingAmount": order.RemainingCents, "refundedAmount": order.RefundedCents, "createdAt": order.CreatedAt, "items": items}
	if order.FastFoodPlate != nil {
		view["fastFoodPlate"] = map[string]any{"publicId": order.FastFoodPlate.PublicID, "plateName": order.FastFoodPlate.Name, "plateCode": order.FastFoodPlate.PlateCode}
		view["fastFoodPlatePublicId"] = order.FastFoodPlate.PublicID
		view["fastFoodPlateName"] = order.FastFoodPlate.Name
		view["fastFoodPlateCode"] = order.FastFoodPlate.PlateCode
	}
	if order.Table != nil {
		view["table"] = map[string]any{"publicId": order.Table.PublicID, "name": order.Table.Name, "areaName": order.Table.AreaName, "tableCode": order.Table.TableCode}
		view["tablePublicId"] = order.Table.PublicID
		view["tableName"] = order.Table.Name
		view["tableCode"] = order.Table.TableCode
		view["tableAreaName"] = order.Table.AreaName
	}
	return view
}
