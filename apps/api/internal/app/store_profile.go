package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const maxStoreProfileImages = 6

type merchantStoreProfile struct {
	StoreID              int64    `json:"storeId"`
	VisibleInMiniapp     bool     `json:"visibleInMiniapp"`
	ContactName          string   `json:"contactName"`
	Region               string   `json:"region"`
	MainProducts         string   `json:"mainProducts"`
	AverageSpendCents    int64    `json:"averageSpendCents"`
	ServiceChannels      []string `json:"serviceChannels"`
	EnvironmentImageURLs []string `json:"environmentImageUrls"`
	FoodSafetyImageURLs  []string `json:"foodSafetyImageUrls"`
	StoreLatitude        *float64 `json:"storeLatitude"`
	StoreLongitude       *float64 `json:"storeLongitude"`
}

func defaultMerchantStoreProfile(storeID int64) merchantStoreProfile {
	return merchantStoreProfile{
		StoreID:              storeID,
		VisibleInMiniapp:     true,
		ServiceChannels:      []string{"DINE_IN", "TAKEOUT"},
		EnvironmentImageURLs: []string{},
		FoodSafetyImageURLs:  []string{},
	}
}

func normalizeStoreProfile(input merchantStoreProfile) (merchantStoreProfile, error) {
	input.ContactName = strings.TrimSpace(input.ContactName)
	input.Region = strings.TrimSpace(input.Region)
	input.MainProducts = strings.TrimSpace(input.MainProducts)
	if len([]rune(input.ContactName)) > 80 {
		return input, errors.New("contactName must not exceed 80 characters")
	}
	if len([]rune(input.Region)) > 120 {
		return input, errors.New("region must not exceed 120 characters")
	}
	if len([]rune(input.MainProducts)) > 255 {
		return input, errors.New("mainProducts must not exceed 255 characters")
	}
	if input.AverageSpendCents < 0 || input.AverageSpendCents > 100000000 {
		return input, errors.New("averageSpendCents must be between 0 and 100000000")
	}
	if (input.StoreLatitude == nil) != (input.StoreLongitude == nil) {
		return input, errors.New("storeLatitude and storeLongitude must be provided together")
	}
	if input.StoreLatitude != nil && !validCoordinate(input.StoreLatitude, input.StoreLongitude) {
		return input, errors.New("store coordinates are invalid")
	}
	channels, err := normalizeStoreProfileStrings(input.ServiceChannels, 3, func(value string) bool {
		return value == "DINE_IN" || value == "TAKEOUT" || value == "DELIVERY"
	})
	if err != nil || len(channels) == 0 {
		return input, errors.New("serviceChannels must contain at least one supported channel")
	}
	input.ServiceChannels = channels
	if input.EnvironmentImageURLs, err = normalizeStoreProfileStrings(input.EnvironmentImageURLs, maxStoreProfileImages, func(value string) bool { return len(value) <= 1024 }); err != nil {
		return input, errors.New("environmentImageUrls contains too many or invalid images")
	}
	if input.FoodSafetyImageURLs, err = normalizeStoreProfileStrings(input.FoodSafetyImageURLs, maxStoreProfileImages, func(value string) bool { return len(value) <= 1024 }); err != nil {
		return input, errors.New("foodSafetyImageUrls contains too many or invalid images")
	}
	return input, nil
}

func normalizeStoreProfileStrings(values []string, limit int, allowed func(string) bool) ([]string, error) {
	if len(values) > limit {
		return nil, errors.New("too many values")
	}
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || seen[value] || !allowed(value) {
			return nil, errors.New("invalid or duplicate value")
		}
		seen[value] = true
		result = append(result, value)
	}
	return result, nil
}

func decodeStringList(raw string, fallback []string) []string {
	var values []string
	if json.Unmarshal([]byte(raw), &values) != nil || values == nil {
		return fallback
	}
	return values
}

func (s *Server) ensureMerchantStoreProfile(r *http.Request, tenantID, storeID int64) error {
	_, err := s.DB.ExecContext(r.Context(), `INSERT IGNORE INTO store_profiles(
		store_id,tenant_id,visible_in_miniapp,contact_name,region,main_products,average_spend_cents,
		service_channels_json,environment_image_urls_json,food_safety_image_urls_json
	) SELECT s.id,s.tenant_id,1,t.contact_name,'','',0,'["DINE_IN","TAKEOUT"]','[]','[]'
		FROM stores s JOIN tenants t ON t.id=s.tenant_id
		WHERE s.id=? AND s.tenant_id=? AND s.deleted_at IS NULL`, storeID, tenantID)
	return err
}

func (s *Server) getMerchantStoreProfile(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if err = s.ensureMerchantStoreProfile(r, actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	item := defaultMerchantStoreProfile(storeID)
	var channelsJSON, environmentJSON, foodSafetyJSON string
	err = s.DB.QueryRowContext(r.Context(), `SELECT visible_in_miniapp,contact_name,region,main_products,average_spend_cents,
		service_channels_json,environment_image_urls_json,food_safety_image_urls_json
		FROM store_profiles WHERE tenant_id=? AND store_id=?`, actor.TenantID, storeID).
		Scan(&item.VisibleInMiniapp, &item.ContactName, &item.Region, &item.MainProducts, &item.AverageSpendCents,
			&channelsJSON, &environmentJSON, &foodSafetyJSON)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	item.ServiceChannels = decodeStringList(channelsJSON, []string{"DINE_IN", "TAKEOUT"})
	item.EnvironmentImageURLs = decodeStringList(environmentJSON, []string{})
	item.FoodSafetyImageURLs = decodeStringList(foodSafetyJSON, []string{})
	var latitude, longitude sql.NullFloat64
	err = s.DB.QueryRowContext(r.Context(), `SELECT store_latitude,store_longitude FROM store_operation_settings WHERE tenant_id=? AND store_id=?`, actor.TenantID, storeID).Scan(&latitude, &longitude)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	if latitude.Valid && longitude.Valid {
		item.StoreLatitude = &latitude.Float64
		item.StoreLongitude = &longitude.Float64
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updateMerchantStoreProfile(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input merchantStoreProfile
	if !decodeJSON(w, r, &input) {
		return
	}
	input.StoreID = storeID
	input, err = normalizeStoreProfile(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	channelsJSON, _ := json.Marshal(input.ServiceChannels)
	environmentJSON, _ := json.Marshal(input.EnvironmentImageURLs)
	foodSafetyJSON, _ := json.Marshal(input.FoodSafetyImageURLs)
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if err = lockDecorationStore(r.Context(), tx, actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	for _, rawURL := range append(append([]string{}, input.EnvironmentImageURLs...), input.FoodSafetyImageURLs...) {
		if err = s.validateManagedMediaURL(r.Context(), tx, actor.TenantID, storeID, rawURL); err != nil {
			writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
			return
		}
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO store_profiles(
		store_id,tenant_id,visible_in_miniapp,contact_name,region,main_products,average_spend_cents,
		service_channels_json,environment_image_urls_json,food_safety_image_urls_json
	) VALUES(?,?,?,?,?,?,?,?,?,?)
	ON DUPLICATE KEY UPDATE visible_in_miniapp=VALUES(visible_in_miniapp),contact_name=VALUES(contact_name),
		region=VALUES(region),main_products=VALUES(main_products),average_spend_cents=VALUES(average_spend_cents),
		service_channels_json=VALUES(service_channels_json),environment_image_urls_json=VALUES(environment_image_urls_json),
		food_safety_image_urls_json=VALUES(food_safety_image_urls_json)`,
		storeID, actor.TenantID, input.VisibleInMiniapp, input.ContactName, input.Region, input.MainProducts,
		input.AverageSpendCents, string(channelsJSON), string(environmentJSON), string(foodSafetyJSON))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO store_operation_settings(
		store_id,tenant_id,settlement_mode,ordering_mode,store_latitude,store_longitude,privacy_policy_text,user_agreement_text,official_account_events_json
	) VALUES(?,?,'PAY_BEFORE','MULTI_PERSON',?,?,'','','["ORDER_PAID","REFUND_CREATED","PRINT_FAILED"]')
	ON DUPLICATE KEY UPDATE store_latitude=VALUES(store_latitude),store_longitude=VALUES(store_longitude)`,
		storeID, actor.TenantID, nullableFloat64(input.StoreLatitude), nullableFloat64(input.StoreLongitude))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "merchant.store_profile.update", "store", int64String(storeID), input, r)
	s.getMerchantStoreProfile(w, r)
}
