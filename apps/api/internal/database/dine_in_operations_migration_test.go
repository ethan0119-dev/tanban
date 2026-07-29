package database

import (
	"os"
	"strings"
	"testing"
)

func TestDineInOperationsMigrationKeepsOperationalRecordsAuditable(t *testing.T) {
	body, err := os.ReadFile("../../migrations/034_dine_in_operations.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS order_table_events",
		"CREATE TABLE IF NOT EXISTS order_settlement_parts",
		"UNIQUE KEY uk_order_settlement_idempotency",
		"CREATE TABLE IF NOT EXISTS order_return_requests",
		"CREATE TABLE IF NOT EXISTS offline_reconciliations",
		"UNIQUE KEY uk_offline_reconciliation_date",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("dine-in operations migration missing %q", fragment)
		}
	}
	if strings.Contains(strings.ToUpper(sql), "ALTER TABLE ORDERS DROP") {
		t.Fatal("production up migration must be expand-only")
	}
}
