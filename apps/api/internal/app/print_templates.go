package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

const defaultReceiptTemplate = "订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n备注：{{remark}}"

var defaultPrintTemplateContent = map[string]string{
	orderTypeDineIn:   "【店内】 {{table_area}} {{table_name}}\n订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n备注：{{remark}}",
	orderTypeTakeout:  "【自提】\n" + defaultReceiptTemplate,
	orderTypeDelivery: "【外卖】\n" + defaultReceiptTemplate,
}

var defaultLabelTemplateContent = map[string]string{
	orderTypeDineIn:   "【店内】 {{table_name}} #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}",
	orderTypeTakeout:  "【自提】 #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}",
	orderTypeDelivery: "【外卖】 #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}",
}

type printTemplateDTO struct {
	ID           int64  `json:"id"`
	StoreID      int64  `json:"storeId"`
	BusinessType string `json:"businessType"`
	TemplateType string `json:"templateType"`
	Name         string `json:"name"`
	Content      string `json:"content"`
	TriggerEvent string `json:"triggerEvent"`
	Copies       int    `json:"copies"`
	Enabled      bool   `json:"enabled"`
	Status       string `json:"status"`
	UpdatedAt    string `json:"updatedAt"`
}

type printTemplateInput struct {
	StoreID           int64  `json:"storeId"`
	LegacyStoreID     int64  `json:"store_id"`
	BusinessType      string `json:"businessType"`
	LegacyBusiness    string `json:"business_type"`
	TemplateType      string `json:"templateType"`
	LegacyTemplate    string `json:"template_type"`
	Name              string `json:"name"`
	Content           string `json:"content"`
	LegacyContentText string `json:"content_text"`
	TriggerEvent      string `json:"triggerEvent"`
	LegacyTrigger     string `json:"trigger_event"`
	Copies            int    `json:"copies"`
	Enabled           *bool  `json:"enabled"`
	Status            string `json:"status"`
}

func normalizePrintTemplateInput(input *printTemplateInput) error {
	if input.StoreID == 0 {
		input.StoreID = input.LegacyStoreID
	}
	if input.BusinessType == "" {
		input.BusinessType = input.LegacyBusiness
	}
	if input.TemplateType == "" {
		input.TemplateType = input.LegacyTemplate
	}
	if input.Content == "" {
		input.Content = input.LegacyContentText
	}
	if input.TriggerEvent == "" {
		input.TriggerEvent = input.LegacyTrigger
	}
	input.BusinessType = strings.ToUpper(strings.TrimSpace(input.BusinessType))
	input.TemplateType = strings.ToUpper(strings.TrimSpace(input.TemplateType))
	input.Name = strings.TrimSpace(input.Name)
	input.TriggerEvent = strings.ToUpper(strings.TrimSpace(input.TriggerEvent))
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.TemplateType == "" {
		input.TemplateType = "RECEIPT"
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.Enabled != nil {
		if *input.Enabled {
			input.Status = "ACTIVE"
		} else {
			input.Status = "DISABLED"
		}
	}
	if input.TriggerEvent == "" {
		input.TriggerEvent = "PAYMENT_SUCCESS"
	}
	if input.Copies == 0 {
		input.Copies = 1
	}
	if !validStatus(input.BusinessType, orderTypeDineIn, orderTypeTakeout, orderTypeDelivery) {
		return errors.New("businessType must be DINE_IN, TAKEOUT or DELIVERY")
	}
	if !validStatus(input.TemplateType, "RECEIPT", "LABEL") {
		return errors.New("templateType must be RECEIPT or LABEL")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	if !validStatus(input.TriggerEvent, "ORDER_CREATED", "PAYMENT_SUCCESS") {
		return errors.New("triggerEvent must be ORDER_CREATED or PAYMENT_SUCCESS")
	}
	if input.Copies < 1 || input.Copies > 5 {
		return errors.New("copies must be between 1 and 5")
	}
	if input.Name == "" || len([]rune(input.Name)) > 100 {
		return errors.New("name is required and must not exceed 100 characters")
	}
	if strings.TrimSpace(input.Content) == "" || len([]rune(input.Content)) > 20000 {
		return errors.New("content is required and must not exceed 20000 characters")
	}
	return nil
}

func ensureDefaultPrintTemplates(ctx context.Context, executor sqlExecer, tenantID, storeID int64) error {
	for _, businessType := range []string{orderTypeDineIn, orderTypeTakeout, orderTypeDelivery} {
		for _, templateType := range []string{"RECEIPT", "LABEL"} {
			name := map[string]string{orderTypeDineIn: "店内", orderTypeTakeout: "自提", orderTypeDelivery: "外卖"}[businessType]
			content := defaultPrintTemplateContent[businessType]
			if templateType == "LABEL" {
				content = defaultLabelTemplateContent[businessType]
				name += "标签"
			} else {
				name += "小票"
			}
			if _, err := executor.ExecContext(ctx, `INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)
				SELECT ?,id,?,?,?,?,default_print_trigger,1,'ACTIVE' FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, tenantID, businessType, templateType, name, content, storeID, tenantID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) listPrintTemplates(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if err = ensureDefaultPrintTemplates(r.Context(), s.DB, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	where := " WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL"
	args := []any{identity.TenantID, storeID}
	businessType := r.URL.Query().Get("businessType")
	if businessType == "" {
		businessType = r.URL.Query().Get("business_type")
	}
	if businessType != "" {
		businessType = strings.ToUpper(strings.TrimSpace(businessType))
		if !validStatus(businessType, orderTypeDineIn, orderTypeTakeout, orderTypeDelivery) {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "businessType must be DINE_IN, TAKEOUT or DELIVERY")
			return
		}
		where += " AND business_type=?"
		args = append(args, businessType)
	}
	templateType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("templateType")))
	if templateType != "" {
		if !validStatus(templateType, "RECEIPT", "LABEL") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "templateType must be RECEIPT or LABEL")
			return
		}
		where += " AND template_type=?"
		args = append(args, templateType)
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status,DATE_FORMAT(updated_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM print_templates`+where+" ORDER BY FIELD(business_type,'DINE_IN','TAKEOUT','DELIVERY'),template_type,id", args...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []printTemplateDTO{}
	for rows.Next() {
		var item printTemplateDTO
		if err = scanPrintTemplate(rows, &item); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createPrintTemplate(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input printTemplateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizePrintTemplateInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
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
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)
		SELECT ?,id,?,?,?,?,?,?,? FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL
		ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id),name=VALUES(name),content_text=VALUES(content_text),trigger_event=VALUES(trigger_event),copies=VALUES(copies),status=VALUES(status),deleted_at=NULL`, identity.TenantID, input.BusinessType, input.TemplateType, input.Name, input.Content, input.TriggerEvent, input.Copies, input.Status, storeID, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	if id == 0 {
		writeError(w, http.StatusNotFound, "STORE_NOT_FOUND", "store not found")
		return
	}
	s.audit(r.Context(), identity, "print_template.upsert", "print_template", int64String(id), map[string]any{"business_type": input.BusinessType, "template_type": input.TemplateType}, r)
	s.getPrintTemplateByID(w, r, identity.TenantID, id)
}

func scanPrintTemplate(row scanner, item *printTemplateDTO) error {
	if err := row.Scan(&item.ID, &item.StoreID, &item.BusinessType, &item.TemplateType, &item.Name, &item.Content, &item.TriggerEvent, &item.Copies, &item.Status, &item.UpdatedAt); err != nil {
		return err
	}
	item.Enabled = item.Status == "ACTIVE"
	return nil
}

func (s *Server) getPrintTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "templateID")
	if ok {
		s.getPrintTemplateByID(w, r, currentIdentity(r.Context()).TenantID, id)
	}
}

func (s *Server) getPrintTemplateByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var item printTemplateDTO
	err := scanPrintTemplate(s.DB.QueryRowContext(r.Context(), `SELECT id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status,DATE_FORMAT(updated_at,'%Y-%m-%dT%H:%i:%sZ')
		FROM print_templates WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, id, tenantID), &item)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updatePrintTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "templateID")
	if !ok {
		return
	}
	var input printTemplateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizePrintTemplateInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	identity := currentIdentity(r.Context())
	_, err := s.DB.ExecContext(r.Context(), `UPDATE print_templates SET business_type=?,template_type=?,name=?,content_text=?,trigger_event=?,copies=?,status=?
		WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, input.BusinessType, input.TemplateType, input.Name, input.Content, input.TriggerEvent, input.Copies, input.Status, id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "print_template.update", "print_template", int64String(id), map[string]any{"business_type": input.BusinessType, "template_type": input.TemplateType}, r)
	s.getPrintTemplateByID(w, r, identity.TenantID, id)
}

func (s *Server) deletePrintTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "templateID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), "UPDATE print_templates SET status='DISABLED' WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if count, _ := result.RowsAffected(); count == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "print template not found")
		return
	}
	s.audit(r.Context(), identity, "print_template.delete", "print_template", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

type activePrintTemplate struct {
	Content      string
	TriggerEvent string
	Copies       int
	Found        bool
	Enabled      bool
}

func loadActivePrintTemplate(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64, businessType, templateType string) (activePrintTemplate, error) {
	var item activePrintTemplate
	var status string
	err := queryer.QueryRowContext(ctx, `SELECT content_text,trigger_event,copies,status FROM print_templates WHERE tenant_id=? AND store_id=?
		AND business_type=? AND template_type=? AND deleted_at IS NULL LIMIT 1`, tenantID, storeID, businessType, templateType).Scan(&item.Content, &item.TriggerEvent, &item.Copies, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return item, nil
	}
	if err == nil {
		item.Found = true
		item.Enabled = status == "ACTIVE"
	}
	return item, err
}
