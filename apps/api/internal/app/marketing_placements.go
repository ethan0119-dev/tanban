package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type marketingPlacementInput struct {
	Name           string     `json:"name"`
	PlacementCode  string     `json:"placement_code"`
	ImageURL       string     `json:"image_url"`
	Title          string     `json:"title"`
	Subtitle       string     `json:"subtitle"`
	ActionType     string     `json:"action_type"`
	ActionTargetID *int64     `json:"action_target_id"`
	Frequency      string     `json:"frequency"`
	Priority       int        `json:"priority"`
	ChannelScope   string     `json:"channel_scope"`
	ActiveFrom     *time.Time `json:"active_from"`
	ActiveTo       *time.Time `json:"active_to"`
}

type marketingPlacementRow struct {
	ID             int64
	TenantID       int64
	StoreID        int64
	Name           string
	PlacementCode  string
	ImageURL       string
	Title          string
	Subtitle       string
	ActionType     string
	ActionTargetID sql.NullInt64
	Frequency      string
	Priority       int
	ChannelScope   string
	ActiveFrom     sql.NullTime
	ActiveTo       sql.NullTime
	Status         string
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Impressions    int64
	Clicks         int64
	Closes         int64
}

const marketingPlacementSelect = `SELECT p.id,p.tenant_id,p.store_id,p.name,p.placement_code,p.image_url,p.title,p.subtitle,p.action_type,p.action_target_id,p.frequency,p.priority,p.channel_scope,p.active_from,p.active_to,p.status,p.version,p.created_at,p.updated_at,
	(SELECT COUNT(*) FROM marketing_events e WHERE e.tenant_id=p.tenant_id AND e.placement_id=p.id AND e.event_type='IMPRESSION'),
	(SELECT COUNT(*) FROM marketing_events e WHERE e.tenant_id=p.tenant_id AND e.placement_id=p.id AND e.event_type='CLICK'),
	(SELECT COUNT(*) FROM marketing_events e WHERE e.tenant_id=p.tenant_id AND e.placement_id=p.id AND e.event_type='CLOSE')
	FROM marketing_placements p`

func scanMarketingPlacement(scanner interface{ Scan(...any) error }) (marketingPlacementRow, error) {
	var row marketingPlacementRow
	err := scanner.Scan(&row.ID, &row.TenantID, &row.StoreID, &row.Name, &row.PlacementCode, &row.ImageURL, &row.Title, &row.Subtitle, &row.ActionType, &row.ActionTargetID,
		&row.Frequency, &row.Priority, &row.ChannelScope, &row.ActiveFrom, &row.ActiveTo, &row.Status, &row.Version, &row.CreatedAt, &row.UpdatedAt,
		&row.Impressions, &row.Clicks, &row.Closes)
	return row, err
}

func marketingPlacementView(row marketingPlacementRow) map[string]any {
	var target any
	if row.ActionTargetID.Valid {
		target = row.ActionTargetID.Int64
	}
	return map[string]any{
		"id": row.ID, "name": row.Name, "placement_code": row.PlacementCode, "image_url": row.ImageURL, "title": row.Title, "subtitle": row.Subtitle,
		"action_type": row.ActionType, "action_target_id": target, "frequency": row.Frequency, "priority": row.Priority, "channel_scope": row.ChannelScope,
		"active_from": marketingTime(row.ActiveFrom), "active_to": marketingTime(row.ActiveTo), "status": row.Status, "version": row.Version,
		"impression_count": row.Impressions, "click_count": row.Clicks, "close_count": row.Closes,
		"created_at": formatBeijingDateTime(row.CreatedAt), "updated_at": formatBeijingDateTime(row.UpdatedAt),
	}
}

func publicMarketingPlacementView(row marketingPlacementRow) map[string]any {
	var target any
	if row.ActionTargetID.Valid {
		target = row.ActionTargetID.Int64
	}
	return map[string]any{
		"id": row.ID, "name": row.Name, "placement_code": row.PlacementCode, "image_url": row.ImageURL,
		"title": row.Title, "subtitle": row.Subtitle, "action_type": row.ActionType, "action_target_id": target,
		"frequency": row.Frequency, "priority": row.Priority, "channel_scope": row.ChannelScope,
	}
}

func normalizeMarketingPlacementInput(input *marketingPlacementInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.PlacementCode = strings.ToUpper(strings.TrimSpace(input.PlacementCode))
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	input.Title = strings.TrimSpace(input.Title)
	input.Subtitle = strings.TrimSpace(input.Subtitle)
	input.ActionType = strings.ToUpper(strings.TrimSpace(input.ActionType))
	input.Frequency = strings.ToUpper(strings.TrimSpace(input.Frequency))
	if input.PlacementCode == "" {
		input.PlacementCode = "HOME_POPUP"
	}
	if input.ActionType == "" {
		input.ActionType = "NONE"
	}
	if input.Frequency == "" {
		input.Frequency = "ONCE_PER_CAMPAIGN"
	}
	channel, err := marketingChannel(input.ChannelScope)
	if err != nil {
		return err
	}
	input.ChannelScope = channel
	if input.Name == "" || len([]rune(input.Name)) > 100 || len([]rune(input.Title)) > 80 || len([]rune(input.Subtitle)) > 200 {
		return errors.New("placement name is required and text must stay within its limit")
	}
	if !validStatus(input.PlacementCode, "HOME_POPUP", "MENU_POPUP", "CHECKOUT_POPUP", "ORDER_RESULT_POPUP", "PROFILE_POPUP") {
		return errors.New("placement_code is invalid")
	}
	if !validDecorationURL(input.ImageURL) {
		return errors.New("image_url must use HTTPS, except localhost development URLs")
	}
	if !validStatus(input.ActionType, "NONE", "OPEN_MENU", "OPEN_COUPONS", "CLAIM_COUPON", "OPEN_LOTTERY") {
		return errors.New("action_type is invalid")
	}
	requiresTarget := input.ActionType == "CLAIM_COUPON" || input.ActionType == "OPEN_LOTTERY"
	if requiresTarget && (input.ActionTargetID == nil || *input.ActionTargetID <= 0) {
		return errors.New("action_target_id is required for the selected action")
	}
	if !requiresTarget && input.ActionTargetID != nil {
		return errors.New("action_target_id is only allowed for CLAIM_COUPON or OPEN_LOTTERY")
	}
	if !validStatus(input.Frequency, "EVERY_VISIT", "DAILY", "ONCE_PER_CAMPAIGN") {
		return errors.New("frequency is invalid")
	}
	if input.Priority < -10000 || input.Priority > 10000 {
		return errors.New("priority must be between -10000 and 10000")
	}
	if !marketingWindowValid(input.ActiveFrom, input.ActiveTo) {
		return errors.New("active_from must be before active_to")
	}
	return nil
}

func (s *Server) loadMarketingPlacement(ctx context.Context, tenantID, storeID, id int64) (marketingPlacementRow, error) {
	return scanMarketingPlacement(s.DB.QueryRowContext(ctx, marketingPlacementSelect+" WHERE p.id=? AND p.tenant_id=? AND p.store_id=? AND p.deleted_at IS NULL", id, tenantID, storeID))
}

func (s *Server) listMarketingPlacements(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	page, size, offset := pagination(r)
	where := " WHERE p.tenant_id=? AND p.store_id=? AND p.deleted_at IS NULL"
	args := []any{actor.TenantID, storeID}
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" {
		if !validStatus(status, "DRAFT", "ACTIVE", "PAUSED", "ENDED") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status is invalid")
			return
		}
		where += " AND p.status=?"
		args = append(args, status)
	}
	var total int
	if err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM marketing_placements p"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), marketingPlacementSelect+where+" ORDER BY p.priority DESC,p.id DESC LIMIT ? OFFSET ?", append(args, size, offset)...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		row, scanErr := scanMarketingPlacement(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		items = append(items, marketingPlacementView(row))
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) getMarketingPlacement(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "placementID")
	if !ok {
		return
	}
	row, err := s.loadMarketingPlacement(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingPlacementView(row))
}

func (s *Server) createMarketingPlacement(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input marketingPlacementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err = normalizeMarketingPlacementInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
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
	if err = s.validateManagedMediaURL(r.Context(), tx, actor.TenantID, storeID, input.ImageURL); err != nil {
		writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
		return
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO marketing_placements(tenant_id,store_id,name,placement_code,image_url,title,subtitle,action_type,action_target_id,frequency,priority,channel_scope,active_from,active_to,status,created_by,updated_by)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,'DRAFT',?,?)`, actor.TenantID, storeID, input.Name, input.PlacementCode, input.ImageURL, input.Title, input.Subtitle, input.ActionType, nullableMarketingID(input.ActionTargetID), input.Frequency,
		input.Priority, input.ChannelScope, marketingTimeArg(input.ActiveFrom), marketingTimeArg(input.ActiveTo), actor.UserID, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "marketing.placement.create", "marketing_placement", int64String(id), input, r)
	row, err := s.loadMarketingPlacement(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, marketingPlacementView(row))
}

func nullableMarketingID(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func (s *Server) updateMarketingPlacement(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "placementID")
	if !ok {
		return
	}
	var input marketingPlacementInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err = normalizeMarketingPlacementInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
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
	var currentStatus string
	if err = tx.QueryRowContext(r.Context(), `SELECT status FROM marketing_placements WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL FOR UPDATE`, id, actor.TenantID, storeID).Scan(&currentStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if currentStatus == "ACTIVE" {
		writeError(w, http.StatusConflict, "PLACEMENT_ACTIVE", "pause the placement before editing it")
		return
	}
	if err = s.validateManagedMediaURL(r.Context(), tx, actor.TenantID, storeID, input.ImageURL); err != nil {
		writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE marketing_placements SET name=?,placement_code=?,image_url=?,title=?,subtitle=?,action_type=?,action_target_id=?,frequency=?,priority=?,channel_scope=?,active_from=?,active_to=?,version=version+1,updated_by=?
		WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status<>'ACTIVE'`, input.Name, input.PlacementCode, input.ImageURL, input.Title, input.Subtitle, input.ActionType, nullableMarketingID(input.ActionTargetID), input.Frequency,
		input.Priority, input.ChannelScope, marketingTimeArg(input.ActiveFrom), marketingTimeArg(input.ActiveTo), actor.UserID, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "PLACEMENT_CHANGED", "placement changed concurrently")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "marketing.placement.update", "marketing_placement", int64String(id), input, r)
	updated, err := s.loadMarketingPlacement(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingPlacementView(updated))
}

func (s *Server) deleteMarketingPlacement(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "placementID")
	if !ok {
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE marketing_placements SET status='ENDED',deleted_at=NOW(3),updated_by=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status IN ('DRAFT','PAUSED','ENDED')", actor.UserID, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "PLACEMENT_ACTIVE", "active placement cannot be deleted")
		return
	}
	s.audit(r.Context(), actor, "marketing.placement.delete", "marketing_placement", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) activateMarketingPlacement(w http.ResponseWriter, r *http.Request) {
	s.transitionMarketingPlacement(w, r, "ACTIVE")
}

func (s *Server) pauseMarketingPlacement(w http.ResponseWriter, r *http.Request) {
	s.transitionMarketingPlacement(w, r, "PAUSED")
}

func (s *Server) transitionMarketingPlacement(w http.ResponseWriter, r *http.Request, target string) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "placementID")
	if !ok {
		return
	}
	row, err := s.loadMarketingPlacement(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if target == "ACTIVE" {
		if row.ChannelScope == "DELIVERY" {
			writeError(w, http.StatusConflict, "DELIVERY_NOT_AVAILABLE", "delivery placements are reserved but cannot be activated in this release")
			return
		}
		if row.ActiveTo.Valid && !row.ActiveTo.Time.After(time.Now().UTC()) {
			writeError(w, http.StatusConflict, "PLACEMENT_EXPIRED", "expired placement cannot be activated")
			return
		}
		if !validStatus(row.Status, "DRAFT", "PAUSED") {
			writeError(w, http.StatusConflict, "INVALID_STATUS", "placement cannot be activated from its current status")
			return
		}
		if err = s.validateMarketingPlacementTarget(r.Context(), row); err != nil {
			writeError(w, http.StatusConflict, "INVALID_ACTION_TARGET", err.Error())
			return
		}
	} else if row.Status != "ACTIVE" {
		writeError(w, http.StatusConflict, "INVALID_STATUS", "only an active placement can be paused")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE marketing_placements SET status=?,version=version+1,updated_by=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status=?", target, actor.UserID, id, actor.TenantID, storeID, row.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "PLACEMENT_CHANGED", "placement changed concurrently")
		return
	}
	s.audit(r.Context(), actor, "marketing.placement."+strings.ToLower(target), "marketing_placement", int64String(id), map[string]any{"from": row.Status, "to": target}, r)
	updated, err := s.loadMarketingPlacement(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingPlacementView(updated))
}

func (s *Server) validateMarketingPlacementTarget(ctx context.Context, row marketingPlacementRow) error {
	if !row.ActionTargetID.Valid {
		return nil
	}
	var count int
	switch row.ActionType {
	case "CLAIM_COUPON":
		err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM coupon_campaigns WHERE id=? AND tenant_id=? AND store_id=? AND distribution_mode='PUBLIC_CLAIM' AND status='ACTIVE' AND deleted_at IS NULL", row.ActionTargetID.Int64, row.TenantID, row.StoreID).Scan(&count)
		if err != nil {
			return err
		}
	case "OPEN_LOTTERY":
		err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM lottery_campaigns WHERE id=? AND tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL", row.ActionTargetID.Int64, row.TenantID, row.StoreID).Scan(&count)
		if err != nil {
			return err
		}
	default:
		return nil
	}
	if count != 1 {
		return errors.New("action target must be active and belong to the same store")
	}
	return nil
}

func (s *Server) publicMarketingPopup(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	now := time.Now().UTC()
	placementCode := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("placementCode")))
	if placementCode == "" {
		placementCode = "HOME_POPUP"
	}
	if !validStatus(placementCode, "HOME_POPUP", "MENU_POPUP", "CHECKOUT_POPUP", "ORDER_RESULT_POPUP", "PROFILE_POPUP") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "placementCode is invalid")
		return
	}
	channelScope := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("channelScope")))
	if channelScope == "" {
		channelScope = "ALL"
	}
	if !validStatus(channelScope, "ALL", "DINE_IN", "TAKEOUT") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "channelScope is invalid")
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), marketingPlacementSelect+` WHERE p.tenant_id=? AND p.store_id=? AND p.deleted_at IS NULL AND p.status='ACTIVE' AND p.placement_code=? AND (p.channel_scope='ALL' OR p.channel_scope=?)
		AND (p.active_from IS NULL OR p.active_from<=?) AND (p.active_to IS NULL OR p.active_to>?) ORDER BY p.priority DESC,p.id DESC LIMIT 20`, store.TenantID, store.ID, placementCode, channelScope, now, now)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	placements := make([]marketingPlacementRow, 0)
	for rows.Next() {
		row, scanErr := scanMarketingPlacement(rows)
		if scanErr != nil {
			_ = rows.Close()
			handleSQLError(w, scanErr)
			return
		}
		placements = append(placements, row)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		handleSQLError(w, err)
		return
	}
	if err = rows.Close(); err != nil {
		handleSQLError(w, err)
		return
	}
	for _, row := range placements {
		if strings.TrimSpace(row.ImageURL) == "" && strings.TrimSpace(row.Title) == "" && strings.TrimSpace(row.Subtitle) == "" {
			continue
		}
		available, availabilityErr := s.marketingPlacementTargetAvailable(r.Context(), row, now)
		if availabilityErr != nil {
			handleSQLError(w, availabilityErr)
			return
		}
		if available {
			writeData(w, http.StatusOK, publicMarketingPlacementView(row))
			return
		}
	}
	writeData(w, http.StatusOK, nil)
}

func (s *Server) marketingPlacementTargetAvailable(ctx context.Context, row marketingPlacementRow, now time.Time) (bool, error) {
	if !row.ActionTargetID.Valid {
		return true, nil
	}
	var count int
	var err error
	switch row.ActionType {
	case "CLAIM_COUPON":
		err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM coupon_campaigns WHERE id=? AND tenant_id=? AND store_id=? AND distribution_mode='PUBLIC_CLAIM' AND status='ACTIVE' AND deleted_at IS NULL
			AND issued_count<total_stock AND (claim_start_at IS NULL OR claim_start_at<=?) AND (claim_end_at IS NULL OR claim_end_at>?)
			AND (validity_mode<>'FIXED_RANGE' OR valid_to>?)`, row.ActionTargetID.Int64, row.TenantID, row.StoreID, now, now, now).Scan(&count)
	case "OPEN_LOTTERY":
		err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM lottery_campaigns WHERE id=? AND tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL
			AND channel_scope<>'DELIVERY' AND active_from<=? AND active_to>?`, row.ActionTargetID.Int64, row.TenantID, row.StoreID, now, now).Scan(&count)
	default:
		return true, nil
	}
	return count == 1, err
}

type publicMarketingEventInput struct {
	PlacementID   int64  `json:"placement_id"`
	EventType     string `json:"event_type"`
	SubjectKey    string `json:"subject_key,omitempty"`
	EventID       string `json:"event_id,omitempty"`
	IdempotencyID string `json:"idempotency_key,omitempty"`
}

func (s *Server) publicRecordMarketingEvent(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input publicMarketingEventInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.EventType = strings.ToUpper(strings.TrimSpace(input.EventType))
	if input.PlacementID <= 0 || !validStatus(input.EventType, "IMPRESSION", "CLICK", "CLOSE") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "placement_id and a valid event_type are required")
		return
	}
	bodyKey := input.IdempotencyID
	if bodyKey == "" {
		bodyKey = input.EventID
	}
	idempotencyKey, err := marketingIdempotencyKey(r, bodyKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", err.Error())
		return
	}
	subjectHash := ""
	if strings.TrimSpace(input.SubjectKey) != "" {
		subjectHash, err = marketingSubjectHash(input.SubjectKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
	}
	fingerprint := requestFingerprint(map[string]any{"placement_id": input.PlacementID, "event_type": input.EventType, "subject_key_hash": subjectHash})
	var existingID int64
	var existingFingerprint string
	err = s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM marketing_events WHERE tenant_id=? AND idempotency_key=?", store.TenantID, idempotencyKey).Scan(&existingID, &existingFingerprint)
	if err == nil {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used for another marketing event")
			return
		}
		writeData(w, http.StatusOK, map[string]any{"id": existingID, "event_type": input.EventType})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	var placementExists int
	now := time.Now().UTC()
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM marketing_placements WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status='ACTIVE'
		AND (active_from IS NULL OR active_from<=?) AND (active_to IS NULL OR active_to>?)`, input.PlacementID, store.TenantID, store.ID, now, now).Scan(&placementExists); err != nil {
		handleSQLError(w, err)
		return
	}
	if placementExists != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "marketing placement not found")
		return
	}
	if !s.allowPublicMarketingEvent(r.Context(), r, store.Code, input.PlacementID, input.EventType) {
		writeError(w, http.StatusTooManyRequests, "MARKETING_EVENT_RATE_LIMITED", "too many marketing events; retry later")
		return
	}
	if !s.allowPublicMarketingEventSubject(r.Context(), r, store.Code, input.PlacementID, input.EventType, subjectHash) {
		writeData(w, http.StatusAccepted, map[string]any{"event_type": input.EventType, "recorded": false, "reason": "deduplicated"})
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO marketing_events(tenant_id,store_id,placement_id,event_type,subject_key_hash,idempotency_key,request_fingerprint,occurred_at)
		VALUES(?,?,?,?,?,?,?,?)`, store.TenantID, store.ID, input.PlacementID, input.EventType, subjectHash, idempotencyKey, fingerprint, now)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			loadErr := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM marketing_events WHERE tenant_id=? AND idempotency_key=?", store.TenantID, idempotencyKey).Scan(&existingID, &existingFingerprint)
			if loadErr == nil && existingFingerprint == fingerprint {
				writeData(w, http.StatusOK, map[string]any{"id": existingID, "event_type": input.EventType})
				return
			}
			if loadErr == nil {
				writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used for another marketing event")
				return
			}
			if !errors.Is(loadErr, sql.ErrNoRows) {
				handleSQLError(w, loadErr)
				return
			}
		}
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	writeData(w, http.StatusCreated, map[string]any{"id": id, "event_type": input.EventType})
}
