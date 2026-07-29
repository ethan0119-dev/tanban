package database

import (
	"os"
	"strings"
	"testing"
)

func TestAttributeLibraryMigrationKeepsProductOptionCompatibility(t *testing.T) {
	body, err := os.ReadFile("../../migrations/021_attribute_library.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS attribute_groups",
		"CREATE TABLE IF NOT EXISTS attribute_values",
		"ADD COLUMN attribute_group_id BIGINT UNSIGNED NULL",
		"ADD COLUMN attribute_value_id BIGINT UNSIGNED NULL",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("attribute library migration missing %q", expected)
		}
	}
	downBody, err := os.ReadFile("../../migrations/021_attribute_library.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"DROP COLUMN attribute_value_id", "DROP COLUMN attribute_group_id", "DROP TABLE IF EXISTS attribute_values", "DROP TABLE IF EXISTS attribute_groups"} {
		if !strings.Contains(string(downBody), expected) {
			t.Fatalf("attribute library down migration missing %q", expected)
		}
	}
}
