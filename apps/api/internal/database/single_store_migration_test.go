package database

import (
	"os"
	"strings"
	"testing"
)

func TestSingleStoreTenantMigrationAddsUniqueBoundary(t *testing.T) {
	content, err := os.ReadFile("../../migrations/024_single_store_tenant.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	if !strings.Contains(sql, "unique key uk_stores_tenant (tenant_id)") {
		t.Fatal("single-store migration must enforce one store row per tenant")
	}
}
