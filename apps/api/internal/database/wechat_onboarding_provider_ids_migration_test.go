package database

import (
	"os"
	"strings"
	"testing"
)

func TestWechatOnboardingProviderIDsAllowMultipleDrafts(t *testing.T) {
	body, err := os.ReadFile("../../migrations/047_wechat_onboarding_nullable_provider_ids.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, expected := range []string{
		"MODIFY COLUMN business_code VARCHAR(64) NULL DEFAULT NULL",
		"MODIFY COLUMN wechat_applyment_id VARCHAR(64) NULL DEFAULT NULL",
		"SET business_code = NULL",
		"SET wechat_applyment_id = NULL",
		"ADD UNIQUE KEY uk_wechat_onboarding_business_code (business_code)",
		"ADD UNIQUE KEY uk_wechat_onboarding_applyment_id (wechat_applyment_id)",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("WeChat onboarding provider ID migration missing %q", expected)
		}
	}
}
