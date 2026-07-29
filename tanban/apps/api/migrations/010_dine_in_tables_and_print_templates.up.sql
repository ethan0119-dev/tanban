CREATE TABLE IF NOT EXISTS table_areas (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(80) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_table_areas_store (tenant_id, store_id, status, sort_order),
  CONSTRAINT fk_table_areas_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS table_codes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  area_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(80) NOT NULL,
  table_code VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  public_scene VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
  capacity INT NOT NULL DEFAULT 1,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_table_codes_scene (public_scene),
  UNIQUE KEY uk_table_codes_store_code (tenant_id, store_id, table_code),
  KEY idx_table_codes_area (tenant_id, store_id, area_id, status, sort_order),
  CONSTRAINT fk_table_codes_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_table_codes_area FOREIGN KEY (area_id) REFERENCES table_areas(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE orders
  ADD COLUMN order_type VARCHAR(24) NOT NULL DEFAULT 'TAKEOUT' AFTER fulfillment_type;

ALTER TABLE orders
  ADD COLUMN table_id BIGINT UNSIGNED NULL AFTER order_type;

ALTER TABLE orders
  ADD COLUMN table_area_name_snapshot VARCHAR(80) NOT NULL DEFAULT '' AFTER table_id;

ALTER TABLE orders
  ADD COLUMN table_name_snapshot VARCHAR(80) NOT NULL DEFAULT '' AFTER table_area_name_snapshot;

ALTER TABLE orders
  ADD COLUMN table_public_id_snapshot VARCHAR(32) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER table_name_snapshot;

ALTER TABLE orders
  ADD COLUMN table_code_snapshot VARCHAR(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER table_public_id_snapshot;

ALTER TABLE orders
  ADD KEY idx_orders_type_created (tenant_id, store_id, order_type, created_at);

ALTER TABLE orders
  ADD KEY idx_orders_table (tenant_id, store_id, table_id, created_at);

UPDATE orders
SET order_type=CASE
  WHEN fulfillment_type='DINE_IN' THEN 'DINE_IN'
  WHEN fulfillment_type='DELIVERY' THEN 'DELIVERY'
  ELSE 'TAKEOUT'
END;

CREATE TABLE IF NOT EXISTS print_templates (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  business_type VARCHAR(24) NOT NULL,
  template_type VARCHAR(24) NOT NULL DEFAULT 'RECEIPT',
  name VARCHAR(100) NOT NULL,
  content_text TEXT NOT NULL,
  trigger_event VARCHAR(32) NOT NULL DEFAULT 'PAYMENT_SUCCESS',
  copies INT NOT NULL DEFAULT 1,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_print_templates_scope (tenant_id, store_id, business_type, template_type),
  KEY idx_print_templates_store (tenant_id, store_id, status, business_type),
  CONSTRAINT fk_print_templates_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE printer_devices
  ADD COLUMN output_type VARCHAR(24) NOT NULL DEFAULT 'RECEIPT' AFTER print_trigger;

INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)
SELECT tenant_id,id,'DINE_IN','RECEIPT','店内小票','【店内】 {{table_area}} {{table_name}}\n订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n备注：{{remark}}',default_print_trigger,1,'ACTIVE'
FROM stores WHERE deleted_at IS NULL;

INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)
SELECT tenant_id,id,'DINE_IN','LABEL','店内标签','【店内】 {{table_name}} #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}',default_print_trigger,1,'ACTIVE'
FROM stores WHERE deleted_at IS NULL;

INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)
SELECT tenant_id,id,'TAKEOUT','RECEIPT','自提小票','【自提】\n订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n备注：{{remark}}',default_print_trigger,1,'ACTIVE'
FROM stores WHERE deleted_at IS NULL;

INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)
SELECT tenant_id,id,'TAKEOUT','LABEL','自提标签','【自提】 #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}',default_print_trigger,1,'ACTIVE'
FROM stores WHERE deleted_at IS NULL;

INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)
SELECT tenant_id,id,'DELIVERY','RECEIPT','外卖小票','【外卖】\n订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n备注：{{remark}}',default_print_trigger,1,'ACTIVE'
FROM stores WHERE deleted_at IS NULL;

INSERT IGNORE INTO print_templates(tenant_id,store_id,business_type,template_type,name,content_text,trigger_event,copies,status)
SELECT tenant_id,id,'DELIVERY','LABEL','外卖标签','【外卖】 #{{pickup_no}}\n{{product_name}} {{sku_name}}\n{{options}}\n{{modifiers}}\n{{item_remark}}',default_print_trigger,1,'ACTIVE'
FROM stores WHERE deleted_at IS NULL;
