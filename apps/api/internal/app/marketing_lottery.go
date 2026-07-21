package app

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type marketingLotteryPrizeInput struct {
	Name             string `json:"name"`
	PrizeType        string `json:"prize_type"`
	CouponCampaignID *int64 `json:"coupon_campaign_id"`
	Weight           int64  `json:"weight"`
	TotalStock       int64  `json:"total_stock"`
	SortOrder        int    `json:"sort_order"`
	Status           string `json:"status"`
}

type marketingLotteryInput struct {
	Name         string                       `json:"name"`
	Description  string                       `json:"description"`
	ChannelScope string                       `json:"channel_scope"`
	ActiveFrom   time.Time                    `json:"active_from"`
	ActiveTo     time.Time                    `json:"active_to"`
	DailyLimit   int                          `json:"daily_limit"`
	TotalLimit   int                          `json:"total_limit"`
	Terms        string                       `json:"terms"`
	Prizes       []marketingLotteryPrizeInput `json:"prizes"`
}

type marketingLotteryRow struct {
	ID           int64
	TenantID     int64
	StoreID      int64
	Name         string
	Description  string
	ChannelScope string
	ActiveFrom   time.Time
	ActiveTo     time.Time
	DailyLimit   int
	TotalLimit   int
	DrawCount    int64
	Terms        string
	Status       string
	Version      int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	WinCount     int64
	Prizes       []marketingLotteryPrizeRow
}

type marketingLotteryPrizeRow struct {
	ID               int64
	TenantID         int64
	CampaignID       int64
	Name             string
	PrizeType        string
	CouponCampaignID sql.NullInt64
	Weight           int64
	TotalStock       int64
	AwardedCount     int64
	SortOrder        int
	Status           string
}

const marketingLotterySelect = `SELECT l.id,l.tenant_id,l.store_id,l.name,l.description,l.channel_scope,l.active_from,l.active_to,l.daily_limit,l.total_limit,l.draw_count,l.terms,l.status,l.version,l.created_at,l.updated_at,
	(SELECT COUNT(*) FROM lottery_draws d WHERE d.tenant_id=l.tenant_id AND d.campaign_id=l.id AND d.result_type='COUPON')
	FROM lottery_campaigns l`

func scanMarketingLottery(scanner interface{ Scan(...any) error }) (marketingLotteryRow, error) {
	var row marketingLotteryRow
	err := scanner.Scan(&row.ID, &row.TenantID, &row.StoreID, &row.Name, &row.Description, &row.ChannelScope, &row.ActiveFrom, &row.ActiveTo, &row.DailyLimit,
		&row.TotalLimit, &row.DrawCount, &row.Terms, &row.Status, &row.Version, &row.CreatedAt, &row.UpdatedAt, &row.WinCount)
	return row, err
}

func marketingLotteryPrizeView(row marketingLotteryPrizeRow) map[string]any {
	var couponCampaignID any
	if row.CouponCampaignID.Valid {
		couponCampaignID = row.CouponCampaignID.Int64
	}
	return map[string]any{"id": row.ID, "name": row.Name, "prize_type": row.PrizeType, "coupon_campaign_id": couponCampaignID, "weight": row.Weight,
		"total_stock": row.TotalStock, "awarded_count": row.AwardedCount, "sort_order": row.SortOrder, "status": row.Status}
}

func marketingLotteryView(row marketingLotteryRow) map[string]any {
	prizes := make([]map[string]any, 0, len(row.Prizes))
	var totalWeight int64
	for _, prize := range row.Prizes {
		prizes = append(prizes, marketingLotteryPrizeView(prize))
		if prize.Status == "ACTIVE" && prize.Weight > 0 && totalWeight <= math.MaxInt64-prize.Weight {
			totalWeight += prize.Weight
		}
	}
	return map[string]any{
		"id": row.ID, "name": row.Name, "description": row.Description, "channel_scope": row.ChannelScope,
		"active_from": formatBeijingDateTime(row.ActiveFrom), "active_to": formatBeijingDateTime(row.ActiveTo),
		"daily_limit": row.DailyLimit, "total_limit": row.TotalLimit, "limit_scope": "PER_ANONYMOUS_SUBJECT", "draw_count": row.DrawCount,
		"terms": row.Terms, "status": row.Status, "version": row.Version, "win_count": row.WinCount, "total_weight": totalWeight, "prizes": prizes,
		"created_at": formatBeijingDateTime(row.CreatedAt), "updated_at": formatBeijingDateTime(row.UpdatedAt),
	}
}

func publicMarketingLotteryView(row marketingLotteryRow) map[string]any {
	prizes := make([]map[string]any, 0, len(row.Prizes))
	for _, prize := range row.Prizes {
		if prize.Status == "ACTIVE" {
			prizes = append(prizes, marketingLotteryPrizeView(prize))
		}
	}
	return map[string]any{
		"id": row.ID, "name": row.Name, "description": row.Description, "channel_scope": row.ChannelScope,
		"active_from": formatBeijingDateTime(row.ActiveFrom), "active_to": formatBeijingDateTime(row.ActiveTo),
		"daily_limit": row.DailyLimit, "total_limit": row.TotalLimit, "limit_scope": "PER_ANONYMOUS_SUBJECT",
		"draw_count": row.DrawCount, "terms": row.Terms, "status": row.Status, "prizes": prizes,
	}
}

func normalizeMarketingLotteryInput(input *marketingLotteryInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Terms = strings.TrimSpace(input.Terms)
	channel, err := marketingChannel(input.ChannelScope)
	if err != nil {
		return err
	}
	input.ChannelScope = channel
	if input.Name == "" || len([]rune(input.Name)) > 100 || len([]rune(input.Description)) > 500 || input.Terms == "" || len([]rune(input.Terms)) > 5000 {
		return errors.New("lottery name and terms are required and text must stay within its limit")
	}
	if input.ActiveFrom.IsZero() || input.ActiveTo.IsZero() || !input.ActiveFrom.Before(input.ActiveTo) {
		return errors.New("active_from must be before active_to")
	}
	if input.DailyLimit <= 0 || input.DailyLimit > 100 || input.TotalLimit <= 0 || input.TotalLimit > 10000 || input.DailyLimit > input.TotalLimit {
		return errors.New("daily_limit and total_limit are outside the supported range")
	}
	if len(input.Prizes) == 0 || len(input.Prizes) > 50 {
		return errors.New("between 1 and 50 prizes are required")
	}
	var totalWeight int64
	for index := range input.Prizes {
		prize := &input.Prizes[index]
		prize.Name = strings.TrimSpace(prize.Name)
		prize.PrizeType = strings.ToUpper(strings.TrimSpace(prize.PrizeType))
		prize.Status = strings.ToUpper(strings.TrimSpace(prize.Status))
		if prize.Status == "" {
			prize.Status = "ACTIVE"
		}
		if prize.Name == "" || len([]rune(prize.Name)) > 100 || !validStatus(prize.PrizeType, "THANKS", "COUPON") || !validStatus(prize.Status, "ACTIVE", "DISABLED") {
			return errors.New("prize name, prize_type or status is invalid")
		}
		if prize.Weight <= 0 || prize.Weight > 1_000_000_000_000 || totalWeight > math.MaxInt64-prize.Weight {
			return errors.New("prize weights are outside the supported range")
		}
		totalWeight += prize.Weight
		if prize.PrizeType == "COUPON" {
			if prize.CouponCampaignID == nil || *prize.CouponCampaignID <= 0 || prize.TotalStock <= 0 || prize.TotalStock > 1_000_000_000 {
				return errors.New("COUPON prize requires coupon_campaign_id and positive total_stock")
			}
		} else {
			if prize.CouponCampaignID != nil {
				return errors.New("THANKS prize cannot reference a coupon campaign")
			}
			prize.TotalStock = 0
		}
	}
	return nil
}

func (s *Server) loadMarketingLottery(ctx context.Context, tenantID, storeID, id int64) (marketingLotteryRow, error) {
	row, err := scanMarketingLottery(s.DB.QueryRowContext(ctx, marketingLotterySelect+" WHERE l.id=? AND l.tenant_id=? AND l.store_id=? AND l.deleted_at IS NULL", id, tenantID, storeID))
	if err != nil {
		return row, err
	}
	row.Prizes, err = s.loadMarketingLotteryPrizes(ctx, s.DB, tenantID, id, false)
	return row, err
}

type marketingPrizeQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Server) loadMarketingLotteryPrizes(ctx context.Context, queryer marketingPrizeQueryer, tenantID, campaignID int64, forUpdate bool) ([]marketingLotteryPrizeRow, error) {
	query := `SELECT id,tenant_id,campaign_id,name,prize_type,coupon_campaign_id,weight,total_stock,awarded_count,sort_order,status
		FROM lottery_prizes WHERE tenant_id=? AND campaign_id=? ORDER BY sort_order,id`
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := queryer.QueryContext(ctx, query, tenantID, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]marketingLotteryPrizeRow, 0)
	for rows.Next() {
		var row marketingLotteryPrizeRow
		if err = rows.Scan(&row.ID, &row.TenantID, &row.CampaignID, &row.Name, &row.PrizeType, &row.CouponCampaignID, &row.Weight, &row.TotalStock, &row.AwardedCount, &row.SortOrder, &row.Status); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *Server) listMarketingLotteries(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	page, size, offset := pagination(r)
	where := " WHERE l.tenant_id=? AND l.store_id=? AND l.deleted_at IS NULL"
	args := []any{actor.TenantID, storeID}
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" {
		if !validStatus(status, "DRAFT", "ACTIVE", "PAUSED", "ENDED") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status is invalid")
			return
		}
		where += " AND l.status=?"
		args = append(args, status)
	}
	var total int
	if err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM lottery_campaigns l"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), marketingLotterySelect+where+" ORDER BY l.id DESC LIMIT ? OFFSET ?", append(args, size, offset)...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	lotteries := make([]marketingLotteryRow, 0)
	for rows.Next() {
		row, scanErr := scanMarketingLottery(rows)
		if scanErr != nil {
			_ = rows.Close()
			handleSQLError(w, scanErr)
			return
		}
		lotteries = append(lotteries, row)
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
	items := make([]map[string]any, 0, len(lotteries))
	for _, row := range lotteries {
		row.Prizes, err = s.loadMarketingLotteryPrizes(r.Context(), s.DB, actor.TenantID, row.ID, false)
		if err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, marketingLotteryView(row))
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) getMarketingLottery(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "lotteryID")
	if !ok {
		return
	}
	row, err := s.loadMarketingLottery(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingLotteryView(row))
}

func (s *Server) createMarketingLottery(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input marketingLotteryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err = normalizeMarketingLotteryInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `INSERT INTO lottery_campaigns(tenant_id,store_id,name,description,channel_scope,active_from,active_to,daily_limit,total_limit,terms,status,created_by,updated_by)
		VALUES(?,?,?,?,?,?,?,?,?,?,'DRAFT',?,?)`, actor.TenantID, storeID, input.Name, input.Description, input.ChannelScope, formatBeijingDateTime(input.ActiveFrom), formatBeijingDateTime(input.ActiveTo), input.DailyLimit, input.TotalLimit, input.Terms, actor.UserID, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	if err = insertMarketingLotteryPrizes(r.Context(), tx, actor.TenantID, id, input.Prizes); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "marketing.lottery.create", "lottery_campaign", int64String(id), input, r)
	row, err := s.loadMarketingLottery(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, marketingLotteryView(row))
}

func insertMarketingLotteryPrizes(ctx context.Context, tx *sql.Tx, tenantID, campaignID int64, prizes []marketingLotteryPrizeInput) error {
	for _, prize := range prizes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO lottery_prizes(tenant_id,campaign_id,name,prize_type,coupon_campaign_id,weight,total_stock,sort_order,status)
			VALUES(?,?,?,?,?,?,?,?,?)`, tenantID, campaignID, prize.Name, prize.PrizeType, nullableMarketingID(prize.CouponCampaignID), prize.Weight, prize.TotalStock, prize.SortOrder, prize.Status); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) updateMarketingLottery(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "lotteryID")
	if !ok {
		return
	}
	current, err := s.loadMarketingLottery(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if current.Status == "ACTIVE" || current.DrawCount > 0 {
		writeError(w, http.StatusConflict, "LOTTERY_IMMUTABLE", "active or already drawn lottery must be paused and cloned instead of edited")
		return
	}
	var input marketingLotteryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err = normalizeMarketingLotteryInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE lottery_campaigns SET name=?,description=?,channel_scope=?,active_from=?,active_to=?,daily_limit=?,total_limit=?,terms=?,version=version+1,updated_by=?
		WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status<>'ACTIVE' AND draw_count=0`, input.Name, input.Description, input.ChannelScope, formatBeijingDateTime(input.ActiveFrom), formatBeijingDateTime(input.ActiveTo), input.DailyLimit, input.TotalLimit, input.Terms, actor.UserID, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "LOTTERY_CHANGED", "lottery changed concurrently")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM lottery_prizes WHERE tenant_id=? AND campaign_id=?", actor.TenantID, id); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = insertMarketingLotteryPrizes(r.Context(), tx, actor.TenantID, id, input.Prizes); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "marketing.lottery.update", "lottery_campaign", int64String(id), input, r)
	row, err := s.loadMarketingLottery(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingLotteryView(row))
}

func (s *Server) deleteMarketingLottery(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "lotteryID")
	if !ok {
		return
	}
	var placementReferences int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM marketing_placements WHERE tenant_id=? AND store_id=? AND action_type='OPEN_LOTTERY' AND action_target_id=? AND deleted_at IS NULL`, actor.TenantID, storeID, id).Scan(&placementReferences); err != nil {
		handleSQLError(w, err)
		return
	}
	if placementReferences > 0 {
		writeError(w, http.StatusConflict, "LOTTERY_IN_USE", "lottery referenced by a popup placement cannot be deleted")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), "UPDATE lottery_campaigns SET status='ENDED',deleted_at=NOW(3),updated_by=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status IN ('DRAFT','PAUSED','ENDED') AND draw_count=0", actor.UserID, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "LOTTERY_IN_USE", "active or already drawn lottery cannot be deleted")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM lottery_prizes WHERE tenant_id=? AND campaign_id=?", actor.TenantID, id); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "marketing.lottery.delete", "lottery_campaign", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) activateMarketingLottery(w http.ResponseWriter, r *http.Request) {
	s.transitionMarketingLottery(w, r, "ACTIVE")
}

func (s *Server) pauseMarketingLottery(w http.ResponseWriter, r *http.Request) {
	s.transitionMarketingLottery(w, r, "PAUSED")
}

func (s *Server) transitionMarketingLottery(w http.ResponseWriter, r *http.Request, target string) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "lotteryID")
	if !ok {
		return
	}
	row, err := s.loadMarketingLottery(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if target == "ACTIVE" {
		if row.ChannelScope == "DELIVERY" {
			writeError(w, http.StatusConflict, "DELIVERY_NOT_AVAILABLE", "delivery lottery campaigns are reserved but cannot be activated in this release")
			return
		}
		if !row.ActiveTo.After(time.Now().UTC()) {
			writeError(w, http.StatusConflict, "LOTTERY_EXPIRED", "expired lottery cannot be activated")
			return
		}
		if !validStatus(row.Status, "DRAFT", "PAUSED") {
			writeError(w, http.StatusConflict, "INVALID_STATUS", "lottery cannot be activated from its current status")
			return
		}
		if err = s.validateMarketingLotteryPrizes(r.Context(), row); err != nil {
			writeError(w, http.StatusConflict, "INVALID_PRIZES", err.Error())
			return
		}
	} else if row.Status != "ACTIVE" {
		writeError(w, http.StatusConflict, "INVALID_STATUS", "only an active lottery can be paused")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE lottery_campaigns SET status=?,version=version+1,updated_by=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL AND status=?", target, actor.UserID, id, actor.TenantID, storeID, row.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "LOTTERY_CHANGED", "lottery changed concurrently")
		return
	}
	s.audit(r.Context(), actor, "marketing.lottery."+strings.ToLower(target), "lottery_campaign", int64String(id), map[string]any{"from": row.Status, "to": target}, r)
	updated, err := s.loadMarketingLottery(r.Context(), actor.TenantID, storeID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, marketingLotteryView(updated))
}

func (s *Server) validateMarketingLotteryPrizes(ctx context.Context, lottery marketingLotteryRow) error {
	if len(lottery.Prizes) == 0 {
		return errors.New("lottery requires at least one prize")
	}
	var totalWeight int64
	for _, prize := range lottery.Prizes {
		if prize.Status != "ACTIVE" {
			continue
		}
		if prize.Weight <= 0 || totalWeight > math.MaxInt64-prize.Weight {
			return errors.New("active prize weights are invalid")
		}
		totalWeight += prize.Weight
		if prize.PrizeType != "COUPON" {
			continue
		}
		if !prize.CouponCampaignID.Valid {
			return errors.New("coupon prize is missing coupon_campaign_id")
		}
		var count int
		if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM coupon_campaigns WHERE id=? AND tenant_id=? AND store_id=? AND distribution_mode='LOTTERY_ONLY' AND status='ACTIVE' AND deleted_at IS NULL`, prize.CouponCampaignID.Int64, lottery.TenantID, lottery.StoreID).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return errors.New("coupon prizes must reference an active LOTTERY_ONLY coupon campaign in the same store")
		}
	}
	if totalWeight <= 0 {
		return errors.New("lottery requires a positive total prize weight")
	}
	return nil
}

func (s *Server) publicListMarketingLotteries(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
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
	now := time.Now().UTC()
	rows, err := s.DB.QueryContext(r.Context(), marketingLotterySelect+` WHERE l.tenant_id=? AND l.store_id=? AND l.deleted_at IS NULL AND l.status='ACTIVE' AND (l.channel_scope='ALL' OR l.channel_scope=?) AND l.active_from<=? AND l.active_to>?
		ORDER BY l.id DESC`, store.TenantID, store.ID, channelScope, now, now)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	lotteries := make([]marketingLotteryRow, 0)
	for rows.Next() {
		row, scanErr := scanMarketingLottery(rows)
		if scanErr != nil {
			_ = rows.Close()
			handleSQLError(w, scanErr)
			return
		}
		lotteries = append(lotteries, row)
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
	items := make([]map[string]any, 0, len(lotteries))
	for _, row := range lotteries {
		row.Prizes, err = s.loadMarketingLotteryPrizes(r.Context(), s.DB, store.TenantID, row.ID, false)
		if err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, publicMarketingLotteryView(row))
	}
	writeData(w, http.StatusOK, map[string]any{"items": items, "identity_verified": false, "asset_issuance_mode": "PROVISIONAL_ONLY", "warning": provisionalMarketingAssetWarning})
}

func (s *Server) publicGetMarketingLottery(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
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
	id, ok := pathID(w, r, "lotteryID")
	if !ok {
		return
	}
	row, err := s.loadMarketingLottery(r.Context(), store.TenantID, store.ID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	now := time.Now().UTC()
	if row.Status != "ACTIVE" || (row.ChannelScope != "ALL" && row.ChannelScope != channelScope) || now.Before(row.ActiveFrom) || !now.Before(row.ActiveTo) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "active lottery not found")
		return
	}
	view := publicMarketingLotteryView(row)
	view["identity_verified"] = false
	view["asset_issuance_mode"] = "PROVISIONAL_ONLY"
	view["warning"] = provisionalMarketingAssetWarning
	writeData(w, http.StatusOK, view)
}

func secureMarketingRandomBelow(limit int64) (int64, error) {
	if limit <= 0 {
		return 0, errors.New("random limit must be positive")
	}
	value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(limit))
	if err != nil {
		return 0, err
	}
	return value.Int64(), nil
}

func chooseWeightedMarketingPrize(prizes []marketingLotteryPrizeRow, ticket int64) (marketingLotteryPrizeRow, int64, error) {
	var total int64
	for _, prize := range prizes {
		if prize.Status != "ACTIVE" || prize.Weight <= 0 {
			continue
		}
		if total > math.MaxInt64-prize.Weight {
			return marketingLotteryPrizeRow{}, 0, errors.New("total prize weight overflow")
		}
		total += prize.Weight
	}
	if total <= 0 || ticket < 0 || ticket >= total {
		return marketingLotteryPrizeRow{}, total, errors.New("lottery ticket is outside the total weight")
	}
	var cursor int64
	for _, prize := range prizes {
		if prize.Status != "ACTIVE" || prize.Weight <= 0 {
			continue
		}
		cursor += prize.Weight
		if ticket < cursor {
			return prize, total, nil
		}
	}
	return marketingLotteryPrizeRow{}, total, errors.New("lottery prize could not be selected")
}

func (s *Server) publicDrawMarketingLottery(w http.ResponseWriter, r *http.Request) {
	store, err := s.findPublicStore(r.Context(), chi.URLParam(r, "storeCode"))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	campaignID, ok := pathID(w, r, "lotteryID")
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
	if view, existingFingerprint, found, loadErr := s.loadMarketingDrawByKey(r.Context(), store.TenantID, idempotencyKey); loadErr != nil {
		handleSQLError(w, loadErr)
		return
	} else if found {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used for a different lottery draw")
			return
		}
		writeData(w, http.StatusOK, view)
		return
	}
	if !s.allowPublicMarketingMutation(r.Context(), r, store.Code, "lottery-draw") {
		writeError(w, http.StatusTooManyRequests, "MARKETING_RATE_LIMITED", "too many lottery draw attempts; try again later")
		return
	}

	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var lottery marketingLotteryRow
	err = tx.QueryRowContext(r.Context(), `SELECT id,tenant_id,store_id,name,description,channel_scope,active_from,active_to,daily_limit,total_limit,draw_count,terms,status,version,created_at,updated_at,0
		FROM lottery_campaigns WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL FOR UPDATE`, campaignID, store.TenantID, store.ID).
		Scan(&lottery.ID, &lottery.TenantID, &lottery.StoreID, &lottery.Name, &lottery.Description, &lottery.ChannelScope, &lottery.ActiveFrom, &lottery.ActiveTo, &lottery.DailyLimit, &lottery.TotalLimit,
			&lottery.DrawCount, &lottery.Terms, &lottery.Status, &lottery.Version, &lottery.CreatedAt, &lottery.UpdatedAt, &lottery.WinCount)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if view, existingFingerprint, found, loadErr := loadMarketingDrawByKeyWithQueryer(r.Context(), tx, store.TenantID, idempotencyKey); loadErr != nil {
		handleSQLError(w, loadErr)
		return
	} else if found {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used for a different lottery draw")
			return
		}
		writeData(w, http.StatusOK, view)
		return
	}
	now := time.Now().UTC()
	if lottery.Status != "ACTIVE" || lottery.ChannelScope == "DELIVERY" || now.Before(lottery.ActiveFrom) || !now.Before(lottery.ActiveTo) {
		writeError(w, http.StatusConflict, "LOTTERY_NOT_ACTIVE", "lottery is not active")
		return
	}
	var storeTimezone string
	if err = tx.QueryRowContext(r.Context(), "SELECT timezone FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL", store.ID, store.TenantID).Scan(&storeTimezone); err != nil {
		handleSQLError(w, err)
		return
	}
	businessDate := marketingBusinessDate(now, storeTimezone)
	var todayDraws, allDraws int
	if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM lottery_draws WHERE tenant_id=? AND campaign_id=? AND subject_key_hash=? AND business_date=?", store.TenantID, campaignID, subjectHash, businessDate).Scan(&todayDraws); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM lottery_draws WHERE tenant_id=? AND campaign_id=? AND subject_key_hash=?", store.TenantID, campaignID, subjectHash).Scan(&allDraws); err != nil {
		handleSQLError(w, err)
		return
	}
	if todayDraws >= lottery.DailyLimit || allDraws >= lottery.TotalLimit {
		writeError(w, http.StatusConflict, "DRAW_LIMIT_REACHED", "anonymous subject draw limit was reached")
		return
	}
	prizes, err := s.loadMarketingLotteryPrizes(r.Context(), tx, store.TenantID, campaignID, true)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var totalWeight int64
	for _, prize := range prizes {
		if prize.Status == "ACTIVE" && prize.Weight > 0 {
			if totalWeight > math.MaxInt64-prize.Weight {
				writeError(w, http.StatusConflict, "INVALID_PRIZES", "total prize weight overflow")
				return
			}
			totalWeight += prize.Weight
		}
	}
	if totalWeight <= 0 {
		writeError(w, http.StatusConflict, "INVALID_PRIZES", "lottery has no active positive-weight prizes")
		return
	}
	ticket, err := secureMarketingRandomBelow(totalWeight)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "RANDOM_SOURCE_ERROR", "secure lottery random source is unavailable")
		return
	}
	selected, _, err := chooseWeightedMarketingPrize(prizes, ticket)
	if err != nil {
		writeError(w, http.StatusConflict, "INVALID_PRIZES", err.Error())
		return
	}
	resultType := "THANKS"
	resultReason := "SELECTED_THANKS"
	var couponRecordID any
	var provisionalCoupon map[string]any
	if selected.PrizeType == "COUPON" {
		resultReason = "PRIZE_UNAVAILABLE"
		if selected.AwardedCount < selected.TotalStock && selected.CouponCampaignID.Valid {
			couponIdempotencyKey := "system:lottery_coupon:" + requestFingerprint(idempotencyKey)
			provisionalCoupon, couponRecordID, err = issueLotteryProvisionalCoupon(r.Context(), tx, store, selected, subjectHash, couponIdempotencyKey, fingerprint, now)
			if err != nil && !errors.Is(err, errMarketingPrizeUnavailable) {
				handleSQLError(w, err)
				return
			}
			if err == nil {
				resultType = "COUPON"
				resultReason = "AWARDED"
				update, updateErr := tx.ExecContext(r.Context(), "UPDATE lottery_prizes SET awarded_count=awarded_count+1 WHERE id=? AND tenant_id=? AND awarded_count<total_stock", selected.ID, store.TenantID)
				if updateErr != nil {
					handleSQLError(w, updateErr)
					return
				}
				if changed, _ := update.RowsAffected(); changed != 1 {
					writeError(w, http.StatusConflict, "PRIZE_STOCK_CHANGED", "lottery prize stock changed concurrently")
					return
				}
			}
		}
	}
	prizeSnapshot, _ := json.Marshal(marketingLotteryPrizeView(selected))
	drawNo := newBusinessNo("LD")
	result, err := tx.ExecContext(r.Context(), `INSERT INTO lottery_draws(tenant_id,store_id,campaign_id,prize_id,draw_no,subject_key_hash,business_date,result_type,result_reason,customer_coupon_id,prize_snapshot_json,random_value,total_weight,idempotency_key,request_fingerprint)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, store.TenantID, store.ID, campaignID, selected.ID, drawNo, subjectHash, businessDate, resultType, resultReason, couponRecordID, string(prizeSnapshot), ticket, totalWeight, idempotencyKey, fingerprint)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			_ = tx.Rollback()
			if view, existingFingerprint, found, loadErr := s.loadMarketingDrawByKey(r.Context(), store.TenantID, idempotencyKey); loadErr == nil && found {
				if existingFingerprint != fingerprint {
					writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used for a different lottery draw")
					return
				}
				writeData(w, http.StatusOK, view)
				return
			}
		}
		handleSQLError(w, err)
		return
	}
	drawID, _ := result.LastInsertId()
	drawUpdate, err := tx.ExecContext(r.Context(), "UPDATE lottery_campaigns SET draw_count=draw_count+1 WHERE id=? AND tenant_id=?", campaignID, store.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := drawUpdate.RowsAffected(); changed != 1 {
		writeError(w, http.StatusConflict, "LOTTERY_CHANGED", "lottery changed concurrently")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	view := marketingDrawView(drawID, drawNo, campaignID, lottery.Name, selected, resultType, resultReason, provisionalCoupon, now)
	writeData(w, http.StatusCreated, view)
}

var errMarketingPrizeUnavailable = errors.New("marketing prize is unavailable")

var errMarketingInventoryConflict = errors.New("marketing inventory changed unexpectedly")

func issueLotteryProvisionalCoupon(ctx context.Context, tx *sql.Tx, store storeDTO, prize marketingLotteryPrizeRow, subjectHash, idempotencyKey, parentFingerprint string, now time.Time) (map[string]any, any, error) {
	var campaign marketingCouponRow
	err := tx.QueryRowContext(ctx, `SELECT id,tenant_id,store_id,name,description,coupon_type,threshold_cents,discount_cents,distribution_mode,total_stock,issued_count,per_subject_limit,
		claim_start_at,claim_end_at,validity_mode,valid_from,valid_to,valid_days,order_types_json,status,version,created_at,updated_at,0,0
		FROM coupon_campaigns WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL FOR UPDATE`, prize.CouponCampaignID.Int64, store.TenantID, store.ID).
		Scan(&campaign.ID, &campaign.TenantID, &campaign.StoreID, &campaign.Name, &campaign.Description, &campaign.CouponType, &campaign.ThresholdCents, &campaign.DiscountCents,
			&campaign.DistributionMode, &campaign.TotalStock, &campaign.IssuedCount, &campaign.PerSubjectLimit, &campaign.ClaimStartAt, &campaign.ClaimEndAt, &campaign.ValidityMode,
			&campaign.ValidFrom, &campaign.ValidTo, &campaign.ValidDays, &campaign.OrderTypesJSON, &campaign.Status, &campaign.Version, &campaign.CreatedAt, &campaign.UpdatedAt, &campaign.ClaimCount, &campaign.UseCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, errMarketingPrizeUnavailable
	}
	if err != nil {
		return nil, nil, err
	}
	if campaign.Status != "ACTIVE" || campaign.DistributionMode != "LOTTERY_ONLY" || campaign.IssuedCount >= campaign.TotalStock ||
		(campaign.ClaimStartAt.Valid && now.Before(campaign.ClaimStartAt.Time)) || (campaign.ClaimEndAt.Valid && !now.Before(campaign.ClaimEndAt.Time)) {
		return nil, nil, errMarketingPrizeUnavailable
	}
	var subjectClaims int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM customer_coupons WHERE tenant_id=? AND campaign_id=? AND subject_key_hash=? AND status<>'VOID'", store.TenantID, campaign.ID, subjectHash).Scan(&subjectClaims); err != nil {
		return nil, nil, err
	}
	if subjectClaims >= campaign.PerSubjectLimit {
		return nil, nil, errMarketingPrizeUnavailable
	}
	validFrom, validTo := now, now.Add(time.Duration(campaign.ValidDays)*24*time.Hour)
	if campaign.ValidityMode == "FIXED_RANGE" {
		validFrom, validTo = campaign.ValidFrom.Time, campaign.ValidTo.Time
	}
	if !validTo.After(now) {
		return nil, nil, errMarketingPrizeUnavailable
	}
	update, err := tx.ExecContext(ctx, "UPDATE coupon_campaigns SET issued_count=issued_count+1 WHERE id=? AND tenant_id=? AND issued_count<total_stock", campaign.ID, store.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if changed, _ := update.RowsAffected(); changed != 1 {
		return nil, nil, errMarketingInventoryConflict
	}
	snapshot, _ := json.Marshal(marketingCouponView(campaign))
	couponNo := newBusinessNo("CP")
	fingerprint := requestFingerprint(map[string]any{"parent_fingerprint": parentFingerprint, "campaign_id": campaign.ID, "subject_key_hash": subjectHash})
	result, err := tx.ExecContext(ctx, `INSERT INTO customer_coupons(tenant_id,store_id,campaign_id,coupon_no,subject_key_hash,source,status,campaign_snapshot_json,valid_from,valid_to,idempotency_key,request_fingerprint)
		VALUES(?,?,?,?,?,'LOTTERY','PROVISIONAL',?,?,?,?,?)`, store.TenantID, store.ID, campaign.ID, couponNo, subjectHash, string(snapshot), validFrom, validTo, idempotencyKey, fingerprint)
	if err != nil {
		return nil, nil, err
	}
	id, _ := result.LastInsertId()
	return provisionalCouponView(id, couponNo, campaign.ID, campaign.Name, "LOTTERY", validFrom, validTo), id, nil
}

func marketingDrawView(id int64, drawNo string, campaignID int64, campaignName string, prize marketingLotteryPrizeRow, resultType, resultReason string, coupon map[string]any, createdAt time.Time) map[string]any {
	view := marketingProvisionalFields()
	view["id"] = id
	view["draw_id"] = id
	view["draw_no"] = drawNo
	view["campaign_id"] = campaignID
	view["campaign_name"] = campaignName
	view["result_type"] = resultType
	view["result_reason"] = resultReason
	view["prize_type"] = resultType
	if resultType == "COUPON" || prize.PrizeType == "THANKS" {
		view["prize_name"] = prize.Name
	} else {
		view["prize_name"] = "谢谢参与"
	}
	view["prize"] = marketingLotteryPrizeView(prize)
	view["coupon"] = coupon
	view["created_at"] = formatBeijingDateTime(createdAt)
	return view
}

func (s *Server) loadMarketingDrawByKey(ctx context.Context, tenantID int64, key string) (map[string]any, string, bool, error) {
	return loadMarketingDrawByKeyWithQueryer(ctx, s.DB, tenantID, key)
}

func loadMarketingDrawByKeyWithQueryer(ctx context.Context, queryer marketingCouponRowQueryer, tenantID int64, key string) (map[string]any, string, bool, error) {
	var id, campaignID int64
	var prizeID, couponRecordID, couponCampaignID sql.NullInt64
	var drawNo, campaignName, resultType, resultReason, prizeSnapshot, fingerprint string
	var couponNo, couponCampaignName, couponSource sql.NullString
	var couponValidFrom, couponValidTo sql.NullTime
	var createdAt time.Time
	err := queryer.QueryRowContext(ctx, `SELECT d.id,d.draw_no,d.campaign_id,l.name,d.prize_id,d.result_type,d.result_reason,d.prize_snapshot_json,d.request_fingerprint,d.created_at,
		d.customer_coupon_id,cc.coupon_no,cc.campaign_id,c.name,cc.source,cc.valid_from,cc.valid_to
		FROM lottery_draws d
		JOIN lottery_campaigns l ON l.id=d.campaign_id AND l.tenant_id=d.tenant_id
		LEFT JOIN customer_coupons cc ON cc.id=d.customer_coupon_id AND cc.tenant_id=d.tenant_id
		LEFT JOIN coupon_campaigns c ON c.id=cc.campaign_id AND c.tenant_id=cc.tenant_id
		WHERE d.tenant_id=? AND d.idempotency_key=?`, tenantID, key).
		Scan(&id, &drawNo, &campaignID, &campaignName, &prizeID, &resultType, &resultReason, &prizeSnapshot, &fingerprint, &createdAt,
			&couponRecordID, &couponNo, &couponCampaignID, &couponCampaignName, &couponSource, &couponValidFrom, &couponValidTo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	var prize marketingLotteryPrizeRow
	if prizeID.Valid {
		prize.ID = prizeID.Int64
	}
	var raw map[string]any
	if err = json.Unmarshal([]byte(prizeSnapshot), &raw); err != nil {
		return nil, "", false, err
	}
	prize.Name, _ = raw["name"].(string)
	prize.PrizeType, _ = raw["prize_type"].(string)
	prize.Weight = int64FromJSON(raw["weight"])
	prize.TotalStock = int64FromJSON(raw["total_stock"])
	prize.AwardedCount = int64FromJSON(raw["awarded_count"])
	prize.SortOrder = int(int64FromJSON(raw["sort_order"]))
	prize.Status, _ = raw["status"].(string)
	if snapshotCouponCampaignID := int64FromJSON(raw["coupon_campaign_id"]); snapshotCouponCampaignID > 0 {
		prize.CouponCampaignID = sql.NullInt64{Int64: snapshotCouponCampaignID, Valid: true}
	}
	var coupon map[string]any
	if couponRecordID.Valid && couponCampaignID.Valid && couponNo.Valid && couponCampaignName.Valid && couponSource.Valid && couponValidFrom.Valid && couponValidTo.Valid {
		coupon = provisionalCouponView(couponRecordID.Int64, couponNo.String, couponCampaignID.Int64, couponCampaignName.String, couponSource.String, couponValidFrom.Time, couponValidTo.Time)
	}
	view := marketingDrawView(id, drawNo, campaignID, campaignName, prize, resultType, resultReason, coupon, createdAt)
	return view, fingerprint, true, nil
}

func int64FromJSON(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}

func (s *Server) listMarketingLotteryDraws(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	page, size, offset := pagination(r)
	where := " WHERE d.tenant_id=? AND d.store_id=?"
	args := []any{actor.TenantID, storeID}
	if raw := strings.TrimSpace(r.URL.Query().Get("campaign_id")); raw != "" {
		id, parseErr := strconvParseInt(raw)
		if parseErr != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "campaign_id must be a positive integer")
			return
		}
		where += " AND d.campaign_id=?"
		args = append(args, id)
	}
	var total int
	if err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM lottery_draws d"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT d.id,d.draw_no,d.campaign_id,l.name,d.result_type,d.result_reason,d.subject_key_hash,d.business_date,d.prize_snapshot_json,d.created_at
		FROM lottery_draws d JOIN lottery_campaigns l ON l.id=d.campaign_id AND l.tenant_id=d.tenant_id`+where+` ORDER BY d.id DESC LIMIT ? OFFSET ?`, append(args, size, offset)...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, campaignID int64
		var drawNo, campaignName, resultType, reason, subject, prizeSnapshot string
		var businessDate, createdAt time.Time
		if err = rows.Scan(&id, &drawNo, &campaignID, &campaignName, &resultType, &reason, &subject, &businessDate, &prizeSnapshot, &createdAt); err != nil {
			handleSQLError(w, err)
			return
		}
		var prize any
		_ = json.Unmarshal([]byte(prizeSnapshot), &prize)
		items = append(items, map[string]any{"id": id, "draw_no": drawNo, "campaign_id": campaignID, "campaign_name": campaignName, "result_type": resultType, "result_reason": reason,
			"subject_key_mask": marketingSubjectMask(subject), "business_date": businessDate.Format("2006-01-02"), "prize": prize, "created_at": formatBeijingDateTime(createdAt)})
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeList(w, http.StatusOK, items, total, page, size)
}
