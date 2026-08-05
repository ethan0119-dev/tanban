package database

import (
	"os"
	"strings"
	"testing"
)

func TestProductionMediaURLDomainMigration(t *testing.T) {
	up, err := os.ReadFile("../../migrations/045_production_media_url_domain.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	content := string(up)
	for _, expected := range []string{
		"UPDATE media_assets",
		"UPDATE marketing_placements",
		"UPDATE product_images",
		"UPDATE products",
		"UPDATE store_decoration_versions",
		"UPDATE store_decorations",
		"UPDATE store_operation_settings",
		"UPDATE stores",
		"https://tbapi.666qwe.cn",
		"https://api.tanban.com.cn",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("production media URL migration missing %q", expected)
		}
	}
	if strings.Contains(content, "UPDATE audit_logs") {
		t.Fatal("production media URL migration must preserve immutable audit details")
	}

	down, err := os.ReadFile("../../migrations/045_production_media_url_domain.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "intentionally irreversible") {
		t.Fatal("down migration must document why the data rewrite cannot be reversed safely")
	}
}
