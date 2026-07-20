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
