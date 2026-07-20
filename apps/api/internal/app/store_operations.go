package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const defaultStoreTimezone = "Asia/Shanghai"

var errStoreClosed = errors.New("store is outside business hours")

type weeklyBusinessPeriod struct {
	ID          int64  `json:"id,omitempty"`
	Weekday     int    `json:"weekday"`
	Start       string `json:"start"`
	End         string `json:"end"`
	StartMinute int    `json:"-"`
	EndMinute   int    `json:"-"`
}

type weeklyBusinessDay struct {
	Weekday int                    `json:"weekday"`
	Periods []weeklyBusinessPeriod `json:"periods"`
}

type businessOverride struct {
	ID       int64
	Kind     string
	StartsAt time.Time
	EndsAt   time.Time
	Reason   string
}

type storeBusinessState struct {
	Open         bool
	Reason       string
	Message      string
	Timezone     string
	BusinessDate string
	NextOpenAt   *time.Time
	Override     *businessOverride
}

func clockMinute(value string, allow24 bool) (int, error) {
	value = strings.TrimSpace(value)
	if allow24 && value == "24:00" {
		return 1440, nil
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil || len(value) != 5 {
		return 0, fmt.Errorf("time %q must use HH:mm", value)
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func minuteClock(value int) string {
	if value == 1440 {
		return "24:00"
	}
	return fmt.Sprintf("%02d:%02d", value/60, value%60)
}

func isoWeekday(value time.Weekday) int {
	if value == time.Sunday {
		return 7
	}
	return int(value)
}

func dayStart(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func wallClockTime(base time.Time, minute, dayOffset int) time.Time {
	if minute == 1440 {
		minute = 0
		dayOffset++
	}
	date := dayStart(base).AddDate(0, 0, dayOffset)
	return time.Date(date.Year(), date.Month(), date.Day(), minute/60, minute%60, 0, 0, base.Location())
}

func periodWindow(base time.Time, period weeklyBusinessPeriod) (time.Time, time.Time) {
	start := wallClockTime(base, period.StartMinute, 0)
	endDayOffset := 0
	// Equal endpoints intentionally mean a 24-hour period. This keeps the
	// migrated 00:00-00:00 default explicit instead of accidentally closing a
	// previously always-open store.
	if period.EndMinute <= period.StartMinute {
		endDayOffset = 1
	}
	end := wallClockTime(base, period.EndMinute, endDayOffset)
	return start, end
}

func scheduledBusinessState(now time.Time, timezone string, periods []weeklyBusinessPeriod) storeBusinessState {
	state := storeBusinessState{Timezone: timezone, Reason: "WEEKLY_SCHEDULE", Message: "休息中"}
	localNow := now
	var next *time.Time
	for offset := -1; offset <= 8; offset++ {
		base := dayStart(localNow).AddDate(0, 0, offset)
		weekday := isoWeekday(base.Weekday())
		for _, period := range periods {
			if period.Weekday != weekday {
				continue
			}
			start, end := periodWindow(base, period)
			if !localNow.Before(start) && localNow.Before(end) {
				state.Open = true
				state.Message = "营业中"
				state.BusinessDate = base.Format("2006-01-02")
				return state
			}
			if start.After(localNow) && (next == nil || start.Before(*next)) {
				candidate := start
				next = &candidate
			}
		}
	}
	state.BusinessDate = localNow.Format("2006-01-02")
	state.NextOpenAt = next
	return state
}

func evaluateStoreBusinessState(now time.Time, timezone string, periods []weeklyBusinessPeriod, override *businessOverride) (storeBusinessState, error) {
	if strings.TrimSpace(timezone) == "" {
		timezone = defaultStoreTimezone
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return storeBusinessState{}, fmt.Errorf("invalid store timezone: %w", err)
	}
	localNow := now.In(location)
	state := scheduledBusinessState(localNow, timezone, periods)
	if override == nil || now.Before(override.StartsAt) || !now.Before(override.EndsAt) {
		return state, nil
	}
	state.Override = override
	if override.Kind == "OPEN" {
		state.Open = true
		state.Reason = "TEMPORARY_OPEN"
		state.Message = strings.TrimSpace(override.Reason)
		if state.Message == "" {
			state.Message = "临时营业中"
		}
		state.BusinessDate = localNow.Format("2006-01-02")
		state.NextOpenAt = nil
		return state, nil
	}
	state.Open = false
	state.Reason = "TEMPORARY_CLOSED"
	state.Message = strings.TrimSpace(override.Reason)
	if state.Message == "" {
		state.Message = "临时休息中"
	}
	resume := override.EndsAt.In(location)
	resumeState := scheduledBusinessState(resume, timezone, periods)
	if resumeState.Open {
		state.NextOpenAt = &resume
	} else {
		state.NextOpenAt = resumeState.NextOpenAt
	}
	return state, nil
}

func loadBusinessPeriods(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64) ([]weeklyBusinessPeriod, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT id,weekday,start_minute,end_minute FROM store_business_periods
		WHERE tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL ORDER BY weekday,sort_order,id`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	periods := []weeklyBusinessPeriod{}
	for rows.Next() {
		var period weeklyBusinessPeriod
		if err = rows.Scan(&period.ID, &period.Weekday, &period.StartMinute, &period.EndMinute); err != nil {
			return nil, err
		}
		period.Start, period.End = minuteClock(period.StartMinute), minuteClock(period.EndMinute)
		periods = append(periods, period)
	}
	return periods, rows.Err()
}

func loadCurrentBusinessOverride(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64, now time.Time) (*businessOverride, error) {
	var item businessOverride
	var startsAt, endsAt string
	nowValue := now.UTC().Format("2006-01-02 15:04:05.000")
	err := queryer.QueryRowContext(ctx, `SELECT id,override_type,DATE_FORMAT(starts_at,'%Y-%m-%dT%H:%i:%sZ'),DATE_FORMAT(ends_at,'%Y-%m-%dT%H:%i:%sZ'),reason FROM store_business_overrides
		WHERE tenant_id=? AND store_id=? AND status='ACTIVE' AND starts_at<=? AND ends_at>?
		ORDER BY id DESC LIMIT 1`, tenantID, storeID, nowValue, nowValue).
		Scan(&item.ID, &item.Kind, &startsAt, &endsAt, &item.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.StartsAt, err = time.Parse(time.RFC3339, startsAt)
	if err != nil {
		return nil, err
	}
	item.EndsAt, err = time.Parse(time.RFC3339, endsAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func storeBusinessStateAt(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64, timezone string, now time.Time) (storeBusinessState, error) {
	periods, err := loadBusinessPeriods(ctx, queryer, tenantID, storeID)
	if err != nil {
		return storeBusinessState{}, err
	}
	override, err := loadCurrentBusinessOverride(ctx, queryer, tenantID, storeID, now)
	if err != nil {
		return storeBusinessState{}, err
	}
	return evaluateStoreBusinessState(now, timezone, periods, override)
}

func (s *Server) currentStoreBusinessState(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64) (storeBusinessState, error) {
	var timezone string
	if err := queryer.QueryRowContext(ctx, "SELECT timezone FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL", storeID, tenantID).Scan(&timezone); err != nil {
		return storeBusinessState{}, err
	}
	return storeBusinessStateAt(ctx, queryer, tenantID, storeID, timezone, time.Now())
}

func (s *Server) ensureStoreAcceptingOrders(ctx context.Context, tx *sql.Tx, tenantID, storeID int64) (storeBusinessState, error) {
	var timezone string
	if err := tx.QueryRowContext(ctx, "SELECT timezone FROM stores WHERE id=? AND tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE", storeID, tenantID).Scan(&timezone); err != nil {
		return storeBusinessState{}, err
	}
	state, err := storeBusinessStateAt(ctx, tx, tenantID, storeID, timezone, time.Now())
	if err != nil {
		return state, err
	}
	if !state.Open {
		return state, errStoreClosed
	}
	return state, nil
}

func allocatePickupCode(ctx context.Context, tx *sql.Tx, tenantID, storeID int64, businessDate string) (int64, string, error) {
	if _, err := time.Parse("2006-01-02", businessDate); err != nil {
		return 0, "", errors.New("invalid business date")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO order_pickup_sequences(tenant_id,store_id,business_date,last_value)
		VALUES(?,?,?,1) ON DUPLICATE KEY UPDATE last_value=LAST_INSERT_ID(last_value+1)`, tenantID, storeID, businessDate)
	if err != nil {
		return 0, "", err
	}
	// LastInsertId is the auto-increment row id on the INSERT path, not the
	// business sequence. The upsert keeps this row locked until the surrounding
	// order transaction commits, so reading last_value here is concurrency-safe
	// for both the first and subsequent orders of a business day.
	var sequence int64
	if err = tx.QueryRowContext(ctx, `SELECT last_value FROM order_pickup_sequences
		WHERE tenant_id=? AND store_id=? AND business_date=?`, tenantID, storeID, businessDate).Scan(&sequence); err != nil {
		return 0, "", err
	}
	if sequence <= 0 {
		return 0, "", errors.New("invalid pickup sequence")
	}
	return sequence, fmt.Sprintf("%04d", sequence), nil
}

func businessStateView(state storeBusinessState) map[string]any {
	nextOpenAt := ""
	if state.NextOpenAt != nil {
		nextOpenAt = state.NextOpenAt.Format(time.RFC3339)
	}
	return map[string]any{
		"businessStatus":        map[bool]string{true: "OPEN", false: "CLOSED"}[state.Open],
		"businessStatusReason":  state.Reason,
		"businessStatusMessage": state.Message,
		"timezone":              state.Timezone,
		"businessDate":          state.BusinessDate,
		"nextOpenAt":            nextOpenAt,
		"acceptingOrders":       state.Open,
	}
}

func (s *Server) getStoreBusinessHours(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var timezone string
	if err = s.DB.QueryRowContext(r.Context(), "SELECT timezone FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL", storeID, actor.TenantID).Scan(&timezone); err != nil {
		handleSQLError(w, err)
		return
	}
	periods, err := loadBusinessPeriods(r.Context(), s.DB, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	days := make([]weeklyBusinessDay, 7)
	for index := range days {
		days[index] = weeklyBusinessDay{Weekday: index + 1, Periods: []weeklyBusinessPeriod{}}
	}
	for _, period := range periods {
		days[period.Weekday-1].Periods = append(days[period.Weekday-1].Periods, period)
	}
	state, err := storeBusinessStateAt(r.Context(), s.DB, actor.TenantID, storeID, timezone, time.Now())
	if err != nil {
		handleSQLError(w, err)
		return
	}
	view := businessStateView(state)
	view["storeId"] = storeID
	view["weeklySchedule"] = days
	if state.Override != nil {
		view["temporaryOverride"] = map[string]any{"id": state.Override.ID, "status": state.Override.Kind, "startsAt": state.Override.StartsAt.Format(time.RFC3339), "endsAt": state.Override.EndsAt.Format(time.RFC3339), "reason": state.Override.Reason}
	}
	writeData(w, http.StatusOK, view)
}

type businessHoursInput struct {
	Timezone       string              `json:"timezone"`
	WeeklySchedule []weeklyBusinessDay `json:"weeklySchedule"`
}

func normalizeBusinessSchedule(input businessHoursInput) (string, []weeklyBusinessPeriod, error) {
	timezone := strings.TrimSpace(input.Timezone)
	if timezone == "" {
		timezone = defaultStoreTimezone
	}
	if len(timezone) > 64 {
		return "", nil, errors.New("timezone must not exceed 64 characters")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return "", nil, errors.New("timezone must be a valid IANA timezone")
	}
	seenDays := map[int]bool{}
	occupied := make([]bool, 7*1440)
	periods := []weeklyBusinessPeriod{}
	for _, day := range input.WeeklySchedule {
		if day.Weekday < 1 || day.Weekday > 7 || seenDays[day.Weekday] {
			return "", nil, errors.New("weekday must be unique and between 1 and 7")
		}
		seenDays[day.Weekday] = true
		if len(day.Periods) > 6 {
			return "", nil, errors.New("a day supports at most 6 business periods")
		}
		for index, raw := range day.Periods {
			start, err := clockMinute(raw.Start, false)
			if err != nil {
				return "", nil, err
			}
			end, err := clockMinute(raw.End, true)
			if err != nil {
				return "", nil, err
			}
			duration := end - start
			if duration <= 0 {
				duration += 1440
			}
			base := (day.Weekday-1)*1440 + start
			for minute := 0; minute < duration; minute++ {
				position := (base + minute) % len(occupied)
				if occupied[position] {
					return "", nil, errors.New("business periods must not overlap, including overnight periods")
				}
				occupied[position] = true
			}
			periods = append(periods, weeklyBusinessPeriod{Weekday: day.Weekday, Start: minuteClock(start), End: minuteClock(end), StartMinute: start, EndMinute: end, ID: int64(index)})
		}
	}
	sort.SliceStable(periods, func(i, j int) bool {
		if periods[i].Weekday == periods[j].Weekday {
			return periods[i].StartMinute < periods[j].StartMinute
		}
		return periods[i].Weekday < periods[j].Weekday
	})
	return timezone, periods, nil
}

func (s *Server) updateStoreBusinessHours(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input businessHoursInput
	if !decodeJSON(w, r, &input) {
		return
	}
	timezone, periods, err := normalizeBusinessSchedule(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var lockedID int64
	if err = tx.QueryRowContext(r.Context(), "SELECT id FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", storeID, actor.TenantID).Scan(&lockedID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM store_business_periods WHERE tenant_id=? AND store_id=?", actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	for index, period := range periods {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO store_business_periods(tenant_id,store_id,weekday,start_minute,end_minute,sort_order,status) VALUES(?,?,?,?,?,?,'ACTIVE')`, actor.TenantID, storeID, period.Weekday, period.StartMinute, period.EndMinute, index); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	legacyHours := ""
	if len(periods) > 0 {
		legacyHours = periods[0].Start + "-" + periods[0].End
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE stores SET timezone=?,business_hours=? WHERE id=? AND tenant_id=?", timezone, legacyHours, storeID, actor.TenantID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "store.business_hours.update", "store", int64String(storeID), input, r)
	s.getStoreBusinessHours(w, r)
}

type businessOverrideInput struct {
	Status   string `json:"status"`
	StartsAt string `json:"startsAt"`
	EndsAt   string `json:"endsAt"`
	Reason   string `json:"reason"`
}

func (s *Server) updateStoreBusinessOverride(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input businessOverrideInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Status != "NONE" && input.Status != "OPEN" && input.Status != "CLOSED" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be OPEN, CLOSED or NONE")
		return
	}
	if len([]rune(input.Reason)) > 255 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "reason must not exceed 255 characters")
		return
	}
	now := time.Now().UTC()
	startsAt := now
	var err error
	if strings.TrimSpace(input.StartsAt) != "" {
		startsAt, err = time.Parse(time.RFC3339, input.StartsAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "startsAt must be RFC3339")
			return
		}
	}
	var endsAt time.Time
	if input.Status != "NONE" {
		endsAt, err = time.Parse(time.RFC3339, input.EndsAt)
		if err != nil || !endsAt.After(startsAt) || endsAt.Sub(startsAt) > 31*24*time.Hour {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "endsAt must be after startsAt and within 31 days")
			return
		}
	}
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var lockedID int64
	if err = tx.QueryRowContext(r.Context(), "SELECT id FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", storeID, actor.TenantID).Scan(&lockedID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE store_business_overrides SET status='CANCELLED' WHERE tenant_id=? AND store_id=? AND status='ACTIVE'", actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if input.Status != "NONE" {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO store_business_overrides(tenant_id,store_id,override_type,starts_at,ends_at,reason,status,created_by) VALUES(?,?,?,?,?,?,'ACTIVE',?)`, actor.TenantID, storeID, input.Status, startsAt.UTC().Format("2006-01-02 15:04:05.000"), endsAt.UTC().Format("2006-01-02 15:04:05.000"), input.Reason, actor.UserID); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "store.business_override.update", "store", int64String(storeID), input, r)
	s.getStoreBusinessHours(w, r)
}
