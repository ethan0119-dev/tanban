ALTER TABLE print_templates
  ADD COLUMN copy_role VARCHAR(24) NOT NULL DEFAULT 'MERCHANT' AFTER template_type;

ALTER TABLE print_templates
  ADD COLUMN paper_width INT NOT NULL DEFAULT 58 AFTER copies;

ALTER TABLE print_templates
  ADD COLUMN layout_json TEXT NULL AFTER paper_width;

UPDATE print_templates
SET copy_role=CASE WHEN template_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT' END,
    layout_json=CASE
      WHEN template_type='LABEL' AND content_text=CASE business_type
        WHEN 'DINE_IN' THEN '【店内】 {{table_name}} #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}'
        WHEN 'DELIVERY' THEN '【外卖】 #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}'
        ELSE '【自提】 #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}' END
        THEN '{"schemaVersion":1,"headerStyle":"SIMPLE","fontSize":"LARGE","showStoreName":false,"showOrderType":false,"showOrderNo":true,"showPickupNo":true,"showTable":true,"showItems":true,"showItemOptions":true,"showPrices":false,"showPayment":false,"showRemark":true,"showCustomer":false,"showAddress":false,"showQrCode":false,"customHeader":"","customFooter":""}'
      WHEN template_type='RECEIPT' AND content_text=CASE business_type
        WHEN 'DINE_IN' THEN '【店内】 {{table_area}} {{table_name}}\n订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n备注：{{remark}}'
        WHEN 'DELIVERY' THEN '【外卖】\n订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n备注：{{remark}}'
        ELSE '【自提】\n订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n备注：{{remark}}' END
        THEN '{"schemaVersion":1,"headerStyle":"PROMINENT","fontSize":"NORMAL","showStoreName":true,"showOrderType":true,"showOrderNo":true,"showPickupNo":true,"showTable":true,"showItems":true,"showItemOptions":true,"showPrices":true,"showPayment":true,"showRemark":true,"showCustomer":false,"showAddress":false,"showQrCode":false,"customHeader":"","customFooter":""}'
      ELSE '{}'
    END;

ALTER TABLE print_templates
  DROP KEY uk_print_templates_scope;

ALTER TABLE print_templates
  ADD UNIQUE KEY uk_print_templates_role (tenant_id, store_id, business_type, template_type, copy_role);

ALTER TABLE print_jobs
  ADD COLUMN template_id BIGINT UNSIGNED NULL AFTER printer_id;

ALTER TABLE print_jobs
  ADD COLUMN copy_role VARCHAR(24) NOT NULL DEFAULT '' AFTER template_id;

ALTER TABLE print_jobs
  ADD COLUMN paper_width INT NOT NULL DEFAULT 58 AFTER copy_role;

ALTER TABLE print_jobs
  ADD KEY idx_print_jobs_template (tenant_id, store_id, template_id, copy_role);

ALTER TABLE printer_devices
  ADD COLUMN copy_roles VARCHAR(96) NULL AFTER output_type;

UPDATE printer_devices
SET copy_roles=CASE WHEN output_type='LABEL' THEN 'ITEM' ELSE 'MERCHANT' END;

-- The previous API does not send copy_role. Keep its LABEL upsert mapped to
-- ITEM during the startup rollback window; RECEIPT continues to default to
-- MERCHANT. Additional disabled roles are seeded by the new API only after it
-- is serving, rather than inside this forward-only DDL migration.
DROP TRIGGER IF EXISTS trg_print_templates_legacy_copy_role;

CREATE TRIGGER trg_print_templates_legacy_copy_role
BEFORE INSERT ON print_templates
FOR EACH ROW
SET NEW.copy_role=CASE
  WHEN NEW.template_type='LABEL' AND NEW.copy_role='MERCHANT' THEN 'ITEM'
  ELSE NEW.copy_role
END;
