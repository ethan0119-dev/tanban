package database

import (
	"os"
	"strings"
	"testing"
)

func TestGlobalMerchantAccountsMigration(t *testing.T) {
	body, err := os.ReadFile("../../migrations/023_global_merchant_accounts.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS accounts",
		"UNIQUE KEY uk_accounts_username (username)",
		"CREATE TABLE IF NOT EXISTS tenant_memberships",
		"UNIQUE KEY uk_tenant_memberships_account_tenant (account_id, tenant_id)",
		"INSERT IGNORE INTO accounts",
		"INSERT IGNORE INTO tenant_memberships",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("global-account migration missing %q", expected)
		}
	}
}
