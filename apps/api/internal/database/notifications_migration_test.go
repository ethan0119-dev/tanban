package database

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformAnnouncementsMigrationContainsMerchantInboxSchema(t *testing.T) {
	body, err := os.ReadFile("../../migrations/019_platform_announcements.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS platform_announcements",
		"CREATE TABLE IF NOT EXISTS platform_announcement_targets",
		"CREATE TABLE IF NOT EXISTS merchant_notification_recipients",
		"CREATE TABLE IF NOT EXISTS merchant_notification_reads",
		"published_at DATETIME(3) NULL",
		"PRIMARY KEY (announcement_id, tenant_id)",
		"PRIMARY KEY (announcement_id, user_id)",
		"KEY idx_merchant_notification_recipients_tenant (tenant_id, announcement_id)",
		"KEY idx_merchant_notification_reads_tenant_user (tenant_id, user_id, read_at)",
		"FOREIGN KEY (announcement_id) REFERENCES platform_announcements(id) ON DELETE CASCADE",
		"FOREIGN KEY (tenant_id) REFERENCES tenants(id)",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("notification migration missing %q", expected)
		}
	}

	downBody, err := os.ReadFile("../../migrations/019_platform_announcements.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	downSQL := string(downBody)
	for _, table := range []string{
		"merchant_notification_reads",
		"merchant_notification_recipients",
		"platform_announcement_targets",
		"platform_announcements",
	} {
		if !strings.Contains(downSQL, "DROP TABLE IF EXISTS "+table) {
			t.Fatalf("notification down migration does not remove %s", table)
		}
	}
}
