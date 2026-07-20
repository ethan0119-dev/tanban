package database

import (
	"os"
	"strings"
	"testing"
)

func TestPrintTemplateCopiesMigrationPreservesLegacyTemplatesAndAddsStructuredRoles(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../migrations/011_print_template_copies_layout.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, expected := range []string{
		"ADD COLUMN copy_role VARCHAR(24) NOT NULL DEFAULT 'MERCHANT'",
		"ADD COLUMN paper_width INT NOT NULL DEFAULT 58",
		"ADD COLUMN layout_json TEXT NULL",
		"WHEN template_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT'",
		"WHEN template_type='LABEL' AND content_text=CASE business_type",
		"WHEN template_type='RECEIPT' AND content_text=CASE business_type",
		"ELSE '{}'",
		`"headerStyle":"SIMPLE","fontSize":"LARGE"`,
		`"showPrices":false,"showPayment":false`,
		`"showPrices":true,"showPayment":true`,
		"UNIQUE KEY uk_print_templates_role (tenant_id, store_id, business_type, template_type, copy_role)",
		"ADD COLUMN template_id BIGINT UNSIGNED NULL",
		"ADD COLUMN copy_role VARCHAR(24) NOT NULL DEFAULT ''",
		"ADD KEY idx_print_jobs_template (tenant_id, store_id, template_id, copy_role)",
		"ADD COLUMN copy_roles VARCHAR(96) NULL",
		"WHEN output_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT'",
		"DROP TRIGGER IF EXISTS trg_print_templates_legacy_copy_role",
		"CREATE TRIGGER trg_print_templates_legacy_copy_role",
		"WHEN NEW.template_type='LABEL' AND NEW.copy_role='MERCHANT' THEN 'ITEM'",
		`"headerStyle":"PROMINENT"`,
		`"showStoreName":true`,
		`"showItemOptions":true`,
	} {
		if !strings.Contains(schema, expected) {
			t.Fatalf("migration missing %q", expected)
		}
	}
	if strings.Contains(schema, "MODIFY layout_json TEXT NOT NULL") {
		t.Fatal("layout_json must remain nullable for compatibility with legacy writers")
	}

	downBody, err := os.ReadFile("../../migrations/011_print_template_copies_layout.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	down := string(downBody)
	for _, expected := range []string{
		"DROP TRIGGER IF EXISTS trg_print_templates_legacy_copy_role",
		"DROP KEY uk_print_templates_role",
		"ADD UNIQUE KEY uk_print_templates_scope (tenant_id, store_id, business_type, template_type)",
		"DROP COLUMN template_id",
		"DROP COLUMN copy_roles",
		"DROP COLUMN layout_json",
		"DROP COLUMN copy_role",
	} {
		if !strings.Contains(down, expected) {
			t.Fatalf("down migration missing %q", expected)
		}
	}
}
