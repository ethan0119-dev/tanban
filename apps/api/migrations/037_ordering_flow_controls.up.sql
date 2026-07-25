ALTER TABLE store_operation_settings
  ADD COLUMN table_scan_return_home TINYINT(1) NOT NULL DEFAULT 1 AFTER ordering_mode,
  ADD COLUMN pay_before_clear_mode VARCHAR(32) NOT NULL DEFAULT 'AFTER_ORDER_COMPLETION' AFTER table_scan_return_home,
  ADD COLUMN pay_after_online_payment_enabled TINYINT(1) NOT NULL DEFAULT 1 AFTER pay_before_clear_mode;

UPDATE print_templates
SET trigger_event='PAYMENT_SUCCESS',updated_at=NOW(3)
WHERE business_type IN ('TAKEOUT','DELIVERY') AND deleted_at IS NULL;
