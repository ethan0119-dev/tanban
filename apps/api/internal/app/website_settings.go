package app

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type websiteSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type websiteSettingsMap map[string]string

type customerLead struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Source    string `json:"source"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	IPAddress string `json:"ipAddress,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type customerLeadInput struct {
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Source string `json:"source"`
}

// ---- Public endpoints (no auth) ----

func (s *Server) getPublicWebsiteSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.loadWebsiteSettings(r)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]string{
		"contactPhone":  settings["contact_phone"],
		"contactWechat": settings["contact_wechat"],
		"contactEmail":  settings["contact_email"],
		"wechatQrUrl":   settings["wechat_qr_url"],
		"heroImageUrl":  settings["hero_image_url"],
		"seoTitle":      settings["seo_title"],
		"seoDescription": settings["seo_description"],
	})
}

func (s *Server) submitCustomerLead(w http.ResponseWriter, r *http.Request) {
	// Rate limit: 5 per minute, 20 per hour per IP
	if !s.consumePublicMarketingBucket(r.Context(), "website", "lead", publicClientHash(r), time.Now().UTC(), time.Minute, 5) ||
		!s.consumePublicMarketingBucket(r.Context(), "website", "lead", publicClientHash(r), time.Now().UTC(), time.Hour, 20) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "提交太频繁，请稍后再试")
		return
	}

	var input customerLeadInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		input.Source = "website"
	}

	if input.Name == "" || len(input.Name) > 64 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "请输入姓名（64字以内）")
		return
	}
	if !digitsOnly(input.Phone) || len(input.Phone) != 11 || !strings.HasPrefix(input.Phone, "1") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "请输入正确的手机号码")
		return
	}

	// Check for duplicate within 24 hours
	var exists int
	err := s.DB.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM customer_leads WHERE phone=? AND created_at > DATE_SUB(NOW(3), INTERVAL 24 HOUR)",
		input.Phone).Scan(&exists)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if exists > 0 {
		writeError(w, http.StatusConflict, "DUPLICATE", "您已提交过申请，我们会在1个工作日内联系您")
		return
	}

	if _, err := s.DB.ExecContext(r.Context(),
		"INSERT INTO customer_leads(name, phone, source, ip_address) VALUES(?,?,?,?)",
		input.Name, input.Phone, input.Source, publicClientHost(r)); err != nil {
		handleSQLError(w, err)
		return
	}

	writeData(w, http.StatusCreated, map[string]string{"message": "提交成功，我们会在1个工作日内联系您"})
}

// ---- Platform admin endpoints ----

func (s *Server) getPlatformWebsiteSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.loadWebsiteSettings(r)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, settings)
}

func (s *Server) updatePlatformWebsiteSettings(w http.ResponseWriter, r *http.Request) {
	var input map[string]string
	if !decodeJSON(w, r, &input) {
		return
	}

	allowed := map[string]bool{
		"contact_phone": true, "contact_wechat": true, "contact_email": true,
		"wechat_qr_url": true, "hero_image_url": true,
		"seo_title": true, "seo_description": true,
	}

	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()

	for key, value := range input {
		if !allowed[key] {
			continue
		}
		if _, err := tx.ExecContext(r.Context(),
			"INSERT INTO website_settings(setting_key, setting_value) VALUES(?,?) ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value)",
			key, strings.TrimSpace(value)); err != nil {
			handleSQLError(w, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}

	s.audit(r.Context(), currentIdentity(r.Context()), "platform.website_settings.update", "website", "settings", nil, r)
	s.getPlatformWebsiteSettings(w, r)
}

func (s *Server) listCustomerLeads(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	keyword := "%" + strings.TrimSpace(r.URL.Query().Get("q")) + "%"

	where := "WHERE 1=1"
	args := []any{}
	if status != "" {
		if !validStatus(status, "NEW", "CONTACTED", "CONVERTED", "CLOSED") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status")
			return
		}
		where += " AND status=?"
		args = append(args, status)
	}
	if keyword != "%%" {
		where += " AND (name LIKE ? OR phone LIKE ?)"
		args = append(args, keyword, keyword)
	}

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	if err := s.DB.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM customer_leads "+where, countArgs...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}

	query := `SELECT id, name, phone, source, status, note, ip_address,
		DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s'), DATE_FORMAT(updated_at,'%Y-%m-%d %H:%i:%s')
		FROM customer_leads ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, size, offset)

	rows, err := s.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()

	items := []customerLead{}
	for rows.Next() {
		var item customerLead
		if err := rows.Scan(&item.ID, &item.Name, &item.Phone, &item.Source, &item.Status,
			&item.Note, &item.IPAddress, &item.CreatedAt, &item.UpdatedAt); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}

	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) updateCustomerLead(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "leadID")
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	input.Note = strings.TrimSpace(input.Note)
	if !validStatus(input.Status, "NEW", "CONTACTED", "CONVERTED", "CLOSED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status")
		return
	}
	result, err := s.DB.ExecContext(r.Context(),
		"UPDATE customer_leads SET status=?, note=? WHERE id=?", input.Status, input.Note, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "lead not found")
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "platform.lead.update", "lead", int64String(id),
		map[string]any{"status": input.Status}, r)
	writeData(w, http.StatusOK, map[string]string{"message": "已更新"})
}

// ---- Helpers ----

func (s *Server) loadWebsiteSettings(r *http.Request) (websiteSettingsMap, error) {
	settings := websiteSettingsMap{}
	rows, err := s.DB.QueryContext(r.Context(), "SELECT setting_key, setting_value FROM website_settings")
	if err != nil {
		if err == sql.ErrNoRows {
			return settings, nil
		}
		return settings, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return settings, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
}
