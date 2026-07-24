package app

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type tenantDTO struct {
	ID                     int64  `json:"id"`
	Code                   string `json:"code"`
	Name                   string `json:"name"`
	ContactName            string `json:"contact_name"`
	ContactPhone           string `json:"contact_phone"`
	Status                 string `json:"status"`
	ServiceExpiresAt       string `json:"service_expires_at"`
	ServiceExpired         bool   `json:"service_expired"`
	PaymentProvider        string `json:"payment_provider"`
	PaymentMerchantNo      string `json:"payment_merchant_no"`
	PaymentSubAppID        string `json:"payment_sub_appid"`
	BusinessLicenseURL     string `json:"business_license_url"`
	FoodBusinessLicenseURL string `json:"food_business_license_url"`
	StoreID                int64  `json:"store_id"`
	StoreCode              string `json:"store_code"`
	StoreName              string `json:"store_name"`
	OrderCount             int    `json:"order_count"`
	OwnerUsername          string `json:"owner_username"`
	OwnerDisplayName       string `json:"owner_display_name"`
	OwnerStatus            string `json:"owner_status"`
	HasOwner               bool   `json:"has_owner"`
	CreatedAt              string `json:"created_at"`
}

type tenantInput struct {
	Code              string `json:"code"`
	Name              string `json:"name"`
	ContactName       string `json:"contact_name"`
	ContactPhone      string `json:"contact_phone"`
	Status            string `json:"status"`
	ServiceExpiresAt  string `json:"service_expires_at"`
	PaymentProvider   string `json:"payment_provider"`
	PaymentMerchantNo string `json:"payment_merchant_no"`
	PaymentSubAppID   string `json:"payment_sub_appid"`
	OwnerUsername     string `json:"owner_username"`
	OwnerPassword     string `json:"owner_password"`
	OwnerDisplayName  string `json:"owner_display_name"`
	OwnerAccountMode  string `json:"owner_account_mode"`
	InitialStoreCode  string `json:"initial_store_code"`
	InitialStoreName  string `json:"initial_store_name"`
}

type tenantOwnerInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	AccountMode string `json:"account_mode"`
}

type tenantServiceExpirationInput struct {
	ExpiresAt string `json:"expires_at"`
}

type tenantPaymentSettings struct {
	Provider                   string `json:"provider"`
	MerchantNo                 string `json:"merchantNo"`
	SubAppID                   string `json:"subAppId"`
	OnboardingStatus           string `json:"onboardingStatus"`
	ProductAuthorizationStatus string `json:"productAuthorizationStatus"`
	RefundAuthorized           bool   `json:"refundAuthorized"`
}

type storeDTO struct {
	ID            int64  `json:"id"`
	TenantID      int64  `json:"tenant_id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	LogoURL       string `json:"logo_url"`
	BannerURL     string `json:"banner_url"`
	Address       string `json:"address"`
	Phone         string `json:"phone"`
	BusinessHours string `json:"business_hours"`
	Notice        string `json:"notice"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

type userDTO struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type userInput struct {
	TenantID    int64  `json:"tenant_id"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

func (s *Server) platformRoutes(r chi.Router) {
	r.Use(requireRoles(RolePlatformAdmin, RolePlatformOperator))
	r.Get("/dashboard", s.platformDashboard)
	r.Get("/settings/payment", s.getPlatformPaymentSettings)
	r.With(requireRoles(RolePlatformAdmin)).Put("/settings/payment", s.updatePlatformPaymentSettings)
	r.Get("/settings/system", s.getPlatformSystemSettings)
	r.With(requireRoles(RolePlatformAdmin)).Put("/settings/system", s.updatePlatformSystemSettings)
	r.Get("/settings/printer-providers", s.getPlatformPrinterProviders)
	r.With(requireRoles(RolePlatformAdmin)).Put("/settings/printer-providers/xpyun", s.updatePlatformXPYun)
	r.With(requireRoles(RolePlatformAdmin)).Post("/settings/printer-providers/xpyun/test", s.testPlatformXPYun)
	r.Get("/tenants", s.listTenants)
	r.Get("/announcements", s.listPlatformAnnouncements)
	r.Get("/announcements/{announcementID}", s.getPlatformAnnouncement)
	r.With(requireRoles(RolePlatformAdmin)).Post("/announcements", s.createPlatformAnnouncement)
	r.With(requireRoles(RolePlatformAdmin)).Put("/announcements/{announcementID}", s.updatePlatformAnnouncement)
	r.With(requireRoles(RolePlatformAdmin)).Post("/announcements/{announcementID}/publish", s.publishPlatformAnnouncement)
	r.With(requireRoles(RolePlatformAdmin)).Post("/announcements/{announcementID}/withdraw", s.withdrawPlatformAnnouncement)
	r.With(requireRoles(RolePlatformAdmin)).Post("/tenants", s.createTenant)
	r.Route("/tenants/{tenantID}", func(t chi.Router) {
		t.Get("/", s.getTenant)
		t.With(requireRoles(RolePlatformAdmin)).Put("/", s.updateTenant)
		t.With(requireRoles(RolePlatformAdmin)).Put("/service-expiration", s.updateTenantServiceExpiration)
		t.Get("/payment-settings", s.getTenantPaymentSettings)
		t.With(requireRoles(RolePlatformAdmin)).Put("/payment-settings", s.updateTenantPaymentSettings)
		t.With(requireRoles(RolePlatformAdmin)).Post("/renew-one-year", s.renewTenantOneYear)
		t.With(requireRoles(RolePlatformAdmin)).Delete("/", s.deleteTenant)
		t.With(requireRoles(RolePlatformAdmin)).Post("/owner", s.createTenantOwner)
		t.With(requireRoles(RolePlatformAdmin)).Post("/documents/{documentType}", s.uploadTenantDocument)
	})
	r.Group(func(admin chi.Router) {
		admin.Use(requireRoles(RolePlatformAdmin))
		admin.Get("/users", s.listPlatformUsers)
		admin.Post("/users", s.createPlatformUser)
		admin.Get("/users/{userID}", s.getPlatformUser)
		admin.Put("/users/{userID}", s.updatePlatformUser)
		admin.Delete("/users/{userID}", s.deletePlatformUser)
	})
	r.Get("/audit-logs", s.listAuditLogs)
}

func (s *Server) listTenants(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	search := "%" + strings.TrimSpace(r.URL.Query().Get("q")) + "%"
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !validStatus(status, "ACTIVE", "PENDING", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE, PENDING or DISABLED")
		return
	}
	var total int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenants WHERE deleted_at IS NULL AND (name LIKE ? OR code LIKE ?) AND (?='' OR status=?)`, search, search, status, status).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT t.id,t.code,t.name,t.contact_name,t.contact_phone,t.status,t.payment_provider,t.payment_merchant_no,t.payment_sub_appid,
		COALESCE((SELECT a.url FROM media_assets a WHERE a.id=t.business_license_media_id AND a.tenant_id=t.id AND a.kind='TENANT_DOCUMENT' AND a.status='ACTIVE' AND a.deleted_at IS NULL),''),
		COALESCE((SELECT a.url FROM media_assets a WHERE a.id=t.food_business_license_media_id AND a.tenant_id=t.id AND a.kind='TENANT_DOCUMENT' AND a.status='ACTIVE' AND a.deleted_at IS NULL),''),
		COALESCE((SELECT s.id FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL ORDER BY s.id LIMIT 1),0),
		COALESCE((SELECT s.code FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL ORDER BY s.id LIMIT 1),''),
		COALESCE((SELECT s.name FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL ORDER BY s.id LIMIT 1),''),
		(SELECT COUNT(*) FROM orders o WHERE o.tenant_id=t.id),
		COALESCE((SELECT a.username FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL WHERE m.tenant_id=t.id AND m.role=? AND m.deleted_at IS NULL ORDER BY m.id LIMIT 1),''),
		COALESCE((SELECT a.display_name FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL WHERE m.tenant_id=t.id AND m.role=? AND m.deleted_at IS NULL ORDER BY m.id LIMIT 1),''),
		COALESCE((SELECT m.status FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL WHERE m.tenant_id=t.id AND m.role=? AND m.deleted_at IS NULL ORDER BY m.id LIMIT 1),''),
		EXISTS(SELECT 1 FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL WHERE m.tenant_id=t.id AND m.role=? AND m.deleted_at IS NULL),
		DATE_FORMAT(t.created_at,'%Y-%m-%d %H:%i:%s'),COALESCE(DATE_FORMAT(t.service_expires_at,'%Y-%m-%d'),''),
		(t.service_expires_at IS NOT NULL AND t.service_expires_at < CURRENT_DATE)
		FROM tenants t WHERE t.deleted_at IS NULL AND (t.name LIKE ? OR t.code LIKE ?) AND (?='' OR t.status=?) ORDER BY t.id DESC LIMIT ? OFFSET ?`, RoleMerchantOwner, RoleMerchantOwner, RoleMerchantOwner, RoleMerchantOwner, search, search, status, status, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []tenantDTO{}
	for rows.Next() {
		var item tenantDTO
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.ContactName, &item.ContactPhone, &item.Status, &item.PaymentProvider, &item.PaymentMerchantNo, &item.PaymentSubAppID, &item.BusinessLicenseURL, &item.FoodBusinessLicenseURL, &item.StoreID, &item.StoreCode, &item.StoreName, &item.OrderCount, &item.OwnerUsername, &item.OwnerDisplayName, &item.OwnerStatus, &item.HasOwner, &item.CreatedAt, &item.ServiceExpiresAt, &item.ServiceExpired); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request) {
	var input tenantInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "code and name are required")
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	input.Status = strings.ToUpper(input.Status)
	if !validStatus(input.Status, "ACTIVE", "PENDING", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE, PENDING or DISABLED")
		return
	}
	if input.PaymentProvider == "" {
		input.PaymentProvider = "mock"
	}
	input.PaymentProvider = strings.ToLower(input.PaymentProvider)
	if !validPaymentProvider(input.PaymentProvider) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "payment_provider must be mock, tianque or wechat_partner")
		return
	}
	input.OwnerUsername = strings.TrimSpace(input.OwnerUsername)
	input.OwnerDisplayName = strings.TrimSpace(input.OwnerDisplayName)
	input.OwnerAccountMode = strings.ToUpper(strings.TrimSpace(input.OwnerAccountMode))
	if input.OwnerAccountMode == "" {
		input.OwnerAccountMode = "CREATE"
	}
	input.InitialStoreCode = strings.TrimSpace(input.InitialStoreCode)
	input.InitialStoreName = strings.TrimSpace(input.InitialStoreName)
	if input.OwnerUsername == "" || len(input.OwnerUsername) > 64 || !validStatus(input.OwnerAccountMode, "CREATE", "EXISTING") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "owner_username and owner_account_mode CREATE or EXISTING are required")
		return
	}
	if input.OwnerAccountMode == "CREATE" && (len([]byte(input.OwnerPassword)) < 8 || len([]byte(input.OwnerPassword)) > 72 || input.OwnerDisplayName == "") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "a new owner requires owner_display_name and an owner_password of 8 to 72 bytes")
		return
	}
	if input.InitialStoreCode == "" || input.InitialStoreName == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "initial_store_code and initial_store_name are required")
		return
	}
	if input.ServiceExpiresAt != "" {
		if _, err := time.Parse("2006-01-02", input.ServiceExpiresAt); err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "service_expires_at must use YYYY-MM-DD")
			return
		}
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `INSERT INTO tenants(code,name,contact_name,contact_phone,status,service_expires_at,payment_provider,payment_merchant_no,payment_sub_appid)
		VALUES(?,?,?,?,?,NULLIF(?,''),?,?,?)`, input.Code, input.Name, input.ContactName, input.ContactPhone, strings.ToUpper(input.Status), input.ServiceExpiresAt, strings.ToLower(input.PaymentProvider), input.PaymentMerchantNo, input.PaymentSubAppID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	storeResult, err := tx.ExecContext(r.Context(), `INSERT INTO stores(tenant_id,code,name,status) VALUES(?,?,?,'ACTIVE')`, id, input.InitialStoreCode, input.InitialStoreName)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	storeID, _ := storeResult.LastInsertId()
	if err = seedStoreBusinessPeriods(r.Context(), tx, id, storeID, ""); err != nil {
		handleSQLError(w, err)
		return
	}
	var ownerAccountID int64
	if input.OwnerAccountMode == "EXISTING" {
		if err = tx.QueryRowContext(r.Context(), `SELECT id FROM accounts WHERE username=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE`, input.OwnerUsername).Scan(&ownerAccountID); err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "OWNER_ACCOUNT_NOT_FOUND", "existing owner account not found")
			} else {
				handleSQLError(w, err)
			}
			return
		}
		var incompatible int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE account_id=? AND role<>? AND deleted_at IS NULL`, ownerAccountID, RoleMerchantOwner).Scan(&incompatible); err != nil {
			handleSQLError(w, err)
			return
		}
		if incompatible > 0 {
			writeError(w, http.StatusConflict, "OWNER_ACCOUNT_INCOMPATIBLE", "staff account cannot be linked as a multi-store owner")
			return
		}
	} else {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(input.OwnerPassword), bcrypt.DefaultCost)
		if hashErr != nil {
			handleSQLError(w, hashErr)
			return
		}
		ownerResult, insertErr := tx.ExecContext(r.Context(), `INSERT INTO accounts(username,password_hash,display_name,status) VALUES(?,?,?,'ACTIVE')`, input.OwnerUsername, string(hash), input.OwnerDisplayName)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		ownerAccountID, _ = ownerResult.LastInsertId()
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO tenant_memberships(tenant_id,account_id,role,status) VALUES(?,?,?,'ACTIVE')`, id, ownerAccountID, RoleMerchantOwner); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "tenant.create", "tenant", int64String(id), map[string]any{
		"code": input.Code, "name": input.Name, "payment_provider": input.PaymentProvider,
		"owner_username": input.OwnerUsername, "owner_account_mode": input.OwnerAccountMode, "initial_store_code": input.InitialStoreCode,
	}, r)
	s.getTenantByID(w, r, id)
}

func (s *Server) getTenant(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tenantID")
	if ok {
		s.getTenantByID(w, r, id)
	}
}

func (s *Server) getTenantByID(w http.ResponseWriter, r *http.Request, id int64) {
	var item tenantDTO
	err := s.DB.QueryRowContext(r.Context(), `SELECT t.id,t.code,t.name,t.contact_name,t.contact_phone,t.status,t.payment_provider,t.payment_merchant_no,t.payment_sub_appid,
		COALESCE((SELECT a.url FROM media_assets a WHERE a.id=t.business_license_media_id AND a.tenant_id=t.id AND a.kind='TENANT_DOCUMENT' AND a.status='ACTIVE' AND a.deleted_at IS NULL),''),
		COALESCE((SELECT a.url FROM media_assets a WHERE a.id=t.food_business_license_media_id AND a.tenant_id=t.id AND a.kind='TENANT_DOCUMENT' AND a.status='ACTIVE' AND a.deleted_at IS NULL),''),
		COALESCE((SELECT s.id FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL ORDER BY s.id LIMIT 1),0),
		COALESCE((SELECT s.code FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL ORDER BY s.id LIMIT 1),''),
		COALESCE((SELECT s.name FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL ORDER BY s.id LIMIT 1),''),
		(SELECT COUNT(*) FROM orders o WHERE o.tenant_id=t.id),
		COALESCE((SELECT a.username FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL WHERE m.tenant_id=t.id AND m.role=? AND m.deleted_at IS NULL ORDER BY m.id LIMIT 1),''),
		COALESCE((SELECT a.display_name FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL WHERE m.tenant_id=t.id AND m.role=? AND m.deleted_at IS NULL ORDER BY m.id LIMIT 1),''),
		COALESCE((SELECT m.status FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL WHERE m.tenant_id=t.id AND m.role=? AND m.deleted_at IS NULL ORDER BY m.id LIMIT 1),''),
		EXISTS(SELECT 1 FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL WHERE m.tenant_id=t.id AND m.role=? AND m.deleted_at IS NULL),
		DATE_FORMAT(t.created_at,'%Y-%m-%d %H:%i:%s'),COALESCE(DATE_FORMAT(t.service_expires_at,'%Y-%m-%d'),''),
		(t.service_expires_at IS NOT NULL AND t.service_expires_at < CURRENT_DATE)
		FROM tenants t WHERE t.id=? AND t.deleted_at IS NULL`, RoleMerchantOwner, RoleMerchantOwner, RoleMerchantOwner, RoleMerchantOwner, id).
		Scan(&item.ID, &item.Code, &item.Name, &item.ContactName, &item.ContactPhone, &item.Status, &item.PaymentProvider, &item.PaymentMerchantNo, &item.PaymentSubAppID, &item.BusinessLicenseURL, &item.FoodBusinessLicenseURL, &item.StoreID, &item.StoreCode, &item.StoreName, &item.OrderCount, &item.OwnerUsername, &item.OwnerDisplayName, &item.OwnerStatus, &item.HasOwner, &item.CreatedAt, &item.ServiceExpiresAt, &item.ServiceExpired)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updateTenant(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	var input tenantInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	input.Status = strings.ToUpper(input.Status)
	if !validStatus(input.Status, "ACTIVE", "PENDING", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE, PENDING or DISABLED")
		return
	}
	input.PaymentProvider = strings.ToLower(strings.TrimSpace(input.PaymentProvider))
	if !validPaymentProvider(input.PaymentProvider) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "payment_provider must be mock, tianque or wechat_partner")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE tenants SET code=?,name=?,contact_name=?,contact_phone=?,status=?,payment_provider=?,payment_merchant_no=?,payment_sub_appid=?
		WHERE id=? AND deleted_at IS NULL`, input.Code, input.Name, input.ContactName, input.ContactPhone, strings.ToUpper(input.Status), strings.ToLower(input.PaymentProvider), input.PaymentMerchantNo, input.PaymentSubAppID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "tenant not found")
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "tenant.update", "tenant", int64String(id), map[string]any{
		"code": input.Code, "name": input.Name, "status": input.Status,
		"payment_provider": input.PaymentProvider, "payment_merchant_no_configured": input.PaymentMerchantNo != "",
	}, r)
	s.getTenantByID(w, r, id)
}

func validPaymentProvider(value string) bool {
	return value == "mock" || value == "tianque" || value == "wechat_partner"
}

func (s *Server) getTenantPaymentSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	var settings tenantPaymentSettings
	err := s.DB.QueryRowContext(r.Context(), `SELECT payment_provider,payment_merchant_no,payment_sub_appid,
		payment_onboarding_status,payment_product_authorization_status,payment_refund_authorized
		FROM tenants WHERE id=? AND deleted_at IS NULL`, id).
		Scan(&settings.Provider, &settings.MerchantNo, &settings.SubAppID, &settings.OnboardingStatus, &settings.ProductAuthorizationStatus, &settings.RefundAuthorized)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, settings)
}

func (s *Server) updateTenantPaymentSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	var input tenantPaymentSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	input.MerchantNo = strings.TrimSpace(input.MerchantNo)
	input.SubAppID = strings.TrimSpace(input.SubAppID)
	input.OnboardingStatus = strings.ToUpper(strings.TrimSpace(input.OnboardingStatus))
	input.ProductAuthorizationStatus = strings.ToUpper(strings.TrimSpace(input.ProductAuthorizationStatus))
	if !validPaymentProvider(input.Provider) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "provider must be mock, tianque or wechat_partner")
		return
	}
	if !validStatus(input.OnboardingStatus, "NOT_APPLIED", "REVIEWING", "PENDING_SIGNING", "ACTIVE", "REJECTED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid onboarding status")
		return
	}
	if !validStatus(input.ProductAuthorizationStatus, "NOT_AUTHORIZED", "PENDING", "AUTHORIZED", "REVOKED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid product authorization status")
		return
	}
	if input.Provider == "wechat_partner" {
		if input.MerchantNo != "" && (!digitsOnly(input.MerchantNo) || len(input.MerchantNo) < 8 || len(input.MerchantNo) > 32) {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "微信支付特约商户号必须为 8 至 32 位数字")
			return
		}
		if input.OnboardingStatus == "ACTIVE" && input.MerchantNo == "" {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "进件状态为已开通时必须填写微信支付特约商户号")
			return
		}
		if input.SubAppID != "" && (len(input.SubAppID) != 18 || !strings.HasPrefix(input.SubAppID, "wx")) {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "sub_appid 格式不正确")
			return
		}
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE tenants SET payment_provider=?,payment_merchant_no=?,payment_sub_appid=?,
		payment_onboarding_status=?,payment_product_authorization_status=?,payment_refund_authorized=?
		WHERE id=? AND deleted_at IS NULL`, input.Provider, input.MerchantNo, input.SubAppID,
		input.OnboardingStatus, input.ProductAuthorizationStatus, input.RefundAuthorized, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "tenant not found")
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "tenant.payment_settings.update", "tenant", int64String(id), map[string]any{
		"provider": input.Provider, "merchant_no_configured": input.MerchantNo != "",
		"sub_appid_configured": input.SubAppID != "", "onboarding_status": input.OnboardingStatus,
		"product_authorization_status": input.ProductAuthorizationStatus, "refund_authorized": input.RefundAuthorized,
	}, r)
	s.getTenantPaymentSettings(w, r)
}

func digitsOnly(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}

func (s *Server) updateTenantServiceExpiration(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	var input tenantServiceExpirationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ExpiresAt = strings.TrimSpace(input.ExpiresAt)
	if input.ExpiresAt != "" {
		if _, err := time.Parse("2006-01-02", input.ExpiresAt); err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "expires_at must use YYYY-MM-DD")
			return
		}
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE tenants SET service_expires_at=NULLIF(?,'') WHERE id=? AND deleted_at IS NULL`, input.ExpiresAt, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "tenant not found")
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "tenant.service_expiration.update", "tenant", int64String(id), map[string]any{"service_expires_at": input.ExpiresAt}, r)
	s.getTenantByID(w, r, id)
}

func (s *Server) renewTenantOneYear(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE tenants
		SET service_expires_at=DATE_ADD(GREATEST(COALESCE(service_expires_at,CURRENT_DATE),CURRENT_DATE),INTERVAL 1 YEAR)
		WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "tenant not found")
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "tenant.service_expiration.renew_one_year", "tenant", int64String(id), nil, r)
	s.getTenantByID(w, r, id)
}

func (s *Server) createTenantOwner(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	var input tenantOwnerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.AccountMode = strings.ToUpper(strings.TrimSpace(input.AccountMode))
	if input.AccountMode == "" {
		input.AccountMode = "CREATE"
	}
	if input.Username == "" || len(input.Username) > 64 || !validStatus(input.AccountMode, "CREATE", "EXISTING") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "username and account_mode CREATE or EXISTING are required")
		return
	}
	if input.AccountMode == "CREATE" && (input.DisplayName == "" || len([]byte(input.Password)) < 8 || len([]byte(input.Password)) > 72) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "a new owner requires display_name and a password of 8 to 72 bytes")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var tenantExists bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tenants WHERE id=? AND deleted_at IS NULL)`, tenantID).Scan(&tenantExists); err != nil {
		handleSQLError(w, err)
		return
	}
	if !tenantExists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "tenant not found")
		return
	}
	var ownerExists bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id=? AND role=? AND deleted_at IS NULL)`, tenantID, RoleMerchantOwner).Scan(&ownerExists); err != nil {
		handleSQLError(w, err)
		return
	}
	if ownerExists {
		writeError(w, http.StatusConflict, "OWNER_EXISTS", "tenant owner already exists")
		return
	}
	var accountID int64
	if input.AccountMode == "EXISTING" {
		if err = tx.QueryRowContext(r.Context(), `SELECT id,display_name FROM accounts WHERE username=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE`, input.Username).Scan(&accountID, &input.DisplayName); err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "OWNER_ACCOUNT_NOT_FOUND", "existing owner account not found")
			} else {
				handleSQLError(w, err)
			}
			return
		}
		var incompatible int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE account_id=? AND role<>? AND deleted_at IS NULL`, accountID, RoleMerchantOwner).Scan(&incompatible); err != nil {
			handleSQLError(w, err)
			return
		}
		if incompatible > 0 {
			writeError(w, http.StatusConflict, "OWNER_ACCOUNT_INCOMPATIBLE", "staff account cannot be linked as a multi-store owner")
			return
		}
	} else {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			handleSQLError(w, hashErr)
			return
		}
		result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO accounts(username,password_hash,display_name,status) VALUES(?,?,?,'ACTIVE')`, input.Username, string(hash), input.DisplayName)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		accountID, _ = result.LastInsertId()
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO tenant_memberships(tenant_id,account_id,role,status) VALUES(?,?,?,'ACTIVE')`, tenantID, accountID, RoleMerchantOwner); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "tenant.owner.create", "account", int64String(accountID), map[string]any{"tenant_id": tenantID, "username": input.Username, "account_mode": input.AccountMode}, r)
	writeData(w, http.StatusCreated, map[string]any{"id": accountID, "tenant_id": tenantID, "username": input.Username, "display_name": input.DisplayName, "role": RoleMerchantOwner, "status": "ACTIVE"})
}

func (s *Server) deleteTenant(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE tenants SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "tenant not found")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE stores SET status='DISABLED',deleted_at=COALESCE(deleted_at,NOW(3)) WHERE tenant_id=?", id); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE tenant_memberships SET status='DISABLED' WHERE tenant_id=? AND deleted_at IS NULL", id); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "tenant.delete", "tenant", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) listTenantStores(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,tenant_id,code,name,logo_url,banner_url,address,phone,business_hours,notice,status,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s')
		FROM stores WHERE tenant_id=? AND deleted_at IS NULL ORDER BY id DESC`, tenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []storeDTO{}
	for rows.Next() {
		var item storeDTO
		if err := scanStore(rows, &item); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}

type scanner interface{ Scan(...any) error }

func scanStore(row scanner, item *storeDTO) error {
	return row.Scan(&item.ID, &item.TenantID, &item.Code, &item.Name, &item.LogoURL, &item.BannerURL, &item.Address, &item.Phone, &item.BusinessHours, &item.Notice, &item.Status, &item.CreatedAt)
}

func (s *Server) listPlatformUsers(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM accounts WHERE platform_role IS NOT NULL AND deleted_at IS NULL").Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,0,username,display_name,platform_role,status,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s')
		FROM accounts WHERE platform_role IS NOT NULL AND deleted_at IS NULL ORDER BY id DESC LIMIT ? OFFSET ?`, size, offset)
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
	writeList(w, http.StatusOK, items, total, page, size)
}

func validRole(role string, tenantID int64) bool {
	if tenantID == 0 {
		return role == RolePlatformAdmin || role == RolePlatformOperator
	}
	return role == RoleMerchantOwner || role == RoleMerchantManager || role == RoleMerchantStaff
}

func (s *Server) createPlatformUser(w http.ResponseWriter, r *http.Request) {
	var input userInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Role = strings.ToUpper(input.Role)
	if input.Username == "" || len(input.Password) < 8 || input.TenantID != 0 || !validRole(input.Role, 0) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "platform users require tenant_id=0, a platform role, and a password of at least 8 characters")
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
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO accounts(username,password_hash,display_name,platform_role,status) VALUES(?,?,?,?,?)`, input.Username, string(hash), input.DisplayName, input.Role, input.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), currentIdentity(r.Context()), "user.create", "user", int64String(id), map[string]any{"username": input.Username, "role": input.Role}, r)
	s.getUserByScope(w, r, 0, id, true)
}

func (s *Server) getPlatformUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "userID")
	if ok {
		s.getUserByScope(w, r, 0, id, true)
	}
}

func (s *Server) getUserByScope(w http.ResponseWriter, r *http.Request, tenantID, id int64, platform bool) {
	query := `SELECT a.id,m.tenant_id,a.username,a.display_name,m.role,m.status,DATE_FORMAT(m.created_at,'%Y-%m-%d %H:%i:%s')
		FROM tenant_memberships m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL
		WHERE a.id=? AND m.deleted_at IS NULL`
	args := []any{id}
	if platform {
		query = `SELECT id,0,username,display_name,platform_role,status,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s')
			FROM accounts WHERE id=? AND platform_role IS NOT NULL AND deleted_at IS NULL`
	} else {
		query += " AND m.tenant_id=?"
		args = append(args, tenantID)
	}
	var item userDTO
	if err := scanUser(s.DB.QueryRowContext(r.Context(), query, args...), &item); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updatePlatformUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	var input userInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Role = strings.ToUpper(input.Role)
	if input.TenantID != 0 || !validRole(input.Role, input.TenantID) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "role does not match tenant scope")
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	input.Status = strings.ToUpper(input.Status)
	actor := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var targetRole, targetStatus string
	if err = tx.QueryRowContext(r.Context(), "SELECT platform_role,status FROM accounts WHERE id=? AND platform_role IS NOT NULL AND deleted_at IS NULL FOR UPDATE", id).Scan(&targetRole, &targetStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if actor.UserID == id && (input.Role != targetRole || input.Status != targetStatus) {
		writeError(w, http.StatusConflict, "CANNOT_CHANGE_OWN_ACCESS", "current administrator cannot change its own role or status")
		return
	}
	if targetRole == RolePlatformAdmin && targetStatus == "ACTIVE" && (input.Role != RolePlatformAdmin || input.Status != "ACTIVE") {
		rows, countErr := tx.QueryContext(r.Context(), "SELECT id FROM accounts WHERE platform_role=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE", RolePlatformAdmin)
		if countErr != nil {
			handleSQLError(w, countErr)
			return
		}
		activeAdmins := 0
		for rows.Next() {
			activeAdmins++
		}
		rows.Close()
		if activeAdmins <= 1 {
			writeError(w, http.StatusConflict, "LAST_PLATFORM_ADMIN", "at least one active platform administrator is required")
			return
		}
	}
	var result sql.Result
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
		result, err = tx.ExecContext(r.Context(), `UPDATE accounts SET username=?,password_hash=?,display_name=?,platform_role=?,status=? WHERE id=? AND platform_role IS NOT NULL AND deleted_at IS NULL`, input.Username, string(hash), input.DisplayName, input.Role, input.Status, id)
	} else {
		result, err = tx.ExecContext(r.Context(), `UPDATE accounts SET username=?,display_name=?,platform_role=?,status=? WHERE id=? AND platform_role IS NOT NULL AND deleted_at IS NULL`, input.Username, input.DisplayName, input.Role, input.Status, id)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "user.update", "user", int64String(id), map[string]any{"username": input.Username, "role": input.Role}, r)
	s.getUserByScope(w, r, 0, id, true)
}

func (s *Server) deletePlatformUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	if actor.UserID == id {
		writeError(w, http.StatusConflict, "CANNOT_DELETE_SELF", "current administrator cannot delete itself")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var targetRole, targetStatus string
	if err = tx.QueryRowContext(r.Context(), "SELECT platform_role,status FROM accounts WHERE id=? AND platform_role IS NOT NULL AND deleted_at IS NULL FOR UPDATE", id).Scan(&targetRole, &targetStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if targetRole == RolePlatformAdmin && targetStatus == "ACTIVE" {
		rows, countErr := tx.QueryContext(r.Context(), "SELECT id FROM accounts WHERE platform_role=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE", RolePlatformAdmin)
		if countErr != nil {
			handleSQLError(w, countErr)
			return
		}
		activeAdmins := 0
		for rows.Next() {
			activeAdmins++
		}
		rows.Close()
		if activeAdmins <= 1 {
			writeError(w, http.StatusConflict, "LAST_PLATFORM_ADMIN", "at least one active platform administrator is required")
			return
		}
	}
	result, err := tx.ExecContext(r.Context(), "UPDATE accounts SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND platform_role IS NOT NULL AND deleted_at IS NULL", id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "user.delete", "user", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func scanUser(row scanner, item *userDTO) error {
	return row.Scan(&item.ID, &item.TenantID, &item.Username, &item.DisplayName, &item.Role, &item.Status, &item.CreatedAt)
}

func (s *Server) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM audit_logs").Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,tenant_id,actor_user_id,action,resource_type,resource_id,request_id,ip,details_text,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s')
		FROM audit_logs ORDER BY id DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, tenantID, actorID int64
		var action, resourceType, resourceID, reqID, ip, details, created string
		if err := rows.Scan(&id, &tenantID, &actorID, &action, &resourceType, &resourceID, &reqID, &ip, &details, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "tenant_id": tenantID, "actor_user_id": actorID, "action": action, "resource_type": resourceType, "resource_id": resourceID, "request_id": reqID, "ip": ip, "details": details, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}
