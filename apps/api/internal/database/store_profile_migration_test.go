package database

import (
	"os"
	"strings"
	"testing"
)

func TestStoreProfileMigrationAddsReusableStoreInformation(t *testing.T) {
	body, err := os.ReadFile("../../migrations/022_store_profile.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS store_profiles",
		"visible_in_miniapp",
		"service_channels_json",
		"environment_image_urls_json",
		"food_safety_image_urls_json",
		"INSERT IGNORE INTO store_profiles",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("store profile migration missing %q", expected)
		}
	}
	downBody, err := os.ReadFile("../../migrations/022_store_profile.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(downBody), "DROP TABLE IF EXISTS store_profiles") {
		t.Fatal("store profile down migration must drop store_profiles")
	}
}
