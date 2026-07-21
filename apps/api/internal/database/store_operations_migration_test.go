package database

import (
	"os"
	"strings"
	"testing"
)

func TestStoreOperationsMigrationHasSchedulePlateSnapshotsAndSequence(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/014_store_operations.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{
		"ADD COLUMN timezone VARCHAR(64)",
		"CREATE TABLE IF NOT EXISTS store_business_periods",
		"CREATE TABLE IF NOT EXISTS store_business_overrides",
		"CREATE TABLE IF NOT EXISTS fast_food_plates",
		"UNIQUE KEY uk_fast_food_plates_scene (public_scene)",
		"CREATE TABLE IF NOT EXISTS order_pickup_sequences",
		"ADD COLUMN pickup_code VARCHAR(16)",
		"ADD COLUMN fast_food_plate_public_id_snapshot",
		"ADD UNIQUE KEY uk_orders_pickup_sequence",
		"WHEN SUBSTRING_INDEX(s.business_hours,'-',-1)='24:00' THEN 1440",
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("migration missing %q", expected)
		}
	}
}

func TestBeijingTimeMigrationNormalizesSchedulesAndLegacyOverrides(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/020_beijing_time_contract.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{
		"SET timezone='Asia/Shanghai'",
		"SET starts_at=DATE_ADD(starts_at, INTERVAL 8 HOUR)",
		"ends_at=DATE_ADD(ends_at, INTERVAL 8 HOUR)",
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("Beijing-time migration missing %q", expected)
		}
	}
}

func TestMissingBusinessPeriodMigrationBackfillsOnlyUnconfiguredStores(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/022_backfill_missing_business_periods.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{"INSERT INTO store_business_periods", "NOT EXISTS", "WHEN TRIM(SUBSTRING_INDEX(s.business_hours,'-',-1))='24:00' THEN 1440"} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("missing-period migration missing %q", expected)
		}
	}
}
