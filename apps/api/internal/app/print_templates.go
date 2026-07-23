package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	orderTypeDineIn:   "【店内】 {{table_name}} #{{pickup_no}} 数量：{{item_sequence}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}",
	orderTypeTakeout:  "【自提】 #{{pickup_no}} 数量：{{item_sequence}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}",
	orderTypeDelivery: "【外卖】 #{{pickup_no}} 数量：{{item_sequence}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}",
}

type printTemplateDTO struct {
	ID           int64          `json:"id"`
	StoreID      int64          `json:"storeId"`
	BusinessType string         `json:"businessType"`
	TemplateType string         `json:"templateType"`
	CopyRole     string         `json:"copyRole"`
	Name         string         `json:"name"`
	Content      string         `json:"content"`
	TriggerEvent string         `json:"triggerEvent"`
	Copies       int            `json:"copies"`
	PaperWidth   int            `json:"paperWidth"`
	Layout       map[string]any `json:"layout"`
	Enabled      bool           `json:"enabled"`
	Status       string         `json:"status"`
	UpdatedAt    string         `json:"updatedAt"`
}

type printTemplateInput struct {
	StoreID           int64          `json:"storeId"`
	LegacyStoreID     int64          `json:"store_id"`
	BusinessType      string         `json:"businessType"`
	LegacyBusiness    string         `json:"business_type"`
	TemplateType      string         `json:"templateType"`
	LegacyTemplate    string         `json:"template_type"`
	CopyRole          string         `json:"copyRole"`
	LegacyCopyRole    string         `json:"copy_role"`
	Name              string         `json:"name"`
	Content           string         `json:"content"`
	LegacyContentText string         `json:"content_text"`
	TriggerEvent      string         `json:"triggerEvent"`
	LegacyTrigger     string         `json:"trigger_event"`
	Copies            int            `json:"copies"`
	PaperWidth        int            `json:"paperWidth"`
	LegacyPaperWidth  int            `json:"paper_width"`
	Layout            map[string]any `json:"layout"`
	LayoutJSON        string         `json:"-"`
	Enabled           *bool          `json:"enabled"`
	Status            string         `json:"status"`
}

var printLayoutKeys = map[string]string{
	"schemaVersion": "version", "preset": "string", "headerStyle": "string", "fontSize": "string", "copyTitle": "string",
	"showStoreName": "bool", "showOrderType": "bool", "showOrderNo": "bool",
	"showOrderTime": "bool", "showPickupNo": "bool", "showTable": "bool", "showItems": "bool", "showItemSequence": "bool",
	"showItemHeader": "bool", "showItemOptions": "bool", "showOptionGroupNames": "bool",
	"showPrices": "bool", "showPayment": "bool", "emphasizePaid": "bool",
	"showRemark": "bool", "showCustomer": "bool", "showAddress": "bool",
	"showQrCode": "bool", "showEndMarker": "bool", "endMarkerText": "string",
	"feedLines": "integer", "labelWidthMM": "integer", "labelHeightMM": "integer",
	"customHeader": "string", "customFooter": "string",
}

func defaultStructuredPrintLayout(copyRole string) map[string]any {
	customer := copyRole == "CUSTOMER"
	item := copyRole == "ITEM"
	kitchen := copyRole == "KITCHEN"
	showPrices := copyRole != "KITCHEN" && copyRole != "ITEM"
	showPayment := copyRole != "KITCHEN" && copyRole != "ITEM"
	showStoreName := copyRole != "ITEM"
	showOrderType := true
	showOrderNo := copyRole != "ITEM"
	headerStyle := "PROMINENT"
	fontSize := "NORMAL"
	preset := "DETAILED"
	copyTitle := map[string]string{"MERCHANT": "商", "CUSTOMER": "客", "KITCHEN": "厨", "ITEM": "签"}[copyRole]
	customFooter := ""
	if kitchen {
		fontSize = "LARGE"
		preset = "LARGE"
	}
	if item {
		headerStyle = "SIMPLE"
		fontSize = "LARGE"
		preset = "LARGE"
	}
	if customer {
		customFooter = "感谢光临，欢迎再次惠顾"
	}
	return map[string]any{
		"schemaVersion": 1, "preset": preset, "headerStyle": headerStyle, "fontSize": fontSize, "copyTitle": copyTitle,
		"showStoreName": showStoreName, "showOrderType": showOrderType, "showOrderNo": showOrderNo, "showOrderTime": true, "showPickupNo": true,
		"showTable": true, "showItems": true, "showItemSequence": item, "showItemHeader": !item,
		"showItemOptions": true, "showOptionGroupNames": false, "showPrices": showPrices,
		"showPayment": showPayment, "emphasizePaid": showPayment, "showRemark": true, "showCustomer": customer,
		"showAddress": customer, "showQrCode": customer, "showEndMarker": !item, "endMarkerText": "",
		"feedLines": map[bool]int{true: 0, false: 3}[item], "labelWidthMM": 40, "labelHeightMM": 30,
		"customHeader": "", "customFooter": customFooter,
	}
}

func normalizePrintLayout(layout map[string]any, copyRole string) (map[string]any, string, error) {
	if len(layout) == 0 {
		return map[string]any{}, "{}", nil
	}
	normalized := defaultStructuredPrintLayout(copyRole)
	for key, value := range layout {
		kind, ok := printLayoutKeys[key]
		if !ok {
			return nil, "", fmt.Errorf("layout.%s is not supported", key)
		}
		switch kind {
		case "bool":
			if _, ok = value.(bool); !ok {
				return nil, "", fmt.Errorf("layout.%s must be boolean", key)
			}
		case "string":
			text, stringOK := value.(string)
			if !stringOK {
				return nil, "", fmt.Errorf("layout.%s must be a string", key)
			}
			if len([]rune(text)) > 500 {
				return nil, "", fmt.Errorf("layout.%s must not exceed 500 characters", key)
			}
		case "version":
			version, numberOK := layoutInt(value)
			if !numberOK || version != 1 {
				return nil, "", errors.New("layout.schemaVersion must be 1")
			}
			value = version
		case "integer":
			number, numberOK := layoutInt(value)
			if !numberOK {
				return nil, "", fmt.Errorf("layout.%s must be an integer", key)
			}
			switch key {
			case "feedLines":
				if number < 0 || number > 8 {
					return nil, "", errors.New("layout.feedLines must be between 0 and 8")
				}
			case "labelWidthMM":
				if number < 20 || number > 110 {
					return nil, "", errors.New("layout.labelWidthMM must be between 20 and 110")
				}
			case "labelHeightMM":
				if number < 20 || number > 200 {
					return nil, "", errors.New("layout.labelHeightMM must be between 20 and 200")
				}
			}
			value = number
		}
		normalized[key] = value
	}
	preset := strings.ToUpper(strings.TrimSpace(fmt.Sprint(normalized["preset"])))
	if !validStatus(preset, "COMPACT", "LARGE", "DETAILED", "CUSTOM") {
		return nil, "", errors.New("layout.preset must be COMPACT, LARGE, DETAILED or CUSTOM")
	}
	normalized["preset"] = preset
	headerStyle := strings.ToUpper(strings.TrimSpace(fmt.Sprint(normalized["headerStyle"])))
	if !validStatus(headerStyle, "SIMPLE", "PROMINENT") {
		return nil, "", errors.New("layout.headerStyle must be SIMPLE or PROMINENT")
	}
	normalized["headerStyle"] = headerStyle
	fontSize := strings.ToUpper(strings.TrimSpace(fmt.Sprint(normalized["fontSize"])))
	if !validStatus(fontSize, "NORMAL", "LARGE") {
		return nil, "", errors.New("layout.fontSize must be NORMAL or LARGE")
	}
	normalized["fontSize"] = fontSize
	copyTitle := strings.TrimSpace(fmt.Sprint(normalized["copyTitle"]))
	if copyTitle == "" || len([]rune(copyTitle)) > 4 {
		return nil, "", errors.New("layout.copyTitle is required and must not exceed 4 characters")
	}
	normalized["copyTitle"] = copyTitle
	if len([]rune(strings.TrimSpace(fmt.Sprint(normalized["endMarkerText"])))) > 40 {
		return nil, "", errors.New("layout.endMarkerText must not exceed 40 characters")
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return nil, "", err
	}
	return normalized, string(body), nil
}

func layoutInt(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int64:
		return int(number), true
	case float64:
		return int(number), number == float64(int(number))
	default:
		return 0, false
	}
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
	if input.CopyRole == "" {
		input.CopyRole = input.LegacyCopyRole
	}
	if input.Content == "" {
		input.Content = input.LegacyContentText
	}
	if input.TriggerEvent == "" {
		input.TriggerEvent = input.LegacyTrigger
	}
	input.BusinessType = strings.ToUpper(strings.TrimSpace(input.BusinessType))
	input.TemplateType = strings.ToUpper(strings.TrimSpace(input.TemplateType))
	input.CopyRole = strings.ToUpper(strings.TrimSpace(input.CopyRole))
	input.Name = strings.TrimSpace(input.Name)
	input.TriggerEvent = strings.ToUpper(strings.TrimSpace(input.TriggerEvent))
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.TemplateType == "" {
		input.TemplateType = "RECEIPT"
	}
	if input.CopyRole == "" {
		if input.TemplateType == "LABEL" {
			input.CopyRole = "ITEM"
		} else {
			input.CopyRole = "MERCHANT"
		}
	}
	if input.PaperWidth == 0 {
		input.PaperWidth = input.LegacyPaperWidth
	}
	if input.PaperWidth == 0 {
		input.PaperWidth = 58
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
	if input.TemplateType == "RECEIPT" && !validStatus(input.CopyRole, "MERCHANT", "CUSTOMER", "KITCHEN") {
		return errors.New("RECEIPT copyRole must be MERCHANT, CUSTOMER or KITCHEN")
	}
	if input.TemplateType == "LABEL" && input.CopyRole != "ITEM" {
		return errors.New("LABEL copyRole must be ITEM")
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
	if input.PaperWidth != 58 && input.PaperWidth != 80 {
		return errors.New("paperWidth must be 58 or 80")
	}
	if input.Name == "" || len([]rune(input.Name)) > 100 {
		return errors.New("name is required and must not exceed 100 characters")
	}
	if len([]rune(input.Content)) > 20000 {
		return errors.New("content must not exceed 20000 characters")
	}
	var err error
	input.Layout, input.LayoutJSON, err = normalizePrintLayout(input.Layout, input.CopyRole)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Content) == "" && len(input.Layout) == 0 {
		return errors.New("content or a structured layout is required")
	}
	return nil
}

func ensureDefaultPrintTemplates(ctx context.Context, executor sqlExecer, tenantID, storeID int64) error {
	for _, businessType := range []string{orderTypeDineIn, orderTypeTakeout, orderTypeDelivery} {
		prefix := map[string]string{orderTypeDineIn: "店内", orderTypeTakeout: "自提", orderTypeDelivery: "外卖"}[businessType]
		specs := []struct {
			templateType, copyRole, name, content, status string
		}{
			{"RECEIPT", "MERCHANT", prefix + "商家联", defaultPrintTemplateContent[businessType], "ACTIVE"},
			{"RECEIPT", "CUSTOMER", prefix + "顾客联", defaultPrintTemplateContent[businessType], "DISABLED"},
			{"RECEIPT", "KITCHEN", prefix + "后厨联", defaultPrintTemplateContent[businessType], "DISABLED"},
			{"LABEL", "ITEM", prefix + "商品标签", defaultLabelTemplateContent[businessType], "ACTIVE"},
		}
		for _, spec := range specs {
			_, layoutJSON, err := normalizePrintLayout(defaultStructuredPrintLayout(spec.copyRole), spec.copyRole)
			if err != nil {
				return err
			}
			if _, err = executor.ExecContext(ctx, `INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,copy_role,name,content_text,trigger_event,copies,paper_width,layout_json,status)
				SELECT ?,id,?,?,?,?,?,default_print_trigger,1,58,?,? FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, tenantID, businessType, spec.templateType, spec.copyRole, spec.name, spec.content, layoutJSON, spec.status, storeID, tenantID); err != nil {
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
	if templateType == "" {
		templateType = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("template_type")))
	}
	if templateType != "" {
		if !validStatus(templateType, "RECEIPT", "LABEL") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "templateType must be RECEIPT or LABEL")
			return
		}
		where += " AND template_type=?"
		args = append(args, templateType)
	}
	copyRole := r.URL.Query().Get("copyRole")
	if copyRole == "" {
		copyRole = r.URL.Query().Get("copy_role")
	}
	if copyRole != "" {
		copyRole = strings.ToUpper(strings.TrimSpace(copyRole))
		if !validStatus(copyRole, "MERCHANT", "CUSTOMER", "KITCHEN", "ITEM") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "copyRole is invalid")
			return
		}
		where += " AND copy_role=?"
		args = append(args, copyRole)
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,store_id,business_type,template_type,COALESCE(copy_role,CASE WHEN template_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT' END),name,content_text,trigger_event,copies,paper_width,COALESCE(layout_json,'{}'),status,DATE_FORMAT(updated_at,'%Y-%m-%d %H:%i:%s')
		FROM print_templates`+where+" ORDER BY FIELD(business_type,'DINE_IN','TAKEOUT','DELIVERY'),FIELD(template_type,'RECEIPT','LABEL'),FIELD(copy_role,'MERCHANT','CUSTOMER','KITCHEN','ITEM'),id", args...)
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
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO print_templates(tenant_id,store_id,business_type,template_type,copy_role,name,content_text,trigger_event,copies,paper_width,layout_json,status)
		SELECT ?,id,?,?,?,?,?,?,?,?,?,? FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL
		ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id),name=VALUES(name),content_text=VALUES(content_text),trigger_event=VALUES(trigger_event),copies=VALUES(copies),paper_width=VALUES(paper_width),layout_json=VALUES(layout_json),status=VALUES(status),deleted_at=NULL`, identity.TenantID, input.BusinessType, input.TemplateType, input.CopyRole, input.Name, input.Content, input.TriggerEvent, input.Copies, input.PaperWidth, input.LayoutJSON, input.Status, storeID, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	if id == 0 {
		writeError(w, http.StatusNotFound, "STORE_NOT_FOUND", "store not found")
		return
	}
	s.audit(r.Context(), identity, "print_template.upsert", "print_template", int64String(id), map[string]any{"business_type": input.BusinessType, "template_type": input.TemplateType, "copy_role": input.CopyRole}, r)
	s.getPrintTemplateByID(w, r, identity.TenantID, id)
}

func scanPrintTemplate(row scanner, item *printTemplateDTO) error {
	var layoutJSON string
	if err := row.Scan(&item.ID, &item.StoreID, &item.BusinessType, &item.TemplateType, &item.CopyRole, &item.Name, &item.Content, &item.TriggerEvent, &item.Copies, &item.PaperWidth, &layoutJSON, &item.Status, &item.UpdatedAt); err != nil {
		return err
	}
	item.Layout = map[string]any{}
	if strings.TrimSpace(layoutJSON) != "" {
		if err := json.Unmarshal([]byte(layoutJSON), &item.Layout); err != nil {
			return err
		}
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
	err := scanPrintTemplate(s.DB.QueryRowContext(r.Context(), `SELECT id,store_id,business_type,template_type,COALESCE(copy_role,CASE WHEN template_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT' END),name,content_text,trigger_event,copies,paper_width,COALESCE(layout_json,'{}'),status,DATE_FORMAT(updated_at,'%Y-%m-%d %H:%i:%s')
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
	_, err := s.DB.ExecContext(r.Context(), `UPDATE print_templates SET business_type=?,template_type=?,copy_role=?,name=?,content_text=?,trigger_event=?,copies=?,paper_width=?,layout_json=?,status=?
		WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, input.BusinessType, input.TemplateType, input.CopyRole, input.Name, input.Content, input.TriggerEvent, input.Copies, input.PaperWidth, input.LayoutJSON, input.Status, id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "print_template.update", "print_template", int64String(id), map[string]any{"business_type": input.BusinessType, "template_type": input.TemplateType, "copy_role": input.CopyRole}, r)
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
	ID            int64
	TemplateType  string
	CopyRole      string
	Content       string
	TriggerEvent  string
	Copies        int
	PaperWidth    int
	LabelWidthMM  int
	LabelHeightMM int
	Layout        map[string]any
	Found         bool
	Enabled       bool
}

func loadPrintTemplates(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64, businessType, templateType string) ([]activePrintTemplate, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT id,template_type,COALESCE(copy_role,CASE WHEN template_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT' END),content_text,trigger_event,copies,paper_width,COALESCE(layout_json,'{}'),status
		FROM print_templates WHERE tenant_id=? AND store_id=? AND business_type=? AND template_type=? AND deleted_at IS NULL
		ORDER BY FIELD(copy_role,'MERCHANT','CUSTOMER','KITCHEN','ITEM'),id`, tenantID, storeID, businessType, templateType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []activePrintTemplate{}
	for rows.Next() {
		var item activePrintTemplate
		var layoutJSON, status string
		if err = rows.Scan(&item.ID, &item.TemplateType, &item.CopyRole, &item.Content, &item.TriggerEvent, &item.Copies, &item.PaperWidth, &layoutJSON, &status); err != nil {
			return nil, err
		}
		item.Found = true
		item.Enabled = status == "ACTIVE"
		item.Layout = map[string]any{}
		if strings.TrimSpace(layoutJSON) != "" {
			if err = json.Unmarshal([]byte(layoutJSON), &item.Layout); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
