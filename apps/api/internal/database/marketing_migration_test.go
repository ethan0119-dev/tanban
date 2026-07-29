package database

import (
	"os"
	"strings"
	"testing"
)

func TestMarketingMigrationHasTenantSafeInventoryAndIdempotencyConstraints(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/013_marketing_applications.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS coupon_campaigns",
		"threshold_cents BIGINT NOT NULL DEFAULT 0",
		"discount_cents BIGINT NOT NULL",
		"issued_count BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"CREATE TABLE IF NOT EXISTS customer_coupons",
		"status VARCHAR(24) NOT NULL DEFAULT 'PROVISIONAL'",
		"UNIQUE KEY uk_customer_coupons_idempotency (tenant_id, idempotency_key)",
		"CREATE TABLE IF NOT EXISTS marketing_placements",
		"CREATE TABLE IF NOT EXISTS marketing_events",
		"request_fingerprint CHAR(64) COLLATE utf8mb4_bin NOT NULL",
		"UNIQUE KEY uk_marketing_events_idempotency (tenant_id, idempotency_key)",
		"CREATE TABLE IF NOT EXISTS lottery_campaigns",
		"CREATE TABLE IF NOT EXISTS lottery_prizes",
		"awarded_count BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"CREATE TABLE IF NOT EXISTS lottery_draws",
		"UNIQUE KEY uk_lottery_draws_idempotency (tenant_id, idempotency_key)",
		"KEY idx_lottery_draws_subject_day (tenant_id, campaign_id, subject_key_hash, business_date)",
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("marketing migration missing %q", expected)
		}
	}

	downBody, err := os.ReadFile("../../migrations/013_marketing_applications.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	down := string(downBody)
	for _, table := range []string{"lottery_draws", "lottery_prizes", "lottery_campaigns", "marketing_events", "marketing_placements", "customer_coupons", "coupon_campaigns"} {
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+table) {
			t.Fatalf("down migration does not remove %s", table)
		}
	}
}
