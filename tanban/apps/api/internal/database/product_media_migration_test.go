package database

import (
	"os"
	"strings"
	"testing"
)

func TestProductMediaMigrationHasTenantScopedGroupsAndImageBackfill(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/015_product_media_library.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS media_asset_groups",
		"ADD COLUMN group_id BIGINT UNSIGNED NULL",
		"MODIFY COLUMN image_url VARCHAR(1024)",
		"idx_order_items_product (tenant_id, product_id, order_id)",
		"idx_media_assets_group (tenant_id, store_id, group_id",
		"CREATE TABLE IF NOT EXISTS product_images",
		"idx_product_images_product (tenant_id, store_id, product_id",
		"CONSTRAINT fk_product_images_asset",
		"INSERT INTO product_images(tenant_id,store_id,product_id,url,is_primary,sort_order)",
		"WHERE image_url<>'' AND deleted_at IS NULL",
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("migration missing %q", expected)
		}
	}
}
