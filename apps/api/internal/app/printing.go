package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

type printerDTO struct {
	ID           int64  `json:"id"`
	StoreID      int64  `json:"store_id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	SN           string `json:"sn"`
	PaperWidth   int    `json:"paper_width"`
	PrintTrigger string `json:"print_trigger"`
	TemplateText string `json:"template_text"`
	Status       string `json:"status"`
}

type printerInput struct {
	StoreID      int64  `json:"store_id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	SN           string `json:"sn"`
	PaperWidth   int    `json:"paper_width"`
	PrintTrigger string `json:"print_trigger"`
	TemplateText string `json:"template_text"`
	Status       string `json:"status"`
}

func (s *Server) listPrinters(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,store_id,name,provider,model,sn,paper_width,print_trigger,template_text,status FROM printer_devices WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id DESC`, identity.TenantID, storeID)
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
	normalizePrinterInput(&input)
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO printer_devices(tenant_id,store_id,name,provider,model,sn,paper_width,print_trigger,template_text,status)
		SELECT ?,id,?,?,?,?,?,?,?,? FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, identity.TenantID, input.Name, input.Provider, input.Model, input.SN, input.PaperWidth, input.PrintTrigger, input.TemplateText, input.Status, storeID, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "printer.create", "printer", int64String(id), map[string]any{"sn": input.SN, "provider": input.Provider}, r)
	s.getPrinterByID(w, r, identity.TenantID, id)
}

func normalizePrinterInput(input *printerInput) {
	if input.Provider == "" {
		input.Provider = "mock"
	}
	if input.PaperWidth == 0 {
		input.PaperWidth = 58
	}
	if input.PrintTrigger == "" {
		input.PrintTrigger = "PAYMENT_SUCCESS"
	}
	if input.TemplateText == "" {
		input.TemplateText = "订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分"
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	input.Provider = strings.ToLower(input.Provider)
	input.PrintTrigger = strings.ToUpper(input.PrintTrigger)
	input.Status = strings.ToUpper(input.Status)
}

func (s *Server) getPrinter(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "printerID")
	if ok {
		s.getPrinterByID(w, r, currentIdentity(r.Context()).TenantID, id)
	}
}

func (s *Server) getPrinterByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var item printerDTO
	if err := scanPrinter(s.DB.QueryRowContext(r.Context(), `SELECT id,store_id,name,provider,model,sn,paper_width,print_trigger,template_text,status FROM printer_devices WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, id, tenantID), &item); err != nil {
		handleSQLError(w, err)
		return
	}
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
	normalizePrinterInput(&input)
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), `UPDATE printer_devices SET name=?,provider=?,model=?,sn=?,paper_width=?,print_trigger=?,template_text=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, input.Name, input.Provider, input.Model, input.SN, input.PaperWidth, input.PrintTrigger, input.TemplateText, input.Status, id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "printer not found")
		return
	}
	s.audit(r.Context(), identity, "printer.update", "printer", int64String(id), map[string]any{"sn": input.SN}, r)
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
	if err := scanPrinter(s.DB.QueryRowContext(r.Context(), `SELECT id,store_id,name,provider,model,sn,paper_width,print_trigger,template_text,status FROM printer_devices WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, id, identity.TenantID), &device); err != nil {
		handleSQLError(w, err)
		return
	}
	result, err := s.Printer.Print(r.Context(), provider.PrintRequest{DeviceSN: device.SN, DeviceType: device.Model, Content: "摊伴打印机测试\n设备：" + device.Name})
	if err != nil {
		writeError(w, http.StatusBadGateway, "PRINTER_PROVIDER_ERROR", err.Error())
		return
	}
	s.audit(r.Context(), identity, "printer.test", "printer", int64String(id), result, r)
	writeData(w, http.StatusOK, result)
}

func scanPrinter(row scanner, item *printerDTO) error {
	return row.Scan(&item.ID, &item.StoreID, &item.Name, &item.Provider, &item.Model, &item.SN, &item.PaperWidth, &item.PrintTrigger, &item.TemplateText, &item.Status)
}

func (s *Server) enqueueOrderPrints(ctx context.Context, tenantID, storeID, orderID int64, event string, reprint bool, actorID int64, extra string) error {
	return s.enqueueOrderPrintsWith(ctx, s.DB, tenantID, storeID, orderID, event, reprint, actorID, extra)
}

type sqlQueryExecer interface {
	sqlQueryer
	sqlExecer
}

func (s *Server) enqueueOrderPrintsWith(ctx context.Context, executor sqlQueryExecer, tenantID, storeID, orderID int64, event string, reprint bool, actorID int64, extra string) error {
	order, err := s.loadOrderWith(ctx, executor, tenantID, orderID, "")
	if err != nil {
		return err
	}
	rows, err := executor.QueryContext(ctx, `SELECT id,store_id,name,provider,model,sn,paper_width,print_trigger,template_text,status FROM printer_devices WHERE tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL`, tenantID, storeID)
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
		if event != "REFUND" && event != "REPRINT" && device.PrintTrigger != event {
			continue
		}
		content := renderTicket(device.TemplateText, order, extra, reprint)
		_, err := executor.ExecContext(ctx, `INSERT INTO print_jobs(tenant_id,store_id,order_id,printer_id,content_text,status,is_reprint,created_by) VALUES(?,?,?,?,?,'PENDING',?,?)`, tenantID, storeID, orderID, device.ID, content, reprint, actorID)
		if err != nil {
			return err
		}
	}
	return nil
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
	var sn, model, content string
	var reprint bool
	if err = s.DB.QueryRowContext(ctx, `SELECT d.sn,d.model,j.content_text,j.is_reprint FROM print_jobs j JOIN printer_devices d ON d.id=j.printer_id WHERE j.id=?`, id).Scan(&sn, &model, &content, &reprint); err != nil {
		_, _ = s.DB.ExecContext(ctx, "UPDATE print_jobs SET status='FAILED',error_message=? WHERE id=?", err.Error(), id)
		return err
	}
	printCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	result, printErr := s.Printer.Print(printCtx, provider.PrintRequest{JobID: id, DeviceSN: sn, DeviceType: model, Content: content, Reprint: reprint})
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
		itemLines = append(itemLines, fmt.Sprintf("%s %s x%d", item.ProductName, item.SKUName, item.Quantity))
	}
	replacer := strings.NewReplacer("{{order_no}}", order.OrderNo, "{{items}}", strings.Join(itemLines, "\n"), "{{total_cents}}", int64String(order.TotalCents), "{{remark}}", order.Remark)
	content := replacer.Replace(template)
	if reprint {
		content = "【补打】\n" + content
	}
	if extra != "" {
		content += "\n" + extra
	}
	return content
}

func (s *Server) reprintOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var storeID int64
	if err := s.DB.QueryRowContext(r.Context(), "SELECT store_id FROM orders WHERE id=? AND tenant_id=?", id, identity.TenantID).Scan(&storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err := s.enqueueOrderPrints(r.Context(), identity.TenantID, storeID, id, "REPRINT", true, identity.UserID, ""); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "order.reprint", "order", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"queued": true, "reprint": true})
}

func (s *Server) listPrintJobs(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM print_jobs WHERE tenant_id=?", identity.TenantID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT j.id,j.order_id,j.printer_id,j.provider_job_no,j.status,j.attempts,j.is_reprint,j.reprint_of,j.error_message,DATE_FORMAT(j.created_at,'%Y-%m-%dT%H:%i:%sZ') FROM print_jobs j WHERE j.tenant_id=? ORDER BY j.id DESC LIMIT ? OFFSET ?`, identity.TenantID, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, orderID, printerID int64
		var providerNo, status, errorMessage, created string
		var attempts int
		var reprint bool
		var reprintOf sql.NullInt64
		if err := rows.Scan(&id, &orderID, &printerID, &providerNo, &status, &attempts, &reprint, &reprintOf, &errorMessage, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "order_id": orderID, "printer_id": printerID, "provider_job_no": providerNo, "status": status, "attempts": attempts, "is_reprint": reprint, "reprint_of": reprintOf.Int64, "error_message": errorMessage, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) retryPrintJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "jobID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), `UPDATE print_jobs SET status='PENDING',attempts=0,is_reprint=1,
		content_text=IF(content_text LIKE '【补打】%',content_text,CONCAT('【补打】\n',content_text)),error_message=''
		WHERE id=? AND tenant_id=?`, id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "print job not found")
		return
	}
	s.audit(r.Context(), identity, "print_job.retry", "print_job", int64String(id), nil, r)
	writeData(w, http.StatusAccepted, map[string]any{"id": id, "status": "PENDING", "is_reprint": true})
}
