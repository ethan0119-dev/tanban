package database

import (
	"os"
	"strings"
	"testing"
)

func TestPrintOutboxMigrationHasDedupeRetryAndOperationalIndexes(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/012_print_outbox.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS print_outbox",
		"event_type VARCHAR(32) NOT NULL",
		"dedupe_key VARCHAR(160)",
		"status VARCHAR(24) NOT NULL DEFAULT 'PENDING'",
		"attempts INT NOT NULL DEFAULT 0",
		"available_at DATETIME(3)",
		"last_error VARCHAR(500)",
		"processed_at DATETIME(3) NULL",
		"UNIQUE KEY uk_print_outbox_fact (tenant_id, event_type, dedupe_key)",
		"KEY idx_print_outbox_pending (status, available_at, id)",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("migration missing %q", expected)
		}
	}

	downBody, err := os.ReadFile("../../migrations/012_print_outbox.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(downBody), "DROP TABLE IF EXISTS print_outbox") {
		t.Fatal("down migration must remove print_outbox")
	}
}
