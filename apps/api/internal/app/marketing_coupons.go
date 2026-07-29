package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type marketingCouponInput struct {
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	CouponType       string           `json:"coupon_type"`
	ThresholdCents   int64            `json:"threshold_cents"`
	DiscountCents    int64            `json:"discount_cents"`
	DistributionMode string           `json:"distribution_mode"`
	TotalStock       int64            `json:"total_stock"`
	PerSubjectLimit  int              `json:"per_subject_limit"`
	ClaimStartAt     *requestDateTime `json:"claim_start_at"`
	ClaimEndAt       *requestDateTime `json:"claim_end_at"`
	ValidityMode     string           `json:"validity_mode"`
	ValidFrom        *requestDateTime `json:"valid_from"`
	ValidTo          *requestDateTime `json:"valid_to"`
	ValidDays        int              `json:"valid_days"`
	OrderTypes       []string         `json:"order_types"`
}

type marketingCouponRow struct {
	ID               int64
	TenantID         int64
	StoreID          int64
	Name             string
	Description      string
	CouponType       string
	ThresholdCents   int64
	DiscountCents    int64
	DistributionMode string
	TotalStock       int64
	IssuedCount      int64
	PerSubjectLimit  int
	ClaimStartAt     sql.NullTime
	ClaimEndAt       sql.NullTime
	ValidityMode     string
	ValidFrom        sql.NullTime
	ValidTo          sql.NullTime
	ValidDays        int
	OrderTypesJSON   string
	Status           string
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ClaimCount       int64
	UseCount         int64
}

const marketingCouponSelect = `SELECT c.id,c.tenant_id,c.store_id,c.name,c.description,c.coupon_type,c.threshold_cents,c.discount_cents,
	c.distribution_mode,c.total_stock,c.issued_count,c.per_subject_limit,c.claim_start_at,c.claim_end_at,c.validity_mode,c.valid_from,c.valid_to,
	c.valid_days,c.order_types_json,c.status,c.version,c.created_at,c.updated_at,
	(SELECT COUNT(*) FROM customer_coupons cc WHERE cc.tenant_id=c.tenant_id AND cc.campaign_id=c.id),
	(SELECT COUNT(*) FROM customer_coupons cc WHERE cc.tenant_id=c.tenant_id AND cc.campaign_id=c.id AND cc.status='USED')
	FROM coupon_campaigns c`

func scanMarketingCoupon(scanner interface{ Scan(...any) error }) (marketingCouponRow, error) {
	var row marketingCouponRow
	err := scanner.Scan(&row.ID, &row.TenantID, &row.StoreID, &row.Name, &row.Description, &row.CouponType, &row.ThresholdCents, &row.DiscountCents,
		&row.DistributionMode, &row.TotalStock, &row.IssuedCount, &row.PerSubjectLimit, &row.ClaimStartAt, &row.ClaimEndAt, &row.ValidityMode,
		&row.ValidFrom, &row.ValidTo, &row.ValidDays, &row.OrderTypesJSON, &row.Status, &row.Version, &row.CreatedAt, &row.UpdatedAt,
		&row.ClaimCount, &row.UseCount)
	return row, err
}

func marketingCouponView(row marketingCouponRow) map[string]any {
	return map[string]any{
		"id": row.ID, "name": row.Name, "description": row.Description, "coupon_type": row.CouponType,
		"threshold_cents": row.ThresholdCents, "discount_cents": row.DiscountCents, "distribution_mode": row.DistributionMode,
		"total_stock": row.TotalStock, "issued_count": row.IssuedCount, "per_subject_limit": row.PerSubjectLimit,
		"claim_start_at": marketingTime(row.ClaimStartAt), "claim_end_at": marketingTime(row.ClaimEndAt),
		"validity_mode": row.ValidityMode, "valid_from": marketingTime(row.ValidFrom), "valid_to": marketingTime(row.ValidTo),
		"valid_days": row.ValidDays, "order_types": decodeMarketingOrderTypes(row.OrderTypesJSON), "status": row.Status,
		"version": row.Version, "claim_count": row.ClaimCount, "use_count": row.UseCount,
		"created_at": formatBeijingDateTime(row.CreatedAt), "updated_at": formatBeijingDateTime(row.UpdatedAt),
	}
}

func publicMarketingCouponView(row marketingCouponRow) map[string]any {
	return map[string]any{
		"id": row.ID, "name": row.Name, "description": row.Description, "coupon_type": row.CouponType,
		"threshold_cents": row.ThresholdCents, "discount_cents": row.DiscountCents, "distribution_mode": row.DistributionMode,
		"total_stock": row.TotalStock, "issued_count": row.IssuedCount, "per_subject_limit": row.PerSubjectLimit,
		"claim_start_at": marketingTime(row.ClaimStartAt), "claim_end_at": marketingTime(row.ClaimEndAt),
		"validity_mode": row.ValidityMode, "valid_from": marketingTime(row.ValidFrom), "valid_to": marketingTime(row.ValidTo),
		"valid_days": row.ValidDays, "order_types": decodeMarketingOrderTypes(row.OrderTypesJSON), "status": row.Status,
	}
}

func normalizeMarketingCouponInput(input *marketingCouponInput) (string, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.CouponType = strings.ToUpper(strings.TrimSpace(input.CouponType))
	input.DistributionMode = strings.ToUpper(strings.TrimSpace(input.DistributionMode))
	input.ValidityMode = strings.ToUpper(strings.TrimSpace(input.ValidityMode))
	if input.DistributionMode == "" {
		input.DistributionMode = "PUBLIC_CLAIM"
	}
	if input.ValidityMode == "" {
		input.ValidityMode = "RELATIVE_DAYS"
	}
	if input.PerSubjectLimit == 0 {
		input.PerSubjectLimit = 1
	}
	if input.ValidDays == 0 && input.ValidityMode == "RELATIVE_DAYS" {
		input.ValidDays = 30
	}
	if input.Name == "" || len([]rune(input.Name)) > 100 || len([]rune(input.Description)) > 500 {
		return "", errors.New("name is required and coupon text is too long")
	}
	if !validStatus(input.CouponType, "CASH", "FULL_REDUCTION") {
		return "", errors.New("coupon_type must be CASH or FULL_REDUCTION")
	}
	if input.DiscountCents <= 0 || input.DiscountCents > maxBusinessAmountCents {
		return "", errors.New("discount_cents is outside the supported range")
	}
	if input.CouponType == "CASH" {
		input.ThresholdCents = 0
	} else if input.ThresholdCents <= 0 || input.ThresholdCents > maxBusinessAmountCents || input.DiscountCents > input.ThresholdCents {
		return "", errors.New("FULL_REDUCTION requires threshold_cents not below discount_cents")
	}
	if !validStatus(input.DistributionMode, "PUBLIC_CLAIM", "MANUAL_ONLY", "LOTTERY_ONLY") {
		return "", errors.New("distribution_mode is invalid")
	}
	if input.TotalStock <= 0 || input.TotalStock > 1_000_000_000 || input.PerSubjectLimit <= 0 || int64(input.PerSubjectLimit) > input.TotalStock {
		return "", errors.New("total_stock and per_subject_limit are outside the supported range")
	}
	if !requestDateTimeWindowValid(input.ClaimStartAt, input.ClaimEndAt) {
		return "", errors.New("claim_start_at must be before claim_end_at")
	}
	switch input.ValidityMode {
	case "FIXED_RANGE":
		if input.ValidFrom == nil || input.ValidTo == nil || !requestDateTimeWindowValid(input.ValidFrom, input.ValidTo) {
			return "", errors.New("FIXED_RANGE requires valid_from before valid_to")
		}
		input.ValidDays = 0
	case "RELATIVE_DAYS":
		if input.ValidDays <= 0 || input.ValidDays > 3650 {
			return "", errors.New("RELATIVE_DAYS requires valid_days between 1 and 3650")
		}
		input.ValidFrom, input.ValidTo = nil, nil
	default:
		return "", errors.New("validity_mode must be FIXED_RANGE or RELATIVE_DAYS")
	}
	_, orderTypesJSON, err := normalizeMarketingOrderTypes(input.OrderTypes)
	return orderTypesJSON, err
}

func (s *Server) listMarketingCoupons(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	page, size, offset := pagination(r)
	where := " WHERE c.tenant_id=? AND c.store_id=? AND c.deleted_at IS NULL"
	args := []any{actor.TenantID, storeID}
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" {
		if !validStatus(status, "DRAFT", "ACTIVE", "PAUSED", "ENDED") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status is invalid")
			return
		}
		where += " AND c.status=?"
		args = append(args, status)
	}
	var total int
	if err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM coupon_campaigns c"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, offset)
	rows, err := s.DB.QueryContext(r.Context(), marketingCouponSelect+where+" ORDER BY c.id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		row, scanErr := scanMarketingCoupon(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		items = append(items, marketingCouponView(row))
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) loadMarketingCoupon(ctx context.Context, tenantID, storeID, id int64) (marketingCouponRow, error) {
	return scanMarketingCoupon(s.DB.QueryRowContext(ctx, marketingCouponSelect+" WHERE c.id=? AND c.tenant_id=? AND c.store_id=? AND c.deleted_at IS NULL", id, tenantID, storeID))
}

func (s *Server) getMarketingCoupon(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "couponID")
	if !ok {
		return
	}
	row, err := s.loadMarketingCoupon(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingCouponView(row))
}

func (s *Server) createMarketingCoupon(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input marketingCouponInput
	if !decodeJSON(w, r, &input) {
		return
	}
	orderTypesJSON, err := normalizeMarketingCouponInput(&input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO coupon_campaigns(tenant_id,store_id,name,description,coupon_type,threshold_cents,discount_cents,distribution_mode,total_stock,per_subject_limit,claim_start_at,claim_end_at,validity_mode,valid_from,valid_to,valid_days,order_types_json,status,created_by,updated_by)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'DRAFT',?,?)`, actor.TenantID, storeID, input.Name, input.Description, input.CouponType, input.ThresholdCents, input.DiscountCents,
		input.DistributionMode, input.TotalStock, input.PerSubjectLimit, requestDateTimeArg(input.ClaimStartAt), requestDateTimeArg(input.ClaimEndAt), input.ValidityMode,
		requestDateTimeArg(input.ValidFrom), requestDateTimeArg(input.ValidTo), input.ValidDays, orderTypesJSON, actor.UserID, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), actor, "marketing.coupon.create", "coupon_campaign", int64String(id), input, r)
	row, err := s.loadMarketingCoupon(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, marketingCouponView(row))
}

func (s *Server) updateMarketingCoupon(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "couponID")
	if !ok {
		return
	}
	current, err := s.loadMarketingCoupon(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if current.Status == "ACTIVE" {
		writeError(w, http.StatusConflict, "CAMPAIGN_ACTIVE", "pause the coupon campaign before editing it")
		return
	}
	var input marketingCouponInput
	if !decodeJSON(w, r, &input) {
		return
	}
	orderTypesJSON, err := normalizeMarketingCouponInput(&input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if input.TotalStock < current.IssuedCount {
		writeError(w, http.StatusConflict, "STOCK_BELOW_ISSUED", "total_stock cannot be below issued_count")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE coupon_campaigns SET name=?,description=?,coupon_type=?,threshold_cents=?,discount_cents=?,distribution_mode=?,total_stock=?,per_subject_limit=?,claim_start_at=?,claim_end_at=?,validity_mode=?,valid_from=?,valid_to=?,valid_days=?,order_types_json=?,version=version+1,updated_by=?
		WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status<>'ACTIVE'`, input.Name, input.Description, input.CouponType, input.ThresholdCents, input.DiscountCents, input.DistributionMode,
		input.TotalStock, input.PerSubjectLimit, requestDateTimeArg(input.ClaimStartAt), requestDateTimeArg(input.ClaimEndAt), input.ValidityMode, requestDateTimeArg(input.ValidFrom), requestDateTimeArg(input.ValidTo), input.ValidDays,
		orderTypesJSON, actor.UserID, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "CAMPAIGN_CHANGED", "coupon campaign changed concurrently")
		return
	}
	s.audit(r.Context(), actor, "marketing.coupon.update", "coupon_campaign", int64String(id), input, r)
	row, err := s.loadMarketingCoupon(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingCouponView(row))
}

func (s *Server) deleteMarketingCoupon(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "couponID")
	if !ok {
		return
	}
	var references int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT
		(SELECT COUNT(*) FROM customer_coupons WHERE tenant_id=? AND campaign_id=?) +
		(SELECT COUNT(*) FROM lottery_prizes WHERE tenant_id=? AND coupon_campaign_id=?) +
		(SELECT COUNT(*) FROM marketing_placements WHERE tenant_id=? AND action_type='CLAIM_COUPON' AND action_target_id=? AND deleted_at IS NULL)`, actor.TenantID, id, actor.TenantID, id, actor.TenantID, id).Scan(&references); err != nil {
		handleSQLError(w, err)
		return
	}
	if references > 0 {
		writeError(w, http.StatusConflict, "CAMPAIGN_IN_USE", "campaign with issued records, lottery prizes or popup references cannot be deleted")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE coupon_campaigns SET status='ENDED',deleted_at=NOW(3),updated_by=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status IN ('DRAFT','PAUSED','ENDED')", actor.UserID, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "CAMPAIGN_ACTIVE", "active coupon campaign cannot be deleted")
		return
	}
	s.audit(r.Context(), actor, "marketing.coupon.delete", "coupon_campaign", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) activateMarketingCoupon(w http.ResponseWriter, r *http.Request) {
	s.transitionMarketingCoupon(w, r, "ACTIVE")
}

func (s *Server) pauseMarketingCoupon(w http.ResponseWriter, r *http.Request) {
	s.transitionMarketingCoupon(w, r, "PAUSED")
}

func (s *Server) transitionMarketingCoupon(w http.ResponseWriter, r *http.Request, target string) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "couponID")
	if !ok {
		return
	}
	row, err := s.loadMarketingCoupon(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if target == "ACTIVE" {
		if marketingHasDelivery(decodeMarketingOrderTypes(row.OrderTypesJSON)) {
			writeError(w, http.StatusConflict, "DELIVERY_NOT_AVAILABLE", "delivery coupon campaigns are reserved but cannot be activated in this release")
			return
		}
		now := time.Now().UTC()
		if row.ClaimEndAt.Valid && !row.ClaimEndAt.Time.After(now) || row.ValidTo.Valid && !row.ValidTo.Time.After(now) {
			writeError(w, http.StatusConflict, "CAMPAIGN_EXPIRED", "expired coupon campaign cannot be activated")
			return
		}
		if !validStatus(row.Status, "DRAFT", "PAUSED") {
			writeError(w, http.StatusConflict, "INVALID_STATUS", "coupon campaign cannot be activated from its current status")
			return
		}
	} else if row.Status != "ACTIVE" {
		writeError(w, http.StatusConflict, "INVALID_STATUS", "only an active coupon campaign can be paused")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE coupon_campaigns SET status=?,version=version+1,updated_by=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status=?", target, actor.UserID, id, actor.TenantID, storeID, row.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "CAMPAIGN_CHANGED", "coupon campaign changed concurrently")
		return
	}
	s.audit(r.Context(), actor, "marketing.coupon."+strings.ToLower(target), "coupon_campaign", int64String(id), map[string]any{"from": row.Status, "to": target}, r)
	updated, err := s.loadMarketingCoupon(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingCouponView(updated))
}

func (s *Server) listMarketingCouponRecords(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	page, size, offset := pagination(r)
	where := " WHERE cc.tenant_id=? AND cc.store_id=?"
	args := []any{actor.TenantID, storeID}
	if raw := strings.TrimSpace(r.URL.Query().Get("campaign_id")); raw != "" {
		id, parseErr := strconvParseInt(raw)
		if parseErr != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "campaign_id must be a positive integer")
			return
		}
		where += " AND cc.campaign_id=?"
		args = append(args, id)
	}
	var total int
	if err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM customer_coupons cc"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, offset)
	rows, err := s.DB.QueryContext(r.Context(), `SELECT cc.id,cc.coupon_no,cc.campaign_id,c.name,cc.source,cc.status,cc.subject_key_hash,cc.valid_from,cc.valid_to,cc.claimed_at
		FROM customer_coupons cc JOIN coupon_campaigns c ON c.id=cc.campaign_id AND c.tenant_id=cc.tenant_id`+where+` ORDER BY cc.id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, campaignID int64
		var no, campaignName, source, status, subject string
		var validFrom, validTo, claimed time.Time
		if err = rows.Scan(&id, &no, &campaignID, &campaignName, &source, &status, &subject, &validFrom, &validTo, &claimed); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "coupon_no": no, "campaign_id": campaignID, "campaign_name": campaignName, "source": source, "status": status, "subject_key_mask": marketingSubjectMask(subject), "valid_from": formatBeijingDateTime(validFrom), "valid_to": formatBeijingDateTime(validTo), "claimed_at": formatBeijingDateTime(claimed)})
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) publicListMarketingCoupons(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.DB.QueryContext(r.Context(), marketingCouponSelect+` WHERE c.tenant_id=? AND c.store_id=? AND c.deleted_at IS NULL AND c.status='ACTIVE' AND c.distribution_mode='PUBLIC_CLAIM'
		AND (c.claim_start_at IS NULL OR c.claim_start_at<=?) AND (c.claim_end_at IS NULL OR c.claim_end_at>?)
		AND (c.validity_mode<>'FIXED_RANGE' OR c.valid_to>?) ORDER BY c.id DESC`, store.TenantID, store.ID, now, now, now)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		row, scanErr := scanMarketingCoupon(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		if !marketingOrderTypesContain(decodeMarketingOrderTypes(row.OrderTypesJSON), channel) {
			continue
		}
		view := publicMarketingCouponView(row)
		view["remaining_stock"] = row.TotalStock - row.IssuedCount
		items = append(items, view)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items, "identity_verified": false, "asset_issuance_mode": "PROVISIONAL_ONLY", "warning": provisionalMarketingAssetWarning})
}

type publicMarketingSubjectInput struct {
	SubjectKey     string `json:"subject_key"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func (s *Server) publicClaimMarketingCoupon(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	campaignID, ok := pathID(w, r, "couponID")
	if !ok {
		return
	}
	var input publicMarketingSubjectInput
	if !decodeJSON(w, r, &input) {
		return
	}
	subjectHash, err := marketingSubjectHash(input.SubjectKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	idempotencyKey, err := marketingIdempotencyKey(r, input.IdempotencyKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", err.Error())
		return
	}
	fingerprint := requestFingerprint(map[string]any{"campaign_id": campaignID, "subject_key_hash": subjectHash})
	if existing, existingFingerprint, found, loadErr := s.loadProvisionalCouponByKey(r.Context(), store.TenantID, idempotencyKey); loadErr != nil {
		handleSQLError(w, loadErr)
		return
	} else if found {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used for a different coupon claim")
			return
		}
		writeData(w, http.StatusOK, existing)
		return
	}
	if !s.allowPublicMarketingMutation(r.Context(), r, store.Code, "coupon-claim") {
		writeError(w, http.StatusTooManyRequests, "MARKETING_RATE_LIMITED", "too many coupon claim attempts; try again later")
		return
	}

	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var campaign marketingCouponRow
	err = tx.QueryRowContext(r.Context(), `SELECT id,tenant_id,store_id,name,description,coupon_type,threshold_cents,discount_cents,distribution_mode,total_stock,issued_count,per_subject_limit,
		claim_start_at,claim_end_at,validity_mode,valid_from,valid_to,valid_days,order_types_json,status,version,created_at,updated_at,0,0
		FROM coupon_campaigns WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL FOR UPDATE`, campaignID, store.TenantID, store.ID).
		Scan(&campaign.ID, &campaign.TenantID, &campaign.StoreID, &campaign.Name, &campaign.Description, &campaign.CouponType, &campaign.ThresholdCents, &campaign.DiscountCents,
			&campaign.DistributionMode, &campaign.TotalStock, &campaign.IssuedCount, &campaign.PerSubjectLimit, &campaign.ClaimStartAt, &campaign.ClaimEndAt, &campaign.ValidityMode,
			&campaign.ValidFrom, &campaign.ValidTo, &campaign.ValidDays, &campaign.OrderTypesJSON, &campaign.Status, &campaign.Version, &campaign.CreatedAt, &campaign.UpdatedAt, &campaign.ClaimCount, &campaign.UseCount)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if existing, existingFingerprint, found, loadErr := loadProvisionalCouponByKeyWithQueryer(r.Context(), tx, store.TenantID, idempotencyKey); loadErr != nil {
		handleSQLError(w, loadErr)
		return
	} else if found {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used for a different coupon claim")
			return
		}
		writeData(w, http.StatusOK, existing)
		return
	}
	now := time.Now().UTC()
	if campaign.Status != "ACTIVE" || campaign.DistributionMode != "PUBLIC_CLAIM" ||
		(campaign.ClaimStartAt.Valid && now.Before(campaign.ClaimStartAt.Time)) || (campaign.ClaimEndAt.Valid && !now.Before(campaign.ClaimEndAt.Time)) {
		writeError(w, http.StatusConflict, "CAMPAIGN_NOT_CLAIMABLE", "coupon campaign is not currently claimable")
		return
	}
	if campaign.IssuedCount >= campaign.TotalStock {
		writeError(w, http.StatusConflict, "COUPON_SOLD_OUT", "coupon campaign is sold out")
		return
	}
	var subjectClaims int
	if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM customer_coupons WHERE tenant_id=? AND campaign_id=? AND subject_key_hash=? AND status<>'VOID'", store.TenantID, campaignID, subjectHash).Scan(&subjectClaims); err != nil {
		handleSQLError(w, err)
		return
	}
	if subjectClaims >= campaign.PerSubjectLimit {
		writeError(w, http.StatusConflict, "CLAIM_LIMIT_REACHED", "anonymous subject claim limit was reached")
		return
	}
	validFrom, validTo := now, now.Add(time.Duration(campaign.ValidDays)*24*time.Hour)
	if campaign.ValidityMode == "FIXED_RANGE" {
		validFrom, validTo = campaign.ValidFrom.Time, campaign.ValidTo.Time
	}
	if !validTo.After(now) {
		writeError(w, http.StatusConflict, "CAMPAIGN_EXPIRED", "coupon validity has ended")
		return
	}
	snapshot, _ := json.Marshal(marketingCouponView(campaign))
	couponNo := newBusinessNo("CP")
	result, err := tx.ExecContext(r.Context(), `INSERT INTO customer_coupons(tenant_id,store_id,campaign_id,coupon_no,subject_key_hash,source,status,campaign_snapshot_json,valid_from,valid_to,idempotency_key,request_fingerprint)
		VALUES(?,?,?,?,?,'PUBLIC_CLAIM','PROVISIONAL',?,?,?,?,?)`, store.TenantID, store.ID, campaignID, couponNo, subjectHash, string(snapshot), validFrom, validTo, idempotencyKey, fingerprint)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			_ = tx.Rollback()
			if existing, existingFingerprint, found, loadErr := s.loadProvisionalCouponByKey(r.Context(), store.TenantID, idempotencyKey); loadErr == nil && found {
				if existingFingerprint != fingerprint {
					writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used for a different coupon claim")
					return
				}
				writeData(w, http.StatusOK, existing)
				return
			}
		}
		handleSQLError(w, err)
		return
	}
	recordID, _ := result.LastInsertId()
	update, err := tx.ExecContext(r.Context(), "UPDATE coupon_campaigns SET issued_count=issued_count+1 WHERE id=? AND tenant_id=? AND issued_count<total_stock", campaignID, store.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := update.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "COUPON_SOLD_OUT", "coupon campaign stock changed concurrently")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	view := provisionalCouponView(recordID, couponNo, campaignID, campaign.Name, "PUBLIC_CLAIM", validFrom, validTo)
	view["campaign"] = publicMarketingCouponView(campaign)
	writeData(w, http.StatusCreated, view)
}

func provisionalCouponView(id int64, couponNo string, campaignID int64, campaignName, source string, validFrom, validTo time.Time) map[string]any {
	view := marketingProvisionalFields()
	view["id"] = id
	view["coupon_no"] = couponNo
	view["campaign_id"] = campaignID
	view["campaign_name"] = campaignName
	view["source"] = source
	view["status"] = "PROVISIONAL"
	view["valid_from"] = formatBeijingDateTime(validFrom)
	view["valid_to"] = formatBeijingDateTime(validTo)
	return view
}

func (s *Server) loadProvisionalCouponByKey(ctx context.Context, tenantID int64, key string) (map[string]any, string, bool, error) {
	return loadProvisionalCouponByKeyWithQueryer(ctx, s.DB, tenantID, key)
}

type marketingCouponRowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadProvisionalCouponByKeyWithQueryer(ctx context.Context, queryer marketingCouponRowQueryer, tenantID int64, key string) (map[string]any, string, bool, error) {
	var id, campaignID int64
	var no, campaignName, source, fingerprint string
	var validFrom, validTo time.Time
	err := queryer.QueryRowContext(ctx, `SELECT cc.id,cc.coupon_no,cc.campaign_id,c.name,cc.source,cc.valid_from,cc.valid_to,cc.request_fingerprint
		FROM customer_coupons cc JOIN coupon_campaigns c ON c.id=cc.campaign_id AND c.tenant_id=cc.tenant_id WHERE cc.tenant_id=? AND cc.idempotency_key=?`, tenantID, key).
		Scan(&id, &no, &campaignID, &campaignName, &source, &validFrom, &validTo, &fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	return provisionalCouponView(id, no, campaignID, campaignName, source, validFrom, validTo), fingerprint, true, nil
}
