package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

type printerDTO struct {
	ID                int64      `json:"id"`
	StoreID           int64      `json:"store_id"`
	Name              string     `json:"name"`
	Provider          string     `json:"provider"`
	Model             string     `json:"model"`
	SN                string     `json:"sn"`
	PaperWidth        int        `json:"paper_width"`
	LabelWidthMM      int        `json:"label_width_mm"`
	LabelHeightMM     int        `json:"label_height_mm"`
	PrintTrigger      string     `json:"print_trigger"`
	OutputType        string     `json:"output_type"`
	CopyRoles         []string   `json:"copyRoles"`
	LegacyCopyRoles   []string   `json:"copy_roles"`
	TemplateText      string     `json:"template_text"`
	Status            string     `json:"status"`
	ConnectionStatus  string     `json:"connection_status"`
	ConnectionMessage string     `json:"connection_message,omitempty"`
	StatusCheckedAt   *time.Time `json:"status_checked_at,omitempty"`
	LastSeenAt        *time.Time `json:"last_seen_at,omitempty"`
}

type printerInput struct {
	StoreID           int64    `json:"store_id"`
	Name              string   `json:"name"`
	Provider          string   `json:"provider"`
	Model             string   `json:"model"`
	SN                string   `json:"sn"`
	PaperWidth        int      `json:"paper_width"`
	LabelWidthMM      int      `json:"label_width_mm"`
	LabelHeightMM     int      `json:"label_height_mm"`
	PrintTrigger      string   `json:"print_trigger"`
	OutputType        string   `json:"output_type"`
	OutputTypeUI      string   `json:"outputType"`
	CopyRoles         []string `json:"copyRoles"`
	LegacyCopyRoles   []string `json:"copy_roles"`
	CopyRolesDatabase string   `json:"-"`
	TemplateText      string   `json:"template_text"`
	Status            string   `json:"status"`
}

func nullablePositiveInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (s *Server) listPrinters(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,store_id,name,provider,model,sn,paper_width,label_width_mm,label_height_mm,print_trigger,output_type,copy_roles,template_text,status FROM printer_devices WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id DESC`, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []printerDTO{}
	for rows.Next() {
		var item printerDTO
		if err := scanPrinter(rows, &item); err != nil {
			handleSQLError(w, err)
			return
		}
		s.resolvePrinterConnection(r.Context(), &item)
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createPrinter(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input printerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name == "" || input.SN == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name and sn are required")
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
	if err := normalizePrinterInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO printer_devices(tenant_id,store_id,name,provider,model,sn,paper_width,label_width_mm,label_height_mm,print_trigger,output_type,copy_roles,template_text,status)
		SELECT ?,id,?,?,?,?,?,?,?,?,?,?,?,? FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, identity.TenantID, input.Name, input.Provider, input.Model, input.SN, input.PaperWidth, nullablePositiveInt(input.LabelWidthMM), nullablePositiveInt(input.LabelHeightMM), input.PrintTrigger, input.OutputType, input.CopyRolesDatabase, input.TemplateText, input.Status, storeID, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	_ = s.syncPrinterRegistration(r.Context(), input.Provider, input.Name, input.SN)
	s.audit(r.Context(), identity, "printer.create", "printer", int64String(id), map[string]any{"sn": input.SN, "provider": input.Provider, "copy_roles": input.CopyRoles}, r)
	s.getPrinterByID(w, r, identity.TenantID, id)
}

func normalizePrinterInput(input *printerInput) error {
	if input.Provider == "" {
		input.Provider = "mock"
	}
	if input.PaperWidth == 0 {
		input.PaperWidth = 58
	}
	if input.PrintTrigger == "" {
		input.PrintTrigger = "PAYMENT_SUCCESS"
	}
	if input.OutputType == "" {
		input.OutputType = input.OutputTypeUI
	}
	if input.OutputType == "" {
		input.OutputType = "RECEIPT"
	}
	if input.TemplateText == "" {
		input.TemplateText = "订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分"
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	input.Provider = strings.ToLower(input.Provider)
	input.PrintTrigger = strings.ToUpper(input.PrintTrigger)
	input.OutputType = strings.ToUpper(input.OutputType)
	input.Status = strings.ToUpper(input.Status)
	if !validStatus(input.OutputType, "RECEIPT", "LABEL") {
		return errors.New("output_type must be RECEIPT or LABEL")
	}
	if input.OutputType == "LABEL" {
		if input.LabelWidthMM < 20 || input.LabelWidthMM > 110 {
			return errors.New("label_width_mm must be between 20 and 110")
		}
		if input.LabelHeightMM < 20 || input.LabelHeightMM > 200 {
			return errors.New("label_height_mm must be between 20 and 200")
		}
	} else {
		input.LabelWidthMM = 0
		input.LabelHeightMM = 0
	}
	if input.PaperWidth != 58 && input.PaperWidth != 80 {
		return errors.New("paper_width must be 58 or 80")
	}
	if !validStatus(input.PrintTrigger, "ORDER_CREATED", "PAYMENT_SUCCESS") {
		return errors.New("print_trigger must be ORDER_CREATED or PAYMENT_SUCCESS")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	roles := input.CopyRoles
	if len(roles) == 0 {
		roles = input.LegacyCopyRoles
	}
	normalizedRoles, err := normalizePrinterCopyRoles(input.OutputType, roles)
	if err != nil {
		return err
	}
	input.CopyRoles = normalizedRoles
	input.LegacyCopyRoles = append([]string(nil), normalizedRoles...)
	input.CopyRolesDatabase = strings.Join(normalizedRoles, ",")
	return nil
}

func normalizePrinterCopyRoles(outputType string, roles []string) ([]string, error) {
	if len(roles) == 0 {
		if outputType == "LABEL" {
			return []string{"ITEM"}, nil
		}
		return []string{"MERCHANT"}, nil
	}
	seen := map[string]bool{}
	for _, role := range roles {
		role = strings.ToUpper(strings.TrimSpace(role))
		if role == "" {
			continue
		}
		if outputType == "LABEL" && role != "ITEM" {
			return nil, errors.New("LABEL printer copyRoles must contain ITEM only")
		}
		if outputType == "RECEIPT" && !validStatus(role, "MERCHANT", "CUSTOMER", "KITCHEN") {
			return nil, errors.New("RECEIPT printer copyRoles must contain MERCHANT, CUSTOMER or KITCHEN")
		}
		seen[role] = true
	}
	order := []string{"MERCHANT", "CUSTOMER", "KITCHEN"}
	if outputType == "LABEL" {
		order = []string{"ITEM"}
	}
	result := make([]string, 0, len(seen))
	for _, role := range order {
		if seen[role] {
			result = append(result, role)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("copyRoles must not be empty")
	}
	return result, nil
}

func (s *Server) getPrinter(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "printerID")
	if ok {
		s.getPrinterByID(w, r, currentIdentity(r.Context()).TenantID, id)
	}
}

func (s *Server) getPrinterByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var item printerDTO
	if err := scanPrinter(s.DB.QueryRowContext(r.Context(), `SELECT id,store_id,name,provider,model,sn,paper_width,label_width_mm,label_height_mm,print_trigger,output_type,copy_roles,template_text,status FROM printer_devices WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, id, tenantID), &item); err != nil {
		handleSQLError(w, err)
		return
	}
	s.resolvePrinterConnection(r.Context(), &item)
	writeData(w, http.StatusOK, item)
}

func (s *Server) updatePrinter(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "printerID")
	if !ok {
		return
	}
	var input printerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizePrinterInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), `UPDATE printer_devices SET name=?,provider=?,model=?,sn=?,paper_width=?,label_width_mm=?,label_height_mm=?,print_trigger=?,output_type=?,copy_roles=?,template_text=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, input.Name, input.Provider, input.Model, input.SN, input.PaperWidth, nullablePositiveInt(input.LabelWidthMM), nullablePositiveInt(input.LabelHeightMM), input.PrintTrigger, input.OutputType, input.CopyRolesDatabase, input.TemplateText, input.Status, id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, rowsErr := result.RowsAffected(); rowsErr != nil {
		handleSQLError(w, rowsErr)
		return
	} else if n == 0 {
		// MySQL reports changed rows by default, so saving an unchanged printer
		// legitimately returns zero. Query the scoped record before deciding that
		// it does not exist.
		var exists int
		if err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM printer_devices WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID).Scan(&exists); err != nil {
			handleSQLError(w, err)
			return
		}
		if exists == 0 {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "printer not found")
			return
		}
	}
	_ = s.syncPrinterRegistration(r.Context(), input.Provider, input.Name, input.SN)
	s.audit(r.Context(), identity, "printer.update", "printer", int64String(id), map[string]any{"sn": input.SN, "copy_roles": input.CopyRoles}, r)
	s.getPrinterByID(w, r, identity.TenantID, id)
}

func (s *Server) deletePrinter(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "printerID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), "UPDATE printer_devices SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "printer not found")
		return
	}
	s.audit(r.Context(), identity, "printer.delete", "printer", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) testPrinter(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "printerID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var device printerDTO
	if err := scanPrinter(s.DB.QueryRowContext(r.Context(), `SELECT id,store_id,name,provider,model,sn,paper_width,label_width_mm,label_height_mm,print_trigger,output_type,copy_roles,template_text,status FROM printer_devices WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, id, identity.TenantID), &device); err != nil {
		handleSQLError(w, err)
		return
	}
	result, err := s.Printer.Print(r.Context(), provider.PrintRequest{Provider: device.Provider, DeviceSN: device.SN, DeviceType: device.Model, OutputType: device.OutputType, LabelWidthMM: device.LabelWidthMM, LabelHeightMM: device.LabelHeightMM, Content: "摊伴打印机测试\n设备：" + device.Name})
	if err != nil {
		writeError(w, http.StatusBadGateway, "PRINTER_PROVIDER_ERROR", err.Error())
		return
	}
	s.audit(r.Context(), identity, "printer.test", "printer", int64String(id), result, r)
	writeData(w, http.StatusOK, result)
}

func scanPrinter(row scanner, item *printerDTO) error {
	var copyRoles sql.NullString
	var labelWidth, labelHeight sql.NullInt64
	if err := row.Scan(&item.ID, &item.StoreID, &item.Name, &item.Provider, &item.Model, &item.SN, &item.PaperWidth, &labelWidth, &labelHeight, &item.PrintTrigger, &item.OutputType, &copyRoles, &item.TemplateText, &item.Status); err != nil {
		return err
	}
	item.LabelWidthMM = int(labelWidth.Int64)
	item.LabelHeightMM = int(labelHeight.Int64)
	roles := []string{}
	if copyRoles.Valid {
		roles = strings.Split(copyRoles.String, ",")
	}
	normalized, err := normalizePrinterCopyRoles(item.OutputType, roles)
	if err != nil {
		return err
	}
	item.CopyRoles = normalized
	item.LegacyCopyRoles = append([]string(nil), normalized...)
	return nil
}

func (s *Server) resolvePrinterConnection(ctx context.Context, item *printerDTO) {
	checkedAt := time.Now()
	if item.Status == "DISABLED" {
		item.ConnectionStatus = "DISABLED"
		item.ConnectionMessage = "设备已在系统中停用"
		item.StatusCheckedAt = &checkedAt
		return
	}
	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result := s.Printer.Status(statusCtx, provider.PrinterStatusRequest{Provider: item.Provider, DeviceSN: item.SN})
	item.ConnectionStatus = result.Status
	item.ConnectionMessage = result.Message
	item.StatusCheckedAt = &result.CheckedAt
	if result.Status == "ONLINE" || result.Status == "PAPER_OUT" {
		item.LastSeenAt = &result.CheckedAt
	}
}

func (s *Server) enqueueOrderPrints(ctx context.Context, tenantID, storeID, orderID int64, event string, reprint bool, actorID int64, extra string) error {
	return s.enqueueOrderPrintsWith(ctx, s.DB, tenantID, storeID, orderID, event, reprint, actorID, extra)
}

type sqlQueryExecer interface {
	sqlQueryer
	sqlExecer
}

type storePrintPolicy struct {
	AutoReceipt bool
	AutoLabel   bool
	Phone       string
}

func loadStorePrintPolicy(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64) (storePrintPolicy, error) {
	var policy storePrintPolicy
	var status string
	var deletedAt sql.NullTime
	err := queryer.QueryRowContext(ctx, `SELECT auto_print_receipt,auto_print_label,COALESCE(phone,''),status,deleted_at FROM stores
		WHERE id=? AND tenant_id=?`, storeID, tenantID).
		Scan(&policy.AutoReceipt, &policy.AutoLabel, &policy.Phone, &status, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// Payment recognition is the money fact and must not roll back merely
		// because the store disappeared from the printable scope meanwhile.
		return storePrintPolicy{}, nil
	}
	if err == nil && (status != "ACTIVE" || deletedAt.Valid) {
		return storePrintPolicy{}, nil
	}
	return policy, err
}

func storePolicyAllowsPrint(policy storePrintPolicy, outputType, event string) bool {
	// A manual reprint is an explicit operator action and must remain available
	// even when automatic printing is disabled for the store.
	if event == "REPRINT" {
		return true
	}
	if outputType == "LABEL" {
		return policy.AutoLabel
	}
	return policy.AutoReceipt
}

func (s *Server) enqueueOrderPrintsWith(ctx context.Context, executor sqlQueryExecer, tenantID, storeID, orderID int64, event string, reprint bool, actorID int64, extra string) error {
	return s.enqueueOrderPrintsWithOutput(ctx, executor, tenantID, storeID, orderID, event, reprint, actorID, extra, "")
}

func (s *Server) enqueueOrderPrintsWithOutput(ctx context.Context, executor sqlQueryExecer, tenantID, storeID, orderID int64, event string, reprint bool, actorID int64, extra, outputType string) error {
	order, err := s.loadOrderWith(ctx, executor, tenantID, orderID, "")
	if err != nil {
		return err
	}
	if err = ensureDefaultPrintTemplates(ctx, executor, tenantID, storeID); err != nil {
		return err
	}
	policy, err := loadStorePrintPolicy(ctx, executor, tenantID, storeID)
	if err != nil {
		return err
	}
	rows, err := executor.QueryContext(ctx, `SELECT id,store_id,name,provider,model,sn,paper_width,label_width_mm,label_height_mm,print_trigger,output_type,copy_roles,template_text,status FROM printer_devices WHERE tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL`, tenantID, storeID)
	if err != nil {
		return err
	}
	var devices []printerDTO
	for rows.Next() {
		var device printerDTO
		if err := scanPrinter(rows, &device); err != nil {
			return err
		}
		devices = append(devices, device)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, device := range devices {
		if !storePolicyAllowsPrint(policy, device.OutputType, event) {
			continue
		}
		if outputType != "" && device.OutputType != outputType {
			continue
		}
		templates, templateErr := loadPrintTemplates(ctx, executor, tenantID, storeID, order.OrderType, device.OutputType)
		if templateErr != nil {
			return templateErr
		}
		if len(templates) == 0 {
			copyRole := "MERCHANT"
			if device.OutputType == "LABEL" {
				copyRole = "ITEM"
			}
			templates = []activePrintTemplate{{CopyRole: copyRole, TemplateType: device.OutputType, Content: device.TemplateText, TriggerEvent: device.PrintTrigger, Copies: 1, PaperWidth: device.PaperWidth}}
		}
		for _, template := range templates {
			if !printerAllowsCopyRole(device, template.CopyRole) {
				continue
			}
			contentTemplate, copies, shouldPrint := resolvePrintPlan(device, template, event)
			if !shouldPrint {
				continue
			}
			paperWidth := device.PaperWidth
			if paperWidth != 58 && paperWidth != 80 {
				paperWidth = 58
			}
			renderTemplate := template
			renderTemplate.PaperWidth = paperWidth
			renderTemplate.LabelWidthMM = device.LabelWidthMM
			renderTemplate.LabelHeightMM = device.LabelHeightMM
			renderTemplate.StorePhone = policy.Phone
			contents := renderTemplateContents(device.OutputType, contentTemplate, renderTemplate, order, extra, reprint)
			for _, content := range contents {
				for copyNo := 0; copyNo < copies; copyNo++ {
					_, err := executor.ExecContext(ctx, `INSERT INTO print_jobs(tenant_id,store_id,order_id,printer_id,template_id,copy_role,paper_width,content_text,status,is_reprint,created_by) VALUES(?,?,?,?,?,?,?,?,'PENDING',?,?)`, tenantID, storeID, orderID, device.ID, nullableID(template.ID), template.CopyRole, paperWidth, content, reprint, actorID)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func printerAllowsCopyRole(device printerDTO, copyRole string) bool {
	for _, allowed := range device.CopyRoles {
		if allowed == copyRole {
			return true
		}
	}
	return false
}

func resolvePrintPlan(device printerDTO, template activePrintTemplate, event string) (content string, copies int, shouldPrint bool) {
	if template.Found {
		if !template.Enabled {
			return "", 0, false
		}
		if event != "REFUND" && event != "REPRINT" && template.TriggerEvent != event {
			return "", 0, false
		}
		copies = template.Copies
		if copies < 1 {
			copies = 1
		}
		return template.Content, copies, true
	}
	if event != "REFUND" && event != "REPRINT" && device.PrintTrigger != event {
		return "", 0, false
	}
	return device.TemplateText, 1, true
}

func (s *Server) StartPrintWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.processPendingPrintOutbox(ctx)
				s.dispatchPendingPrintJobs(ctx)
			}
		}
	}()
}

func (s *Server) dispatchPendingPrintJobs(ctx context.Context) {
	_, _ = s.DB.ExecContext(ctx, "UPDATE print_jobs SET status='PENDING',error_message='worker interrupted; retrying' WHERE status='PROCESSING' AND attempts<5 AND updated_at<DATE_SUB(NOW(3), INTERVAL 2 MINUTE)")
	_, _ = s.DB.ExecContext(ctx, "UPDATE print_jobs SET status='FAILED',error_message='retry limit reached' WHERE status='PROCESSING' AND attempts>=5 AND updated_at<DATE_SUB(NOW(3), INTERVAL 2 MINUTE)")
	_, _ = s.DB.ExecContext(ctx, "UPDATE print_jobs SET status='PENDING' WHERE status='FAILED' AND attempts<5 AND updated_at<DATE_SUB(NOW(3), INTERVAL 15 SECOND)")
	rows, err := s.DB.QueryContext(ctx, "SELECT id FROM print_jobs WHERE status='PENDING' ORDER BY id LIMIT 20")
	if err != nil {
		s.Logger.Error("list pending print jobs", "error", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := s.dispatchPrintJob(ctx, id); err != nil {
			s.Logger.Error("dispatch print job", "job_id", id, "error", err)
		}
	}
}

func (s *Server) dispatchPrintJob(ctx context.Context, id int64) error {
	claimed, err := s.DB.ExecContext(ctx, "UPDATE print_jobs SET status='PROCESSING',attempts=attempts+1 WHERE id=? AND status='PENDING'", id)
	if err != nil {
		return err
	}
	if count, _ := claimed.RowsAffected(); count != 1 {
		return nil
	}
	var providerName, sn, model, outputType, content string
	var labelWidthMM, labelHeightMM sql.NullInt64
	var reprint bool
	if err = s.DB.QueryRowContext(ctx, `SELECT d.provider,d.sn,d.model,d.output_type,d.label_width_mm,d.label_height_mm,j.content_text,j.is_reprint FROM print_jobs j JOIN printer_devices d ON d.id=j.printer_id WHERE j.id=?`, id).Scan(&providerName, &sn, &model, &outputType, &labelWidthMM, &labelHeightMM, &content, &reprint); err != nil {
		_, _ = s.DB.ExecContext(ctx, "UPDATE print_jobs SET status='FAILED',error_message=? WHERE id=?", err.Error(), id)
		return err
	}
	printCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	result, printErr := s.Printer.Print(printCtx, provider.PrintRequest{JobID: id, Provider: providerName, DeviceSN: sn, DeviceType: model, OutputType: outputType, LabelWidthMM: int(labelWidthMM.Int64), LabelHeightMM: int(labelHeightMM.Int64), Content: content, Reprint: reprint})
	if printErr != nil {
		status := "FAILED"
		if errors.Is(printErr, context.DeadlineExceeded) || errors.Is(printCtx.Err(), context.DeadlineExceeded) {
			// A timeout is an ambiguous provider outcome. Do not blindly resend and
			// risk duplicate labels; operators can inspect and explicitly retry.
			status = "UNKNOWN"
		}
		_, _ = s.DB.ExecContext(ctx, "UPDATE print_jobs SET status=?,error_message=? WHERE id=?", status, truncateError(printErr), id)
		return printErr
	}
	_, err = s.DB.ExecContext(ctx, "UPDATE print_jobs SET status='SUCCESS',provider_job_no=?,error_message='' WHERE id=?", result.ProviderJobNo, id)
	return err
}

func renderTicket(template string, order orderDTO, extra string, reprint bool) string {
	var itemLines []string
	for _, item := range order.Items {
		itemLines = append(itemLines, printableOrderItemLines(item, item.Quantity)...)
	}
	return renderOrderTemplate(template, order, strings.Join(itemLines, "\n"), extra, reprint)
}

func renderPrintContents(outputType, template string, order orderDTO, extra string, reprint bool) []string {
	if outputType != "LABEL" {
		return []string{renderTicket(template, order, extra, reprint)}
	}
	contents := []string{}
	total := printableLabelCount(order.Items)
	index := 0
	for _, item := range order.Items {
		for unit := 0; unit < item.Quantity; unit++ {
			index++
			contents = append(contents, renderLabel(template, order, item, index, total, extra, reprint))
		}
	}
	return contents
}

func renderTemplateContents(outputType, legacyContent string, template activePrintTemplate, order orderDTO, extra string, reprint bool) []string {
	if len(template.Layout) == 0 {
		return renderPrintContents(outputType, legacyContent, order, extra, reprint)
	}
	if outputType == "LABEL" {
		contents := []string{}
		total := printableLabelCount(order.Items)
		index := 0
		for _, item := range order.Items {
			for unit := 0; unit < item.Quantity; unit++ {
				index++
				contents = append(contents, renderStructuredLabel(template, order, item, index, total, extra, reprint))
			}
		}
		return contents
	}
	return []string{renderStructuredReceipt(template, order, extra, reprint)}
}

func renderStructuredReceipt(template activePrintTemplate, order orderDTO, extra string, reprint bool) string {
	fontSize := layoutString(template.Layout, "fontSize", "NORMAL")
	width := printableColumns(template.PaperWidth, fontSize)
	separator := strings.Repeat("-", width)
	output := []string{}
	appendReceiptCustomText(&output, layoutString(template.Layout, "customHeader", ""), order, width, fontSize)
	if reprint {
		appendReceiptMarkup(&output, "【补打】", "CENTER", true, true)
	}
	if layoutBool(template.Layout, "showStoreName", true) && order.StoreName != "" {
		appendReceiptMarkup(&output, order.StoreName, "CENTER", false, true)
	}
	pickupCode := printablePickupCode(order)
	copyTitle := layoutString(template.Layout, "copyTitle", copyRoleShortTitle(template.CopyRole))
	if layoutBool(template.Layout, "showPickupNo", true) && pickupCode != "" {
		headline := fmt.Sprintf("(%s)取餐码:%s", fitPrintText(copyTitle, 2), pickupCode)
		appendReceiptProminentLine(&output, headline, template.PaperWidth, false)
	} else {
		headline := "(" + fitPrintText(copyTitle, 2) + ")" + copyRoleTitle(template.CopyRole)
		appendReceiptProminentLine(&output, headline, template.PaperWidth, false)
	}
	appendReceiptMarkup(&output, separator, "LEFT", false, false)
	if layoutBool(template.Layout, "showOrderType", true) {
		appendReceiptBodyLines(&output, []string{printKeyValue("类型", orderTypeTitle(order.OrderType), width)}, fontSize)
	}
	if layoutBool(template.Layout, "showOrderNo", true) {
		appendReceiptBodyLines(&output, []string{printKeyValue("订单", order.OrderNo, width)}, fontSize)
	}
	if order.FastFoodPlate != nil {
		appendReceiptBodyLines(&output, []string{printKeyValue("码牌", strings.TrimSpace(order.FastFoodPlate.Name+" "+order.FastFoodPlate.PlateCode), width)}, fontSize)
	}
	if layoutBool(template.Layout, "showTable", true) && order.Table != nil {
		appendReceiptBodyLines(&output, []string{printKeyValue("桌台", strings.TrimSpace(order.Table.AreaName+" "+order.Table.Name+" "+order.Table.TableCode), width)}, fontSize)
	}
	if createdAt := printableOrderTime(order.CreatedAt); layoutBool(template.Layout, "showOrderTime", true) && createdAt != "" {
		appendReceiptBodyLines(&output, []string{printKeyValue("下单时间", createdAt, width)}, fontSize)
	}
	if layoutBool(template.Layout, "showCustomer", false) {
		customer := strings.TrimSpace(order.CustomerName + " " + order.CustomerPhone)
		if customer != "" {
			appendReceiptBodyLines(&output, []string{printKeyValue("顾客", customer, width)}, fontSize)
		}
	}
	if layoutBool(template.Layout, "showAddress", false) && order.OrderType == orderTypeDelivery {
		// Delivery addresses are intentionally omitted until the delivery-order
		// aggregate stores an immutable address snapshot.
		appendReceiptBodyLines(&output, []string{printKeyValue("地址", "待配送能力启用", width)}, fontSize)
	}
	if layoutBool(template.Layout, "showItems", true) {
		appendReceiptMarkup(&output, separator, "LEFT", false, false)
		showPrices := layoutBool(template.Layout, "showPrices", template.CopyRole != "KITCHEN")
		showOptions := layoutBool(template.Layout, "showItemOptions", true)
		showOptionGroupNames := layoutBool(template.Layout, "showOptionGroupNames", false)
		if layoutBool(template.Layout, "showItemHeader", true) {
			for _, line := range printReceiptItemHeader(width, showPrices) {
				appendReceiptMarkup(&output, line, "LEFT", strings.EqualFold(fontSize, "LARGE"), false)
			}
		}
		for _, item := range order.Items {
			name := printableText(strings.TrimSpace(item.ProductName + " " + item.SKUName))
			if showOptions {
				if options := printableItemOptions(item, showOptionGroupNames); len(options) > 0 {
					name += "（" + strings.Join(options, "，") + "）"
				}
			}
			for _, line := range wrapPrintText(name, width) {
				appendReceiptMarkup(&output, line, "LEFT", strings.EqualFold(fontSize, "LARGE"), false)
			}
			for _, line := range printReceiptItemValueLines(item.Quantity, item.UnitPriceCents, item.SubtotalCents, width, showPrices) {
				appendReceiptMarkup(&output, line, "LEFT", strings.EqualFold(fontSize, "LARGE"), true)
			}
			if showOptions {
				if modifiers := printableItemModifiers(item); len(modifiers) > 0 {
					appendReceiptBodyLines(&output, wrapPrintText("  加料："+strings.Join(modifiers, "、"), width), fontSize)
				}
			}
			if remark := printableText(item.ItemRemark); layoutBool(template.Layout, "showRemark", true) && remark != "" {
				appendReceiptBodyLines(&output, wrapPrintText("  备注："+remark, width), fontSize)
			}
		}
	}
	if layoutBool(template.Layout, "showPrices", template.CopyRole != "KITCHEN") {
		appendReceiptMarkup(&output, separator, "LEFT", false, false)
		appendReceiptBodyLines(&output, []string{printTwoColumns("合计", formatPrintAmount(order.TotalCents), width)}, fontSize)
	}
	if layoutBool(template.Layout, "showPayment", template.CopyRole != "KITCHEN") {
		if layoutBool(template.Layout, "emphasizePaid", true) {
			paidWidth := printableColumns(template.PaperWidth, "LARGE")
			appendReceiptMarkup(&output, printTwoColumns("实付", formatPrintAmount(order.PaidCents), paidWidth), "LEFT", true, false)
		} else {
			appendReceiptBodyLines(&output, []string{printTwoColumns("实付", formatPrintAmount(order.PaidCents), width)}, fontSize)
		}
		if method := printablePaymentMethod(order.Payment); method != "" {
			output = append(output, "<BR>")
			appendReceiptBodyLines(&output, []string{printKeyValue("支付", method, width)}, fontSize)
		}
	}
	if layoutBool(template.Layout, "showRemark", true) && printableText(order.Remark) != "" {
		appendReceiptBodyLines(&output, wrapPrintText("订单备注："+printableText(order.Remark), width), fontSize)
	}
	if layoutBool(template.Layout, "showQrCode", false) {
		appendReceiptBodyLines(&output, []string{printKeyValue("订单码", order.OrderNo, width)}, fontSize)
	}
	if extra != "" {
		appendReceiptBodyLines(&output, wrapPrintText(printableText(extra), width), fontSize)
	}
	customFooter := strings.TrimSpace(layoutString(template.Layout, "customFooter", ""))
	footerPrinted := customFooter != ""
	if footerPrinted {
		appendReceiptMarkup(&output, separator, "LEFT", false, false)
		appendReceiptCenteredCustomText(&output, customFooter, order, width, fontSize)
		if phone := printableText(template.StorePhone); phone != "" {
			appendReceiptMarkup(&output, "客服电话："+phone, "CENTER", false, false)
		}
	}
	if layoutBool(template.Layout, "showEndMarker", true) {
		output = append(output, "<BR>")
		endText := strings.TrimSpace(layoutString(template.Layout, "endMarkerText", ""))
		if endText != "" {
			endText = renderOrderTemplate(endText, order, "", "", false)
		} else if pickupCode != "" {
			endText = "--#" + pickupCode + "完--"
		} else {
			endText = "--" + fitPrintText(copyTitle, 2) + "联打印结束--"
		}
		appendReceiptProminentLine(&output, endText, template.PaperWidth, true)
	}
	for feedLine := 0; feedLine < layoutInteger(template.Layout, "feedLines", 3); feedLine++ {
		output = append(output, "<BR>")
	}
	// Every rendered row already ends in an explicit <BR>. Sending literal
	// newlines as separators makes XPrinter advance twice and produces the
	// oversized line spacing seen on physical receipts.
	return strings.Join(output, "")
}

func renderStructuredLabel(template activePrintTemplate, order orderDTO, item orderItemDTO, itemIndex, itemTotal int, extra string, reprint bool) string {
	widthMM := layoutInteger(template.Layout, "labelWidthMM", template.LabelWidthMM)
	heightMM := layoutInteger(template.Layout, "labelHeightMM", template.LabelHeightMM)
	if widthMM <= 0 {
		widthMM = 40
	}
	if heightMM <= 0 {
		heightMM = 30
	}
	widthDots, heightDots := widthMM*8, heightMM*8
	margin, y := 8, 8
	bottomY := heightDots - 28
	var output strings.Builder
	fmt.Fprintf(&output, `<PAGE l="2"><SIZE>%d,%d</SIZE>`, widthMM, heightMM)

	contextText := orderTypeShortTitle(order.OrderType)
	if layoutBool(template.Layout, "showTable", true) && order.Table != nil {
		contextText = "桌号：" + strings.TrimSpace(order.Table.TableCode+" "+order.Table.Name)
	} else if layoutBool(template.Layout, "showPickupNo", true) && printablePickupCode(order) != "" {
		contextText = "取餐：" + printablePickupCode(order)
	}
	if reprint {
		contextText = "补打 " + contextText
	}
	contextWidth := widthDots - margin*2
	if layoutBool(template.Layout, "showItemSequence", true) && itemTotal > 0 {
		sequence := fmt.Sprintf("数量：%d/%d", itemIndex, itemTotal)
		contextWidth -= labelTextDots(sequence, 1) + 8
		appendLabelRightText(&output, widthDots-margin, y, 1, 1, sequence)
	}
	appendLabelText(&output, margin, y, contextWidth, 1, 1, contextText)
	y += 32

	if customHeader := strings.TrimSpace(layoutString(template.Layout, "customHeader", "")); customHeader != "" && y+24 < bottomY {
		customHeader = renderOrderTemplate(customHeader, order, "", "", false)
		appendLabelText(&output, margin, y, widthDots-margin*2, 1, 1, customHeader)
		y += 28
	}
	if layoutBool(template.Layout, "showItems", true) {
		productHeight := 1
		if strings.EqualFold(layoutString(template.Layout, "fontSize", "NORMAL"), "LARGE") {
			productHeight = 2
		}
		appendLabelText(&output, margin, y, widthDots-margin*2, 1, productHeight, printableText(item.ProductName))
		y += 28*productHeight + 4
		if sku := printableText(item.SKUName); sku != "" && y+24 < bottomY {
			appendLabelText(&output, margin, y, widthDots-margin*2, 1, 1, "规格："+sku)
			y += 28
		}
		if layoutBool(template.Layout, "showItemOptions", true) && y+24 < bottomY {
			details := []string{}
			if options := printableItemOptions(item, layoutBool(template.Layout, "showOptionGroupNames", false)); len(options) > 0 {
				details = append(details, strings.Join(options, "、"))
			}
			if modifiers := printableItemModifiers(item); len(modifiers) > 0 {
				details = append(details, "加料："+strings.Join(modifiers, "、"))
			}
			if len(details) > 0 {
				appendLabelText(&output, margin, y, widthDots-margin*2, 1, 1, "属性："+strings.Join(details, "；"))
				y += 28
			}
		}
		if remark := printableText(item.ItemRemark); layoutBool(template.Layout, "showRemark", true) && remark != "" && y+24 < bottomY {
			appendLabelText(&output, margin, y, widthDots-margin*2, 1, 1, "备注："+remark)
			y += 28
		}
	}
	if layoutBool(template.Layout, "showOrderNo", true) && y+24 < bottomY {
		appendLabelText(&output, margin, y, widthDots-margin*2, 1, 1, "订单："+order.OrderNo)
		y += 28
	}
	if extra != "" && y+24 < bottomY {
		appendLabelText(&output, margin, y, widthDots-margin*2, 1, 1, printableText(extra))
		y += 28
	}
	if customFooter := strings.TrimSpace(layoutString(template.Layout, "customFooter", "")); customFooter != "" && y+24 < bottomY {
		customFooter = renderOrderTemplate(customFooter, order, "", "", false)
		appendLabelText(&output, margin, y, widthDots-margin*2, 1, 1, customFooter)
	}

	if layoutBool(template.Layout, "showOrderTime", true) {
		timeText := printableOrderTime(order.CreatedAt)
		if len(timeText) >= 16 {
			timeText = timeText[5:16]
		}
		appendLabelText(&output, margin, bottomY, widthDots/2-margin, 1, 1, timeText)
	}
	if layoutBool(template.Layout, "showOrderType", true) {
		appendLabelRightText(&output, widthDots-margin, bottomY, 1, 1, orderTypeShortTitle(order.OrderType))
	}
	output.WriteString("</PAGE>")
	return output.String()
}

func printableColumns(paperWidth int, fontSize string) int {
	columns := 32
	if paperWidth == 80 {
		columns = 48
	}
	if strings.EqualFold(fontSize, "LARGE") {
		columns /= 2
	}
	return columns
}

func copyRoleTitle(copyRole string) string {
	return map[string]string{"MERCHANT": "商家联", "CUSTOMER": "顾客联", "KITCHEN": "后厨联", "ITEM": "商品标签"}[copyRole]
}

func copyRoleShortTitle(copyRole string) string {
	return map[string]string{"MERCHANT": "商", "CUSTOMER": "客", "KITCHEN": "厨", "ITEM": "签"}[copyRole]
}

func orderTypeTitle(orderType string) string {
	return map[string]string{orderTypeDineIn: "店内堂食", orderTypeTakeout: "到店自取", orderTypeDelivery: "外卖配送"}[orderType]
}

func orderTypeShortTitle(orderType string) string {
	return map[string]string{orderTypeDineIn: "堂食", orderTypeTakeout: "自提", orderTypeDelivery: "外卖"}[orderType]
}

func printableOrderTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Replace(value, "T", " ", 1)
	return strings.TrimSuffix(value, "Z")
}

func printablePaymentMethod(value any) string {
	payment, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	providerName := strings.ToLower(strings.TrimSpace(fmt.Sprint(payment["provider"])))
	switch providerName {
	case "tianque":
		return "会生活 / 随行付"
	case "wechat_partner":
		return "微信支付"
	case "mock":
		return "模拟支付"
	case "":
		return ""
	default:
		return printableText(providerName)
	}
}

func layoutBool(layout map[string]any, key string, fallback bool) bool {
	value, ok := layout[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func layoutString(layout map[string]any, key, fallback string) string {
	value, ok := layout[key].(string)
	if !ok {
		return fallback
	}
	return value
}

func layoutInteger(layout map[string]any, key string, fallback int) int {
	value, ok := layoutInt(layout[key])
	if !ok {
		return fallback
	}
	return value
}

func appendReceiptBodyLines(output *[]string, lines []string, fontSize string) {
	large := strings.EqualFold(fontSize, "LARGE")
	for _, line := range nonEmptyPrintLines(lines) {
		appendReceiptMarkup(output, line, "LEFT", large, false)
	}
}

func appendReceiptCustomText(output *[]string, custom string, order orderDTO, width int, fontSize string) {
	lines := []string{}
	appendCustomPrintText(&lines, custom, order, width)
	appendReceiptBodyLines(output, lines, fontSize)
}

func appendReceiptCenteredCustomText(output *[]string, custom string, order orderDTO, width int, fontSize string) {
	lines := []string{}
	appendCustomPrintText(&lines, custom, order, width)
	large := strings.EqualFold(fontSize, "LARGE")
	for _, line := range nonEmptyPrintLines(lines) {
		appendReceiptMarkup(output, line, "CENTER", large, false)
	}
}

func appendReceiptProminentLine(output *[]string, value string, paperWidth int, bold bool) {
	value = fitPrintText(value, printableColumns(paperWidth, "NORMAL"))
	appendReceiptMarkup(output, value, "CENTER", printDisplayWidth(value) <= printableColumns(paperWidth, "LARGE"), bold)
}

func appendReceiptMarkup(output *[]string, value, align string, large, bold bool) {
	value = printableMarkupLine(value)
	if strings.TrimSpace(value) == "" {
		return
	}
	value = escapePrintMarkup(value)
	if bold {
		value = "<BOLD>" + value + "</BOLD>"
	}
	switch {
	case large && strings.EqualFold(align, "CENTER"):
		*output = append(*output, "<CB>"+value+"<BR></CB>")
	case large:
		*output = append(*output, "<L><B>"+value+"</B><BR></L>")
	case strings.EqualFold(align, "CENTER"):
		*output = append(*output, "<C>"+value+"<BR></C>")
	case strings.EqualFold(align, "RIGHT"):
		*output = append(*output, "<R>"+value+"<BR></R>")
	default:
		*output = append(*output, "<L>"+value+"<BR></L>")
	}
}

func printableMarkupLine(value string) string {
	value = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(value)
	return strings.TrimRightFunc(value, unicode.IsSpace)
}

func escapePrintMarkup(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
}

func appendLabelText(output *strings.Builder, x, y, maxWidthDots, widthScale, heightScale int, value string) {
	value = truncateLabelDots(printableText(value), maxWidthDots, widthScale)
	if value == "" {
		return
	}
	fmt.Fprintf(output, `<TEXT x="%d" y="%d" font="9" w="%d" h="%d" r="0">%s</TEXT>`,
		x, y, widthScale, heightScale, escapePrintMarkup(value))
}

func appendLabelRightText(output *strings.Builder, rightX, y, widthScale, heightScale int, value string) {
	value = printableText(value)
	width := labelTextDots(value, widthScale)
	x := rightX - width
	if x < 0 {
		x = 0
	}
	appendLabelText(output, x, y, rightX-x, widthScale, heightScale, value)
}

func truncateLabelDots(value string, maxDots, widthScale int) string {
	if maxDots <= 0 {
		return ""
	}
	var output strings.Builder
	used := 0
	for _, char := range value {
		charDots := 12 * widthScale
		if char > unicode.MaxASCII {
			charDots = 24 * widthScale
		}
		if used+charDots > maxDots {
			break
		}
		output.WriteRune(char)
		used += charDots
	}
	return strings.TrimSpace(output.String())
}

func labelTextDots(value string, widthScale int) int {
	dots := 0
	for _, char := range value {
		if char > unicode.MaxASCII {
			dots += 24 * widthScale
		} else {
			dots += 12 * widthScale
		}
	}
	return dots
}

func printHeader(value string, width int, style string) string {
	if strings.EqualFold(style, "SIMPLE") {
		return fitPrintText(value, width)
	}
	return centerPrintText("【"+value+"】", width)
}

func printKeyValue(key, value string, width int) string {
	return printTwoColumns(key+"：", printableText(value), width)
}

func printTwoColumns(left, right string, width int) string {
	left, right = printableText(left), printableText(right)
	rightWidth := printDisplayWidth(right)
	if rightWidth >= width {
		return fitPrintText(right, width)
	}
	available := width - rightWidth - 1
	left = fitPrintText(left, available)
	spaces := width - printDisplayWidth(left) - rightWidth
	if spaces < 1 {
		spaces = 1
	}
	return left + strings.Repeat(" ", spaces) + right
}

func printTwoColumnsWrapped(left, right string, width int) []string {
	left, right = printableText(left), printableText(right)
	rightWidth := printDisplayWidth(right)
	if rightWidth >= width {
		lines := wrapPrintText(left, width)
		return append(lines, fitPrintText(right, width))
	}
	leftWidth := width - rightWidth - 1
	leftLines := wrapPrintText(left, leftWidth)
	if len(leftLines) == 0 {
		return []string{fitPrintText(right, width)}
	}
	last := len(leftLines) - 1
	leftLines[last] = printTwoColumns(leftLines[last], right, width)
	return leftLines
}

func printReceiptItemHeader(width int, showPrices bool) []string {
	if !showPrices {
		return []string{printTwoColumns("商品", "数量", width)}
	}
	if width < 28 {
		return []string{
			"商品",
			printCompactReceiptValueColumns("数量", "单价", "金额", width),
		}
	}
	nameWidth, quantityWidth, unitWidth, amountWidth := receiptItemColumnWidths(width)
	return []string{joinReceiptColumns(
		printTableCell("商品", nameWidth, false),
		printTableCell("数量", quantityWidth, true),
		printTableCell("单价", unitWidth, true),
		printTableCell("金额", amountWidth, true),
	)}
}

func printReceiptItemValueLines(quantity int, unitCents, subtotalCents int64, width int, showPrices bool) []string {
	quantityText := "x" + strconv.Itoa(quantity)
	if !showPrices {
		return []string{printTwoColumns("", quantityText, width)}
	}
	if width < 28 {
		return []string{printCompactReceiptValueColumns(quantityText, formatPrintAmount(unitCents), formatPrintAmount(subtotalCents), width)}
	}
	nameWidth, quantityWidth, unitWidth, amountWidth := receiptItemColumnWidths(width)
	return []string{joinReceiptColumns(
		printTableCell("", nameWidth, false),
		printTableCell(quantityText, quantityWidth, true),
		printTableCell(formatPrintAmount(unitCents), unitWidth, true),
		printTableCell(formatPrintAmount(subtotalCents), amountWidth, true),
	)}
}

func receiptItemColumnWidths(width int) (name, quantity, unit, amount int) {
	quantity, unit, amount = 4, 7, 7
	name = width - quantity - unit - amount - 3
	return
}

func printTableCell(value string, width int, right bool) string {
	value = fitPrintText(value, width)
	padding := width - printDisplayWidth(value)
	if padding < 0 {
		padding = 0
	}
	if right {
		return strings.Repeat(" ", padding) + value
	}
	return value + strings.Repeat(" ", padding)
}

func joinReceiptColumns(columns ...string) string {
	return strings.Join(columns, " ")
}

func printCompactReceiptValueColumns(quantity, unit, amount string, width int) string {
	quantityWidth := 4
	unitWidth := (width - quantityWidth - 2) / 2
	amountWidth := width - quantityWidth - unitWidth - 2
	return joinReceiptColumns(
		printTableCell(quantity, quantityWidth, true),
		printTableCell(unit, unitWidth, true),
		printTableCell(amount, amountWidth, true),
	)
}

func centerPrintText(value string, width int) string {
	value = fitPrintText(printableText(value), width)
	padding := (width - printDisplayWidth(value)) / 2
	if padding < 0 {
		padding = 0
	}
	return strings.Repeat(" ", padding) + value
}

func fitPrintText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	var output strings.Builder
	used := 0
	for _, char := range printableText(value) {
		charWidth := printRuneWidth(char)
		if used+charWidth > width {
			break
		}
		output.WriteRune(char)
		used += charWidth
	}
	return output.String()
}

func wrapPrintText(value string, width int) []string {
	value = printableText(value)
	if value == "" {
		return nil
	}
	lines := []string{}
	var current strings.Builder
	used := 0
	for _, char := range value {
		charWidth := printRuneWidth(char)
		if used > 0 && used+charWidth > width {
			lines = append(lines, current.String())
			current.Reset()
			used = 0
		}
		current.WriteRune(char)
		used += charWidth
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func printDisplayWidth(value string) int {
	width := 0
	for _, char := range value {
		width += printRuneWidth(char)
	}
	return width
}

func printRuneWidth(char rune) int {
	if unicode.Is(unicode.Mn, char) {
		return 0
	}
	if char <= unicode.MaxASCII {
		return 1
	}
	return 2
}

func appendCustomPrintText(lines *[]string, custom string, order orderDTO, width int) {
	if strings.TrimSpace(custom) == "" {
		return
	}
	custom = renderOrderTemplate(custom, order, "", "", false)
	for _, rawLine := range strings.Split(custom, "\n") {
		*lines = append(*lines, wrapPrintText(rawLine, width)...)
	}
}

func nonEmptyPrintLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}

func renderLabel(template string, order orderDTO, item orderItemDTO, itemIndex, itemTotal int, extra string, reprint bool) string {
	options := printableItemOptions(item, false)
	modifiers := printableItemModifiers(item)
	content := renderOrderTemplate(template, order, strings.Join(printableOrderItemLines(item, 1), "\n"), extra, reprint)
	return strings.NewReplacer(
		"{{product_name}}", printableText(item.ProductName),
		"{{sku_name}}", printableText(item.SKUName),
		"{{quantity}}", "1",
		"{{ordered_quantity}}", strconv.Itoa(item.Quantity),
		"{{item_index}}", strconv.Itoa(itemIndex),
		"{{item_total}}", strconv.Itoa(itemTotal),
		"{{item_sequence}}", fmt.Sprintf("%d/%d", itemIndex, itemTotal),
		"{{options}}", strings.Join(options, "、"),
		"{{modifiers}}", strings.Join(modifiers, "、"),
		"{{item_remark}}", printableText(item.ItemRemark),
	).Replace(content)
}

func printableLabelCount(items []orderItemDTO) int {
	total := 0
	for _, item := range items {
		if item.Quantity > 0 {
			total += item.Quantity
		}
	}
	return total
}

func renderOrderTemplate(template string, order orderDTO, items, extra string, reprint bool) string {
	tableName, tableArea, tableCode := "", "", ""
	plateName, plateCode := "", ""
	if order.Table != nil {
		tableName, tableArea, tableCode = order.Table.Name, order.Table.AreaName, order.Table.TableCode
	}
	if order.FastFoodPlate != nil {
		plateName, plateCode = order.FastFoodPlate.Name, order.FastFoodPlate.PlateCode
	}
	replacer := strings.NewReplacer(
		"{{store_name}}", printableText(order.StoreName),
		"{{order_no}}", order.OrderNo,
		"{{pickup_no}}", printablePickupCode(order),
		"{{order_type}}", order.OrderType,
		"{{paid_amount}}", formatPrintAmount(order.PaidCents),
		"{{paid_cents}}", int64String(order.PaidCents),
		"{{table_name}}", tableName,
		"{{table_area}}", tableArea,
		"{{table_code}}", tableCode,
		"{{fast_food_plate_name}}", plateName,
		"{{fast_food_plate_code}}", plateCode,
		"{{items}}", items,
		"{{total_cents}}", int64String(order.TotalCents),
		"{{remark}}", printableText(order.Remark),
	)
	content := replacer.Replace(template)
	if reprint {
		content = "【补打】\n" + content
	}
	if extra != "" {
		content += "\n" + extra
	}
	return content
}

func printablePickupCode(order orderDTO) string {
	if strings.TrimSpace(order.PickupCode) != "" {
		return strings.TrimSpace(order.PickupCode)
	}
	// Historical TAKEOUT rows created before migration 014 did not persist a
	// pickup number. Keep only that read-time compatibility path; all new
	// orders receive an immutable business-day sequence in the transaction.
	if order.BusinessDate == "" && order.ID > 0 {
		return fmt.Sprintf("%04d", order.ID%10000)
	}
	return ""
}

func printableOrderItemLines(item orderItemDTO, quantity int) []string {
	lines := []string{fmt.Sprintf("%s %s x%d", printableText(item.ProductName), printableText(item.SKUName), quantity)}
	for _, option := range printableItemOptions(item, false) {
		lines = append(lines, "  "+option)
	}
	if modifiers := printableItemModifiers(item); len(modifiers) > 0 {
		lines = append(lines, "  加料："+strings.Join(modifiers, "、"))
	}
	if remark := printableText(item.ItemRemark); remark != "" {
		lines = append(lines, "  单品备注："+remark)
	}
	return lines
}

func printableItemOptions(item orderItemDTO, showGroupNames bool) []string {
	result := []string{}
	options, _ := item.Configuration["options"].([]any)
	for _, raw := range options {
		option, _ := raw.(map[string]any)
		groupName := printableText(fmt.Sprint(option["groupName"]))
		valueName := printableText(fmt.Sprint(option["valueName"]))
		if valueName == "" || valueName == "<nil>" {
			continue
		}
		if showGroupNames && groupName != "" && groupName != "<nil>" {
			result = append(result, groupName+"："+valueName)
		} else {
			result = append(result, valueName)
		}
	}
	return result
}

func printableItemModifiers(item orderItemDTO) []string {
	result := []string{}
	modifiers, _ := item.Configuration["modifiers"].([]any)
	for _, raw := range modifiers {
		modifier, _ := raw.(map[string]any)
		name := printableText(fmt.Sprint(modifier["name"]))
		if name == "" || name == "<nil>" {
			continue
		}
		quantity := 1
		if value, ok := modifier["quantity"].(float64); ok && value > 1 {
			quantity = int(value)
		}
		if quantity > 1 {
			name += fmt.Sprintf("x%d", quantity)
		}
		result = append(result, name)
	}
	return result
}

func formatPrintAmount(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func printableText(value string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(value))
}

type reprintOrderInput struct {
	Type          string `json:"type"`
	OutputType    string `json:"output_type"`
	BusinessType  string `json:"business_type"`
	MarkAsReprint bool   `json:"markAsReprint"`
}

func (s *Server) reprintOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	input := reprintOrderInput{}
	if r.ContentLength != 0 && !decodeJSON(w, r, &input) {
		return
	}
	outputType := strings.ToUpper(strings.TrimSpace(input.OutputType))
	if outputType == "" {
		outputType = strings.ToUpper(strings.TrimSpace(input.Type))
	}
	if outputType == "" {
		outputType = "RECEIPT"
	}
	if !validStatus(outputType, "RECEIPT", "LABEL") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "type must be RECEIPT or LABEL")
		return
	}
	var storeID int64
	if err := s.DB.QueryRowContext(r.Context(), "SELECT store_id FROM orders WHERE id=? AND tenant_id=?", id, identity.TenantID).Scan(&storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err := s.enqueueOrderPrintsWithOutput(r.Context(), s.DB, identity.TenantID, storeID, id, "REPRINT", true, identity.UserID, "", outputType); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "order.reprint", "order", int64String(id), map[string]any{"output_type": outputType}, r)
	writeData(w, http.StatusOK, map[string]any{"queued": true, "reprint": true, "output_type": outputType})
}

func (s *Server) listPrintJobs(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var total int
	if err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM print_jobs WHERE tenant_id=? AND store_id=?", identity.TenantID, storeID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT j.id,j.order_id,o.order_no,j.printer_id,j.template_id,j.copy_role,j.paper_width,d.name,d.output_type,j.provider_job_no,j.status,j.attempts,j.is_reprint,j.reprint_of,j.error_message,DATE_FORMAT(j.created_at,'%Y-%m-%d %H:%i:%s')
		FROM print_jobs j JOIN orders o ON o.id=j.order_id AND o.tenant_id=j.tenant_id AND o.store_id=j.store_id
		JOIN printer_devices d ON d.id=j.printer_id AND d.tenant_id=j.tenant_id AND d.store_id=j.store_id
		WHERE j.tenant_id=? AND j.store_id=? ORDER BY j.id DESC LIMIT ? OFFSET ?`, identity.TenantID, storeID, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, orderID, printerID int64
		var templateID sql.NullInt64
		var paperWidth int
		var orderNo, copyRole, printerName, outputType, providerNo, status, errorMessage, created string
		var attempts int
		var reprint bool
		var reprintOf sql.NullInt64
		if err := rows.Scan(&id, &orderID, &orderNo, &printerID, &templateID, &copyRole, &paperWidth, &printerName, &outputType, &providerNo, &status, &attempts, &reprint, &reprintOf, &errorMessage, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "order_id": orderID, "orderNo": orderNo, "order_no": orderNo, "printer_id": printerID, "templateId": templateID.Int64, "template_id": templateID.Int64, "copyRole": copyRole, "copy_role": copyRole, "paperWidth": paperWidth, "paper_width": paperWidth, "printerName": printerName, "printer_name": printerName, "type": outputType, "output_type": outputType, "provider_job_no": providerNo, "status": status, "attempts": attempts, "is_reprint": reprint, "reprint_of": reprintOf.Int64, "error_message": errorMessage, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) retryPrintJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "jobID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	reprintID, err := clonePrintJobForReprint(r.Context(), s.DB, identity.TenantID, storeID, id, identity.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "print job not found or is still processing")
			return
		}
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "print_job.reprint", "print_job", int64String(reprintID), map[string]any{"reprint_of": id}, r)
	writeData(w, http.StatusAccepted, map[string]any{"id": reprintID, "status": "PENDING", "is_reprint": true, "reprint_of": id})
}

func clonePrintJobForReprint(ctx context.Context, executor sqlQueryExecer, tenantID, storeID, sourceID, actorID int64) (int64, error) {
	var orderID, printerID int64
	var templateID sql.NullInt64
	var copyRole, content, outputType string
	var paperWidth int
	err := executor.QueryRowContext(ctx, `SELECT j.order_id,j.printer_id,j.template_id,j.copy_role,j.paper_width,j.content_text,d.output_type
		FROM print_jobs j JOIN printer_devices d ON d.id=j.printer_id AND d.tenant_id=j.tenant_id AND d.store_id=j.store_id
		WHERE j.id=? AND j.tenant_id=? AND j.store_id=? AND j.status IN ('SUCCESS','FAILED','UNKNOWN')`,
		sourceID, tenantID, storeID).Scan(&orderID, &printerID, &templateID, &copyRole, &paperWidth, &content, &outputType)
	if err != nil {
		return 0, err
	}
	content = markPrintJobContentAsReprint(content, outputType)
	result, err := executor.ExecContext(ctx, `INSERT INTO print_jobs(tenant_id,store_id,order_id,printer_id,template_id,copy_role,paper_width,content_text,status,attempts,is_reprint,reprint_of,error_message,created_by)
		VALUES(?,?,?,?,?,?,?,?,'PENDING',0,1,?,'',?)`,
		tenantID, storeID, orderID, printerID, nullableID(templateID.Int64), copyRole, paperWidth, content, sourceID, actorID)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func markPrintJobContentAsReprint(content, outputType string) string {
	if strings.Contains(content, "【补打】") {
		return content
	}
	content = strings.NewReplacer("¥", "", "￥", "").Replace(content)
	if strings.EqualFold(outputType, "RECEIPT") && strings.Contains(strings.ToUpper(content), "<BR>") {
		content = strings.ReplaceAll(strings.ReplaceAll(content, "\r", ""), "\n", "")
		content = compactStoredPickupHeadline(content)
		content = strings.ReplaceAll(content, "—— #", "--#")
		content = strings.ReplaceAll(content, " 完 ——", "完--")
		return "<CB><BOLD>【补打】</BOLD><BR></CB>" + content
	}
	if strings.EqualFold(outputType, "LABEL") && strings.Contains(strings.ToUpper(content), "<PAGE") {
		if textStart := strings.Index(strings.ToUpper(content), "<TEXT "); textStart >= 0 {
			if textEnd := strings.Index(content[textStart:], ">"); textEnd >= 0 {
				valueStart := textStart + textEnd + 1
				return content[:valueStart] + "补打 " + content[valueStart:]
			}
		}
		return content
	}
	return "【补打】\n" + content
}

func compactStoredPickupHeadline(content string) string {
	const marker = "）取餐码："
	end := strings.Index(content, marker)
	if end < 0 {
		return content
	}
	start := strings.LastIndex(content[:end], "（")
	if start < 0 {
		return content
	}
	title := fitPrintText(content[start+len("（"):end], 2)
	return content[:start] + "(" + title + ")取餐码:" + content[end+len(marker):]
}
