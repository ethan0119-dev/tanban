package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type fullReductionInput struct {
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	ThresholdCents int64            `json:"threshold_cents"`
	DiscountCents  int64            `json:"discount_cents"`
	OrderTypes     []string         `json:"order_types"`
	ActiveFrom     *requestDateTime `json:"active_from"`
	ActiveTo       *requestDateTime `json:"active_to"`
}

type fullReductionRow struct {
	ID, TenantID, StoreID, ThresholdCents, DiscountCents, Version int64
	Name, Description, OrderTypesJSON, Status                     string
	ActiveFrom, ActiveTo                                          sql.NullTime
	CreatedAt, UpdatedAt                                          time.Time
}

const fullReductionSelect = `SELECT id,tenant_id,store_id,name,description,threshold_cents,discount_cents,order_types_json,
	active_from,active_to,status,version,created_at,updated_at FROM store_full_reduction_campaigns`

func scanFullReduction(scanner interface{ Scan(...any) error }) (fullReductionRow, error) {
	var row fullReductionRow
	err := scanner.Scan(&row.ID, &row.TenantID, &row.StoreID, &row.Name, &row.Description, &row.ThresholdCents,
		&row.DiscountCents, &row.OrderTypesJSON, &row.ActiveFrom, &row.ActiveTo, &row.Status, &row.Version, &row.CreatedAt, &row.UpdatedAt)
	return row, err
}

func fullReductionView(row fullReductionRow) map[string]any {
	return map[string]any{
		"id": row.ID, "name": row.Name, "description": row.Description, "threshold_cents": row.ThresholdCents,
		"discount_cents": row.DiscountCents, "order_types": decodeMarketingOrderTypes(row.OrderTypesJSON),
		"active_from": marketingTime(row.ActiveFrom), "active_to": marketingTime(row.ActiveTo), "status": row.Status,
		"version": row.Version, "created_at": formatBeijingDateTime(row.CreatedAt), "updated_at": formatBeijingDateTime(row.UpdatedAt),
	}
}

func normalizeFullReductionInput(input *fullReductionInput) (string, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || len([]rune(input.Name)) > 100 || len([]rune(input.Description)) > 500 {
		return "", errors.New("name is required and activity text is too long")
	}
	if input.ThresholdCents <= 0 || input.ThresholdCents > maxBusinessAmountCents ||
		input.DiscountCents <= 0 || input.DiscountCents > input.ThresholdCents {
		return "", errors.New("threshold and discount are outside the supported range")
	}
	if !requestDateTimeWindowValid(input.ActiveFrom, input.ActiveTo) {
		return "", errors.New("active_from must be before active_to")
	}
	_, orderTypesJSON, err := normalizeMarketingOrderTypes(input.OrderTypes)
	return orderTypesJSON, err
}

func (s *Server) listFullReductions(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), fullReductionSelect+` WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id DESC`, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		row, scanErr := scanFullReduction(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		items = append(items, fullReductionView(row))
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) createFullReduction(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input fullReductionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	orderTypesJSON, err := normalizeFullReductionInput(&input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO store_full_reduction_campaigns
		(tenant_id,store_id,name,description,threshold_cents,discount_cents,order_types_json,active_from,active_to,status,created_by,updated_by)
		VALUES(?,?,?,?,?,?,?,?,?,'DRAFT',?,?)`, actor.TenantID, storeID, input.Name, input.Description, input.ThresholdCents,
		input.DiscountCents, orderTypesJSON, requestDateTimeArg(input.ActiveFrom), requestDateTimeArg(input.ActiveTo), actor.UserID, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	row, err := scanFullReduction(s.DB.QueryRowContext(r.Context(), fullReductionSelect+` WHERE id=? AND tenant_id=?`, id, actor.TenantID))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "marketing.full_reduction.create", "store_full_reduction", int64String(id), input, r)
	writeData(w, http.StatusCreated, fullReductionView(row))
}

func (s *Server) updateFullReduction(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	var input fullReductionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	orderTypesJSON, err := normalizeFullReductionInput(&input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE store_full_reduction_campaigns SET name=?,description=?,threshold_cents=?,discount_cents=?,
		order_types_json=?,active_from=?,active_to=?,version=version+1,updated_by=? WHERE id=? AND tenant_id=? AND store_id=? AND status IN ('DRAFT','PAUSED') AND deleted_at IS NULL`,
		input.Name, input.Description, input.ThresholdCents, input.DiscountCents, orderTypesJSON, requestDateTimeArg(input.ActiveFrom),
		requestDateTimeArg(input.ActiveTo), actor.UserID, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "CAMPAIGN_NOT_EDITABLE", "active or missing activity cannot be edited")
		return
	}
	row, err := scanFullReduction(s.DB.QueryRowContext(r.Context(), fullReductionSelect+` WHERE id=? AND tenant_id=?`, id, actor.TenantID))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, fullReductionView(row))
}

func (s *Server) transitionFullReduction(w http.ResponseWriter, r *http.Request, status string) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE store_full_reduction_campaigns SET status=?,version=version+1,updated_by=?
		WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, status, actor.UserID, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "activity not found")
		return
	}
	row, err := scanFullReduction(s.DB.QueryRowContext(r.Context(), fullReductionSelect+` WHERE id=? AND tenant_id=?`, id, actor.TenantID))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, fullReductionView(row))
}

func (s *Server) activateFullReduction(w http.ResponseWriter, r *http.Request) {
	s.transitionFullReduction(w, r, "ACTIVE")
}
func (s *Server) pauseFullReduction(w http.ResponseWriter, r *http.Request) {
	s.transitionFullReduction(w, r, "PAUSED")
}

func (s *Server) publicListFullReductions(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	channel, err := publicMarketingChannel(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	now := time.Now().UTC()
	rows, err := s.DB.QueryContext(r.Context(), fullReductionSelect+` WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL AND status='ACTIVE'
		AND (active_from IS NULL OR active_from<=?) AND (active_to IS NULL OR active_to>?) ORDER BY discount_cents DESC,threshold_cents ASC,id ASC`,
		store.TenantID, store.ID, now, now)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		row, scanErr := scanFullReduction(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		if marketingOrderTypesContain(decodeMarketingOrderTypes(row.OrderTypesJSON), channel) {
			items = append(items, fullReductionView(row))
		}
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

func bestStoreFullReduction(rows []fullReductionRow, subtotal int64, orderType string) fullReductionRow {
	for _, row := range rows {
		if subtotal >= row.ThresholdCents && marketingOrderTypesContain(decodeMarketingOrderTypes(row.OrderTypesJSON), orderType) {
			return row
		}
	}
	return fullReductionRow{}
}
