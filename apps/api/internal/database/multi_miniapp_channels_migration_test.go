package database

import (
	"os"
	"strings"
	"testing"
)

func TestMultiMiniAppMigrationKeepsCredentialsAndIdentitiesScoped(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/036_multi_miniapp_channels.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, required := range []string{
		"tenant_miniapp_channels",
		"dedicated_app_secret_cipher TEXT NOT NULL",
		"UNIQUE KEY uk_tenant_miniapp_channel_key (dedicated_channel_key)",
		"customer_wechat_identities",
		"UNIQUE KEY uk_customer_wechat_channel_openid (tenant_id,channel_key,openid)",
		"source_miniapp_channel_key",
		"source_miniapp_appid",
	} {
		if !strings.Contains(schema, required) {
			t.Fatalf("multi-miniapp migration is missing %q", required)
		}
	}
	if strings.Contains(schema, "dedicated_app_secret VARCHAR") {
		t.Fatal("miniapp AppSecret must never be stored as a plaintext column")
	}
}
