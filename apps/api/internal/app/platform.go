package app

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type tenantDTO struct {
	ID                int64  `json:"id"`
	Code              string `json:"code"`
	Name              string `json:"name"`
	ContactName       string `json:"contact_name"`
	ContactPhone      string `json:"contact_phone"`
	Status            string `json:"status"`
	PaymentProvider   string `json:"payment_provider"`
	PaymentMerchantNo string `json:"payment_merchant_no"`
	PaymentSubAppID   string `json:"payment_sub_appid"`
	StoreCount        int    `json:"store_count"`
	OrderCount        int    `json:"order_count"`
	CreatedAt         string `json:"created_at"`
}

type tenantInput struct {
	Code              string `json:"code"`
	Name              string `json:"name"`
	ContactName       string `json:"contact_name"`
	ContactPhone      string `json:"contact_phone"`
	Status            string `json:"status"`
	PaymentProvider   string `json:"payment_provider"`
	PaymentMerchantNo string `json:"payment_merchant_no"`
	PaymentSubAppID   string `json:"payment_sub_appid"`
	OwnerUsername     string `json:"owner_username"`
	OwnerPassword     string `json:"owner_password"`
	OwnerDisplayName  string `json:"owner_display_name"`
	InitialStoreCode  string `json:"initial_store_code"`
	InitialStoreName  string `json:"initial_store_name"`
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

type storeInput struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	LogoURL       string `json:"logo_url"`
	BannerURL     string `json:"banner_url"`
	Address       string `json:"address"`
	Phone         string `json:"phone"`
	BusinessHours string `json:"business_hours"`
	Notice        string `json:"notice"`
	Status        string `json:"status"`
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
	r.Get("/stores", s.listAllStores)
	r.Get("/settings/payment", s.getPlatformPaymentSettings)
	r.With(requireRoles(RolePlatformAdmin)).Put("/settings/payment", s.updatePlatformPaymentSettings)
	r.Get("/settings/system", s.getPlatformSystemSettings)
	r.With(requireRoles(RolePlatformAdmin)).Put("/settings/system", s.updatePlatformSystemSettings)
	r.Get("/tenants", s.listTenants)
	r.With(requireRoles(RolePlatformAdmin)).Post("/tenants", s.createTenant)
	r.Route("/tenants/{tenantID}", func(t chi.Router) {
		t.Get("/", s.getTenant)
		t.With(requireRoles(RolePlatformAdmin)).Put("/", s.updateTenant)
		t.With(requireRoles(RolePlatformAdmin)).Delete("/", s.deleteTenant)
		t.Get("/stores", s.listPlatformStores)
		t.With(requireRoles(RolePlatformAdmin)).Post("/stores", s.createPlatformStore)
		t.Route("/stores/{storeID}", func(st chi.Router) {
			st.Get("/", s.getPlatformStore)
			st.With(requireRoles(RolePlatformAdmin)).Put("/", s.updatePlatformStore)
			st.With(requireRoles(RolePlatformAdmin)).Delete("/", s.deletePlatformStore)
		})
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
	var total int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenants WHERE deleted_at IS NULL AND (name LIKE ? OR code LIKE ?)`, search, search).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT t.id,t.code,t.name,t.contact_name,t.contact_phone,t.status,t.payment_provider,t.payment_merchant_no,t.payment_sub_appid,
		(SELECT COUNT(*) FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL),
		(SELECT COUNT(*) FROM orders o WHERE o.tenant_id=t.id),DATE_FORMAT(t.created_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM tenants t WHERE t.deleted_at IS NULL AND (t.name LIKE ? OR t.code LIKE ?) ORDER BY t.id DESC LIMIT ? OFFSET ?`, search, search, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []tenantDTO{}
	for rows.Next() {
		var item tenantDTO
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.ContactName, &item.ContactPhone, &item.Status, &item.PaymentProvider, &item.PaymentMerchantNo, &item.PaymentSubAppID, &item.StoreCount, &item.OrderCount, &item.CreatedAt); err != nil {
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
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE or DISABLED")
		return
	}
	if input.PaymentProvider == "" {
		input.PaymentProvider = "mock"
	}
	input.PaymentProvider = strings.ToLower(input.PaymentProvider)
	if input.PaymentProvider != "mock" && input.PaymentProvider != "tianque" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "payment_provider must be mock or tianque")
		return
	}
	if (input.OwnerUsername == "") != (input.OwnerPassword == "") || input.OwnerPassword != "" && len(input.OwnerPassword) < 8 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "owner_username and an owner_password of at least 8 characters must be provided together")
		return
	}
	if (input.InitialStoreCode == "") != (input.InitialStoreName == "") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "initial_store_code and initial_store_name must be provided together")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `INSERT INTO tenants(code,name,contact_name,contact_phone,status,payment_provider,payment_merchant_no,payment_sub_appid)
		VALUES(?,?,?,?,?,?,?,?)`, input.Code, input.Name, input.ContactName, input.ContactPhone, strings.ToUpper(input.Status), strings.ToLower(input.PaymentProvider), input.PaymentMerchantNo, input.PaymentSubAppID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	if input.InitialStoreCode != "" {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO stores(tenant_id,code,name,status) VALUES(?,?,?,'ACTIVE')`, id, input.InitialStoreCode, input.InitialStoreName); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if input.OwnerUsername != "" {
		displayName := strings.TrimSpace(input.OwnerDisplayName)
		if displayName == "" {
			displayName = input.ContactName
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(input.OwnerPassword), bcrypt.DefaultCost)
		if hashErr != nil {
			handleSQLError(w, hashErr)
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO users(tenant_id,username,password_hash,display_name,role,status) VALUES(?,?,?,?,?,'ACTIVE')`, id, input.OwnerUsername, string(hash), displayName, RoleMerchantOwner); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "tenant.create", "tenant", int64String(id), map[string]any{
		"code": input.Code, "name": input.Name, "payment_provider": input.PaymentProvider,
		"owner_username": input.OwnerUsername, "initial_store_code": input.InitialStoreCode,
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
		(SELECT COUNT(*) FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL),
		(SELECT COUNT(*) FROM orders o WHERE o.tenant_id=t.id),DATE_FORMAT(t.created_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM tenants t WHERE t.id=? AND t.deleted_at IS NULL`, id).
		Scan(&item.ID, &item.Code, &item.Name, &item.ContactName, &item.ContactPhone, &item.Status, &item.PaymentProvider, &item.PaymentMerchantNo, &item.PaymentSubAppID, &item.StoreCount, &item.OrderCount, &item.CreatedAt)
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
	if _, err = tx.ExecContext(r.Context(), "UPDATE users SET status='DISABLED' WHERE tenant_id=?", id); err != nil {
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

func (s *Server) listPlatformStores(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,tenant_id,code,name,logo_url,banner_url,address,phone,business_hours,notice,status,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ')
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

func (s *Server) createPlatformStore(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := pathID(w, r, "tenantID")
	if !ok {
		return
	}
	var input storeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Code == "" || input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "code and name are required")
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO stores(tenant_id,code,name,logo_url,banner_url,address,phone,business_hours,notice,status)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, tenantID, input.Code, input.Name, input.LogoURL, input.BannerURL, input.Address, input.Phone, input.BusinessHours, input.Notice, strings.ToUpper(input.Status))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), currentIdentity(r.Context()), "store.create", "store", int64String(id), input, r)
	s.getStoreByScope(w, r, tenantID, id)
}

func (s *Server) getPlatformStore(w http.ResponseWriter, r *http.Request) {
	tenantID, ok1 := pathID(w, r, "tenantID")
	storeID, ok2 := pathID(w, r, "storeID")
	if ok1 && ok2 {
		s.getStoreByScope(w, r, tenantID, storeID)
	}
}

func (s *Server) getStoreByScope(w http.ResponseWriter, r *http.Request, tenantID, storeID int64) {
	var item storeDTO
	err := scanStoreRow(s.DB.QueryRowContext(r.Context(), `SELECT id,tenant_id,code,name,logo_url,banner_url,address,phone,business_hours,notice,status,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ') FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, storeID, tenantID), &item)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updatePlatformStore(w http.ResponseWriter, r *http.Request) {
	tenantID, ok1 := pathID(w, r, "tenantID")
	storeID, ok2 := pathID(w, r, "storeID")
	if !ok1 || !ok2 {
		return
	}
	var input storeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE stores SET code=?,name=?,logo_url=?,banner_url=?,address=?,phone=?,business_hours=?,notice=?,status=?
		WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, input.Code, input.Name, input.LogoURL, input.BannerURL, input.Address, input.Phone, input.BusinessHours, input.Notice, strings.ToUpper(input.Status), storeID, tenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "store not found")
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "store.update", "store", int64String(storeID), input, r)
	s.getStoreByScope(w, r, tenantID, storeID)
}

func (s *Server) deletePlatformStore(w http.ResponseWriter, r *http.Request) {
	tenantID, ok1 := pathID(w, r, "tenantID")
	storeID, ok2 := pathID(w, r, "storeID")
	if !ok1 || !ok2 {
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE stores SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", storeID, tenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "store not found")
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "store.delete", "store", int64String(storeID), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

type scanner interface{ Scan(...any) error }

func scanStore(row scanner, item *storeDTO) error {
	return row.Scan(&item.ID, &item.TenantID, &item.Code, &item.Name, &item.LogoURL, &item.BannerURL, &item.Address, &item.Phone, &item.BusinessHours, &item.Notice, &item.Status, &item.CreatedAt)
}
func scanStoreRow(row *sql.Row, item *storeDTO) error { return scanStore(row, item) }

func (s *Server) listPlatformUsers(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE tenant_id=0 AND deleted_at IS NULL").Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,tenant_id,username,display_name,role,status,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM users WHERE tenant_id=0 AND deleted_at IS NULL ORDER BY id DESC LIMIT ? OFFSET ?`, size, offset)
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
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO users(tenant_id,username,password_hash,display_name,role,status) VALUES(?,?,?,?,?,?)`, input.TenantID, input.Username, string(hash), input.DisplayName, input.Role, input.Status)
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
	query := `SELECT id,tenant_id,username,display_name,role,status,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ') FROM users WHERE id=? AND deleted_at IS NULL`
	args := []any{id}
	if platform {
		query += " AND tenant_id=0"
	} else {
		query += " AND tenant_id=?"
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
	if err = tx.QueryRowContext(r.Context(), "SELECT role,status FROM users WHERE id=? AND tenant_id=0 AND deleted_at IS NULL FOR UPDATE", id).Scan(&targetRole, &targetStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if actor.UserID == id && (input.Role != targetRole || input.Status != targetStatus) {
		writeError(w, http.StatusConflict, "CANNOT_CHANGE_OWN_ACCESS", "current administrator cannot change its own role or status")
		return
	}
	if targetRole == RolePlatformAdmin && targetStatus == "ACTIVE" && (input.Role != RolePlatformAdmin || input.Status != "ACTIVE") {
		rows, countErr := tx.QueryContext(r.Context(), "SELECT id FROM users WHERE tenant_id=0 AND role=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE", RolePlatformAdmin)
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
		result, err = tx.ExecContext(r.Context(), `UPDATE users SET username=?,password_hash=?,display_name=?,role=?,status=? WHERE id=? AND tenant_id=0 AND deleted_at IS NULL`, input.Username, string(hash), input.DisplayName, input.Role, input.Status, id)
	} else {
		result, err = tx.ExecContext(r.Context(), `UPDATE users SET username=?,display_name=?,role=?,status=? WHERE id=? AND tenant_id=0 AND deleted_at IS NULL`, input.Username, input.DisplayName, input.Role, input.Status, id)
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
	if err = tx.QueryRowContext(r.Context(), "SELECT role,status FROM users WHERE id=? AND tenant_id=0 AND deleted_at IS NULL FOR UPDATE", id).Scan(&targetRole, &targetStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if targetRole == RolePlatformAdmin && targetStatus == "ACTIVE" {
		rows, countErr := tx.QueryContext(r.Context(), "SELECT id FROM users WHERE tenant_id=0 AND role=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE", RolePlatformAdmin)
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
	result, err := tx.ExecContext(r.Context(), "UPDATE users SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=0 AND deleted_at IS NULL", id)
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
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,tenant_id,actor_user_id,action,resource_type,resource_id,request_id,ip,details_text,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ')
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
