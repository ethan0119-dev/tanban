package database

import (
	"os"
	"strings"
	"testing"
)

func TestMiniAppSubscriptionNotificationMigrationContainsThreeConfiguredScenes(t *testing.T) {
	body, err := os.ReadFile("../../migrations/039_miniapp_subscription_notifications.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS miniapp_notification_templates",
		"CREATE TABLE IF NOT EXISTS customer_subscription_results",
		"CREATE TABLE IF NOT EXISTS miniapp_notification_outbox",
		"'PICKUP_READY','sPKz9ZotFXeTAQz08giDX9dcarm1uBGp9BqdtE-uQH8'",
		"'RECHARGE_SUCCESS','4Ft2cM2A8zyFFzn04v4TbLGDaggJxRVz_fQHuKtBCS4'",
		"'BALANCE_CONSUMED','gMUJbiXDqPKC0LHG3yGpSrALVOw9VFDNh0YUU_4tMOU'",
		"UNIQUE KEY uk_customer_subscription_request",
		"UNIQUE KEY uk_miniapp_notification_business",
		"ADD COLUMN source_miniapp_channel_key",
		"ADD COLUMN source_miniapp_appid",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("miniapp notification migration missing %q", expected)
		}
	}
}
