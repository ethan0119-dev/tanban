package database

import (
	"os"
	"strings"
	"testing"
)

func TestTenantCashierAvailabilityMigration(t *testing.T) {
	up, err := os.ReadFile("../../migrations/042_tenant_cashier_availability.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	content := string(up)
	for _, expected := range []string{
		"ADD COLUMN cashier_enabled TINYINT(1) NOT NULL DEFAULT 1",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("cashier availability migration missing %q", expected)
		}
	}

	down, err := os.ReadFile("../../migrations/042_tenant_cashier_availability.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "DROP COLUMN cashier_enabled") {
		t.Fatal("cashier availability down migration must drop cashier_enabled")
	}
}
