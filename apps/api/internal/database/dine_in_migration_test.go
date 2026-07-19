package database

import (
	"os"
	"strings"
	"testing"
)

func TestDineInMigrationHasTenantSafeTablesOrderSnapshotsAndPrintControls(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/010_dine_in_tables_and_print_templates.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS table_areas",
		"CREATE TABLE IF NOT EXISTS table_codes",
		"public_scene VARCHAR(32) COLLATE utf8mb4_bin",
		"UNIQUE KEY uk_table_codes_scene (public_scene)",
		"UNIQUE KEY uk_table_codes_store_code (tenant_id, store_id, table_code)",
		"ADD COLUMN order_type VARCHAR(24) NOT NULL DEFAULT 'TAKEOUT'",
		"ADD COLUMN table_id BIGINT UNSIGNED NULL",
		"ADD COLUMN table_public_id_snapshot VARCHAR(32) COLLATE utf8mb4_bin",
		"ADD COLUMN table_area_name_snapshot VARCHAR(80)",
		"ADD COLUMN table_name_snapshot VARCHAR(80)",
		"ADD COLUMN table_code_snapshot VARCHAR(64) COLLATE utf8mb4_bin",
		"trigger_event VARCHAR(32) NOT NULL DEFAULT 'PAYMENT_SUCCESS'",
		"copies INT NOT NULL DEFAULT 1",
		"ADD COLUMN output_type VARCHAR(24) NOT NULL DEFAULT 'RECEIPT'",
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("migration missing %q", expected)
		}
	}
}
