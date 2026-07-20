DELETE FROM print_templates
WHERE template_type='RECEIPT' AND copy_role IN ('CUSTOMER','KITCHEN');

DROP TRIGGER IF EXISTS trg_print_templates_legacy_copy_role;

ALTER TABLE printer_devices
  DROP COLUMN copy_roles;

ALTER TABLE print_templates
  DROP KEY uk_print_templates_role;

ALTER TABLE print_templates
  ADD UNIQUE KEY uk_print_templates_scope (tenant_id, store_id, business_type, template_type);

ALTER TABLE print_jobs
  DROP KEY idx_print_jobs_template;

ALTER TABLE print_jobs
  DROP COLUMN paper_width;

ALTER TABLE print_jobs
  DROP COLUMN copy_role;

ALTER TABLE print_jobs
  DROP COLUMN template_id;

ALTER TABLE print_templates
  DROP COLUMN layout_json;

ALTER TABLE print_templates
  DROP COLUMN paper_width;

ALTER TABLE print_templates
  DROP COLUMN copy_role;
