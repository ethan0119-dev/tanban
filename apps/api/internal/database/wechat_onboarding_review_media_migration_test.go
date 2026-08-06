package database

import (
	"os"
	"strings"
	"testing"
)

func TestWechatOnboardingReviewMediaMigrationEncryptsTenantScopedCopies(t *testing.T) {
	up, err := os.ReadFile("../../migrations/048_wechat_onboarding_review_media.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	content := string(up)
	for _, expected := range []string{
		"CREATE TABLE wechat_pay_onboarding_review_media",
		"tenant_id BIGINT UNSIGNED NOT NULL",
		"field_name VARCHAR(64) NOT NULL",
		"ciphertext LONGTEXT NOT NULL",
		"wechat_media_id VARCHAR(1024) NOT NULL",
		"UNIQUE KEY uk_wechat_onboarding_review_media (tenant_id, field_name, ordinal_no)",
		"FOREIGN KEY (tenant_id) REFERENCES tenants(id)",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("review-media migration missing %q", expected)
		}
	}

	down, err := os.ReadFile("../../migrations/048_wechat_onboarding_review_media.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS wechat_pay_onboarding_review_media") {
		t.Fatal("review-media down migration must remove the encrypted material table")
	}
}
