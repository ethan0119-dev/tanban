ALTER TABLE print_templates
  ADD COLUMN copy_role VARCHAR(24) NULL DEFAULT NULL AFTER template_type;

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

-- Legacy API writers omit copy_role. A generated compatibility key maps those
-- NULL writes to the historical single RECEIPT/LABEL roles without requiring
-- a database trigger or elevated MySQL privileges.
ALTER TABLE print_templates
  MODIFY COLUMN copy_role VARCHAR(24) NULL DEFAULT NULL;

ALTER TABLE print_templates
  ADD COLUMN copy_role_key VARCHAR(24)
    GENERATED ALWAYS AS (
      CASE
        WHEN copy_role IS NOT NULL THEN copy_role
        WHEN template_type='LABEL' THEN 'ITEM'
        ELSE 'MERCHANT'
      END
    ) STORED AFTER copy_role;

ALTER TABLE print_templates
  DROP KEY uk_print_templates_scope;

ALTER TABLE print_templates
  DROP KEY uk_print_templates_role;

ALTER TABLE print_templates
  ADD UNIQUE KEY uk_print_templates_role_v2 (tenant_id, store_id, business_type, template_type, copy_role_key);

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
