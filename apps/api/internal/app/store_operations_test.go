package app

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func mustLocation(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(defaultStoreTimezone)
	if err != nil {
		t.Fatal(err)
	}
	return location
}

func TestClockMinuteSupportsEndOfDay(t *testing.T) {
	t.Parallel()
	if value, err := clockMinute("24:00", true); err != nil || value != 1440 {
		t.Fatalf("24:00=%d,%v; want 1440,nil", value, err)
	}
	if _, err := clockMinute("24:00", false); err == nil {
		t.Fatal("24:00 must not be accepted as a start time")
	}
}

func TestEqualBusinessPeriodEndpointsMeanFullDay(t *testing.T) {
	t.Parallel()
	location := mustLocation(t)
	now := time.Date(2026, time.July, 20, 13, 30, 0, 0, location) // Monday
	state, err := evaluateStoreBusinessState(now, defaultStoreTimezone, []weeklyBusinessPeriod{{Weekday: 1, StartMinute: 0, EndMinute: 0}}, nil)
	if err != nil || !state.Open || state.BusinessDate != "2026-07-20" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestOvernightBusinessPeriodUsesOpeningBusinessDate(t *testing.T) {
	t.Parallel()
	location := mustLocation(t)
	now := time.Date(2026, time.July, 21, 1, 15, 0, 0, location) // Tuesday, inside Monday 18:00-02:00
	state, err := evaluateStoreBusinessState(now, defaultStoreTimezone, []weeklyBusinessPeriod{{Weekday: 1, StartMinute: 18 * 60, EndMinute: 2 * 60}}, nil)
	if err != nil || !state.Open || state.BusinessDate != "2026-07-20" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestPeriodWindowUsesLocalWallClockAcrossDST(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, day := range []time.Time{
		time.Date(2026, time.March, 8, 12, 0, 0, 0, location),
		time.Date(2026, time.November, 1, 12, 0, 0, 0, location),
	} {
		start, end := periodWindow(day, weeklyBusinessPeriod{StartMinute: 9 * 60, EndMinute: 10 * 60})
		if start.Hour() != 9 || end.Hour() != 10 || start.Location() != location || end.Location() != location {
			t.Fatalf("business hours must stay on local wall clock across DST: start=%s end=%s", start, end)
		}
	}
}

func TestTemporaryClosedOverrideWinsAndResumesSchedule(t *testing.T) {
	t.Parallel()
	location := mustLocation(t)
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, location)
	override := &businessOverride{Kind: "CLOSED", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Reason: "设备维护"}
	state, err := evaluateStoreBusinessState(now, defaultStoreTimezone, []weeklyBusinessPeriod{{Weekday: 1, StartMinute: 0, EndMinute: 0}}, override)
	if err != nil || state.Open || state.Reason != "TEMPORARY_CLOSED" || state.NextOpenAt == nil || !state.NextOpenAt.Equal(override.EndsAt) {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestNormalizeBusinessScheduleRejectsCrossDayOverlap(t *testing.T) {
	t.Parallel()
	_, _, err := normalizeBusinessSchedule(businessHoursInput{Timezone: defaultStoreTimezone, WeeklySchedule: []weeklyBusinessDay{
		{Weekday: 1, Periods: []weeklyBusinessPeriod{{Start: "18:00", End: "02:00"}}},
		{Weekday: 2, Periods: []weeklyBusinessPeriod{{Start: "01:00", End: "03:00"}}},
	}})
	if err == nil {
		t.Fatal("overnight overlap must be rejected")
	}
}

func TestPrintUsesPersistedPickupCodeAndPlateSnapshot(t *testing.T) {
	t.Parallel()
	order := orderDTO{ID: 23, OrderType: orderTypeTakeout, BusinessDate: "2026-07-20", PickupCode: "0042", FastFoodPlate: &orderFastFoodPlateDTO{Name: "窗边", PlateCode: "P07"}}
	if code := printablePickupCode(order); code != "0042" {
		t.Fatalf("pickup code=%q", code)
	}
	content := renderOrderTemplate("{{pickup_no}} {{fast_food_plate_name}} {{fast_food_plate_code}}", order, "", "", false)
	if content != "0042 窗边 P07" {
		t.Fatalf("content=%q", content)
	}
}

func TestAllocatePickupCodeReadsBusinessSequenceInsteadOfInsertID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO order_pickup_sequences(tenant_id,store_id,business_date,last_value)
		VALUES(?,?,?,1) ON DUPLICATE KEY UPDATE last_value=LAST_INSERT_ID(last_value+1)`)).
		WithArgs(int64(3), int64(7), "2026-07-20").
		WillReturnResult(sqlmock.NewResult(9821, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT last_value FROM order_pickup_sequences
		WHERE tenant_id=? AND store_id=? AND business_date=?`)).
		WithArgs(int64(3), int64(7), "2026-07-20").
		WillReturnRows(sqlmock.NewRows([]string{"last_value"}).AddRow(1))

	sequence, code, err := allocatePickupCode(context.Background(), tx, 3, 7, "2026-07-20")
	if err != nil {
		t.Fatal(err)
	}
	if sequence != 1 || code != "0001" {
		t.Fatalf("pickup sequence must use last_value, not auto-increment insert id: sequence=%d code=%s", sequence, code)
	}
	mock.ExpectRollback()
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
