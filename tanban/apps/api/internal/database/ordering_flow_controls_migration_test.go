package database

import (
	"os"
	"strings"
	"testing"
)

func TestOrderingFlowControlsMigrationAddsSettingsAndProtectsTakeoutPrinting(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/037_ordering_flow_controls.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{
		"ADD COLUMN table_scan_return_home",
		"ADD COLUMN pay_before_clear_mode",
		"ADD COLUMN pay_after_online_payment_enabled",
		"business_type IN ('TAKEOUT','DELIVERY')",
		"trigger_event='PAYMENT_SUCCESS'",
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("ordering flow migration missing %q", expected)
		}
	}
}
