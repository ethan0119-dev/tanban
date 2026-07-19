package app

import (
	"encoding/json"
	"net/http"
)

func (s *Server) platformDashboard(w http.ResponseWriter, r *http.Request) {
	var tenantCount, activeTenants, storeCount, todayOrders int
	var todayRevenue, monthRevenue int64
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*),COALESCE(SUM(status='ACTIVE'),0) FROM tenants WHERE deleted_at IS NULL").Scan(&tenantCount, &activeTenants); err != nil {
		handleSQLError(w, err)
		return
	}
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM stores WHERE deleted_at IS NULL").Scan(&storeCount); err != nil {
		handleSQLError(w, err)
		return
	}
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*),COALESCE(SUM(paid_cents),0) FROM orders WHERE DATE(created_at)=CURDATE()").Scan(&todayOrders, &todayRevenue); err != nil {
		handleSQLError(w, err)
		return
	}
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(paid_cents-refunded_cents),0) FROM orders WHERE paid_at>=DATE_FORMAT(CURDATE(),'%Y-%m-01')").Scan(&monthRevenue); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT DATE_FORMAT(days.day,'%Y-%m-%d'),COUNT(o.id),COALESCE(SUM(o.paid_cents),0) FROM (
		SELECT CURDATE() day UNION ALL SELECT CURDATE()-INTERVAL 1 DAY UNION ALL SELECT CURDATE()-INTERVAL 2 DAY UNION ALL SELECT CURDATE()-INTERVAL 3 DAY UNION ALL SELECT CURDATE()-INTERVAL 4 DAY UNION ALL SELECT CURDATE()-INTERVAL 5 DAY UNION ALL SELECT CURDATE()-INTERVAL 6 DAY
	) days LEFT JOIN orders o ON DATE(o.created_at)=days.day GROUP BY days.day ORDER BY days.day`)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	trend := []map[string]any{}
	for rows.Next() {
		var date string
		var orders int
		var amount int64
		if err := rows.Scan(&date, &orders, &amount); err != nil {
			handleSQLError(w, err)
			return
		}
		trend = append(trend, map[string]any{"date": date, "orders": orders, "amount": amount})
	}
	recentRows, err := s.DB.QueryContext(r.Context(), `SELECT t.id,t.code,t.name,t.contact_name,t.contact_phone,t.status,t.payment_provider,t.payment_merchant_no,t.payment_sub_appid,
		(SELECT COUNT(*) FROM stores s WHERE s.tenant_id=t.id AND s.deleted_at IS NULL),
		(SELECT COUNT(*) FROM orders o WHERE o.tenant_id=t.id),DATE_FORMAT(t.created_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM tenants t WHERE t.deleted_at IS NULL ORDER BY t.id DESC LIMIT 5`)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer recentRows.Close()
	recentTenants := []tenantDTO{}
	for recentRows.Next() {
		var tenant tenantDTO
		if err = recentRows.Scan(&tenant.ID, &tenant.Code, &tenant.Name, &tenant.ContactName, &tenant.ContactPhone, &tenant.Status, &tenant.PaymentProvider, &tenant.PaymentMerchantNo, &tenant.PaymentSubAppID, &tenant.StoreCount, &tenant.OrderCount, &tenant.CreatedAt); err != nil {
			handleSQLError(w, err)
			return
		}
		recentTenants = append(recentTenants, tenant)
	}
	writeData(w, http.StatusOK, map[string]any{"tenant_count": tenantCount, "tenantCount": tenantCount, "active_tenants": activeTenants, "activeTenantCount": activeTenants, "store_count": storeCount, "storeCount": storeCount, "today_orders": todayOrders, "todayOrderCount": todayOrders, "today_revenue_cents": todayRevenue, "todayTransactionAmount": todayRevenue, "month_revenue_cents": monthRevenue, "trend": trend, "recent_tenants": recentTenants})
}

func (s *Server) listAllStores(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM stores WHERE deleted_at IS NULL").Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT s.id,s.tenant_id,t.name,s.code,s.name,s.phone,s.address,s.business_hours,s.status,DATE_FORMAT(s.created_at,'%Y-%m-%dT%H:%i:%sZ') FROM stores s JOIN tenants t ON t.id=s.tenant_id WHERE s.deleted_at IS NULL ORDER BY s.id DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, tenantID int64
		var tenantName, code, name, phone, address, hours, status, created string
		if err := rows.Scan(&id, &tenantID, &tenantName, &code, &name, &phone, &address, &hours, &status, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "tenant_id": tenantID, "tenant_name": tenantName, "code": code, "name": name, "phone": phone, "address": address, "business_hours": hours, "status": status, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

type paymentSettings struct {
	Provider    string `json:"provider"`
	Enabled     bool   `json:"enabled"`
	Environment string `json:"environment"`
	OrgID       string `json:"orgId"`
	APIBaseURL  string `json:"apiBaseUrl"`
	NotifyURL   string `json:"notifyUrl"`
}

func (s *Server) getPlatformPaymentSettings(w http.ResponseWriter, r *http.Request) {
	settings := paymentSettings{Provider: s.Payment.Name(), Enabled: true, Environment: "sandbox", OrgID: s.Config.TianQue.OrgID, APIBaseURL: s.Config.TianQue.BaseURL, NotifyURL: s.Config.TianQue.NotifyURL}
	_ = s.loadSettingJSON(r, "payment", &settings)
	writeData(w, http.StatusOK, map[string]any{"provider": settings.Provider, "enabled": settings.Enabled, "environment": settings.Environment, "orgId": settings.OrgID, "apiBaseUrl": settings.APIBaseURL, "notifyUrl": settings.NotifyURL, "effectiveProvider": s.Payment.Name(), "restartRequired": settings.Provider != s.Payment.Name(), "tianqueConfigured": s.Config.TianQue.OrgID != "" && s.Config.TianQue.PrivateKey != "", "tianqueAdapterImplemented": false, "mockEnabled": s.AllowMockConfirmation, "publicKeyConfigured": s.Config.TianQue.PublicKey != "", "privateKeyConfigured": s.Config.TianQue.PrivateKey != ""})
}

func (s *Server) updatePlatformPaymentSettings(w http.ResponseWriter, r *http.Request) {
	var input paymentSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Provider != "mock" && input.Provider != "tianque" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "provider must be mock or tianque")
		return
	}
	if err := s.saveSettingJSON(r, "payment", input); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "settings.payment.update", "settings", "payment", map[string]any{"provider": input.Provider, "enabled": input.Enabled}, r)
	s.getPlatformPaymentSettings(w, r)
}

type systemSettings struct {
	PlatformName         string `json:"platformName"`
	SupportPhone         string `json:"supportPhone"`
	SupportEmail         string `json:"supportEmail"`
	OrderExpireMinutes   int    `json:"orderExpireMinutes"`
	LoginFailureLimit    int    `json:"loginFailureLimit"`
	SessionExpireMinutes int    `json:"sessionExpireMinutes"`
}

func (s *Server) getPlatformSystemSettings(w http.ResponseWriter, r *http.Request) {
	settings := systemSettings{PlatformName: "摊伴", OrderExpireMinutes: 15, LoginFailureLimit: 5, SessionExpireMinutes: int(s.Config.JWTTTL.Minutes())}
	_ = s.loadSettingJSON(r, "system", &settings)
	writeData(w, http.StatusOK, settings)
}

func (s *Server) updatePlatformSystemSettings(w http.ResponseWriter, r *http.Request) {
	var input systemSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.OrderExpireMinutes < 1 || input.LoginFailureLimit < 1 || input.SessionExpireMinutes < 1 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "numeric settings must be positive")
		return
	}
	if err := s.saveSettingJSON(r, "system", input); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "settings.system.update", "settings", "system", input, r)
	writeData(w, http.StatusOK, input)
}

func (s *Server) loadSettingJSON(r *http.Request, key string, target any) error {
	var body string
	if err := s.DB.QueryRowContext(r.Context(), "SELECT value_text FROM platform_settings WHERE setting_key=?", key).Scan(&body); err != nil {
		return err
	}
	return json.Unmarshal([]byte(body), target)
}

func (s *Server) saveSettingJSON(r *http.Request, key string, value any) error {
	body, _ := json.Marshal(value)
	_, err := s.DB.ExecContext(r.Context(), `INSERT INTO platform_settings(setting_key,value_text,updated_by) VALUES(?,?,?) ON DUPLICATE KEY UPDATE value_text=VALUES(value_text),updated_by=VALUES(updated_by)`, key, string(body), currentIdentity(r.Context()).UserID)
	return err
}
