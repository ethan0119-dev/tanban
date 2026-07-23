package database

import (
	"os"
	"strings"
	"testing"
)

func TestPrinterLabelDimensionsMigrationAddsAndBackfillsT271USize(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/025_printer_label_dimensions.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{
		"ADD COLUMN label_width_mm INT NULL",
		"ADD COLUMN label_height_mm INT NULL",
		"SET label_width_mm=40,label_height_mm=30",
		"IN ('XP-T271U','T271U')",
		"AND (label_width_mm IS NULL OR label_height_mm IS NULL)",
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("migration missing %q", expected)
		}
	}

	downBody, err := os.ReadFile("../../migrations/025_printer_label_dimensions.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	down := string(downBody)
	if !strings.Contains(down, "DROP COLUMN label_height_mm") || !strings.Contains(down, "DROP COLUMN label_width_mm") {
		t.Fatal("down migration must remove both label dimension columns")
	}
}
