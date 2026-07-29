package database

import (
	"os"
	"strings"
	"testing"
)

func TestCatalogSalesChannelsMigration(t *testing.T) {
	body, err := os.ReadFile("../../migrations/016_catalog_sales_channels.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, fragment := range []string{
		"ALTER TABLE categories",
		"in_store_enabled TINYINT(1) NOT NULL DEFAULT 1",
		"delivery_enabled TINYINT(1) NOT NULL DEFAULT 0",
		"ALTER TABLE products",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration is missing %q", fragment)
		}
	}
}
