ALTER TABLE categories
  ADD COLUMN parent_id BIGINT UNSIGNED NULL AFTER store_id,
  ADD COLUMN category_type VARCHAR(24) NOT NULL DEFAULT 'NORMAL' AFTER name,
  ADD COLUMN icon_url VARCHAR(512) NOT NULL DEFAULT '' AFTER category_type,
  ADD COLUMN is_default TINYINT(1) NOT NULL DEFAULT 0 AFTER icon_url,
  ADD COLUMN cashier_only TINYINT(1) NOT NULL DEFAULT 0 AFTER is_default,
  ADD COLUMN channels_json TEXT NULL AFTER cashier_only,
  ADD COLUMN sale_periods_json TEXT NULL AFTER channels_json;

ALTER TABLE products
  ADD COLUMN product_type VARCHAR(24) NOT NULL DEFAULT 'NORMAL' AFTER category_id,
  ADD COLUMN unit_resource_id BIGINT UNSIGNED NULL AFTER product_type,
  ADD COLUMN recommended TINYINT(1) NOT NULL DEFAULT 0 AFTER sort_order,
  ADD COLUMN max_per_order INT NOT NULL DEFAULT 0 AFTER recommended,
  ADD COLUMN cashier_only TINYINT(1) NOT NULL DEFAULT 0 AFTER max_per_order,
  ADD COLUMN channels_json TEXT NULL AFTER cashier_only,
  ADD COLUMN sale_periods_json TEXT NULL AFTER channels_json,
  ADD COLUMN print_label_resource_id BIGINT UNSIGNED NULL AFTER sale_periods_json;

ALTER TABLE order_items
  ADD COLUMN product_type VARCHAR(24) NOT NULL DEFAULT 'NORMAL' AFTER sku_name,
  ADD COLUMN base_price_cents BIGINT NOT NULL DEFAULT 0 AFTER product_type,
  ADD COLUMN modifier_price_cents BIGINT NOT NULL DEFAULT 0 AFTER base_price_cents,
  ADD COLUMN configuration_json TEXT NULL AFTER attributes_json,
  ADD COLUMN item_remark VARCHAR(255) NOT NULL DEFAULT '' AFTER configuration_json;

UPDATE order_items
SET base_price_cents=unit_price_cents,
    configuration_json='{}'
WHERE configuration_json IS NULL;

CREATE TABLE IF NOT EXISTS catalog_resources (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  resource_type VARCHAR(40) NOT NULL,
  code VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(120) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  price_cents BIGINT NOT NULL DEFAULT 0,
  config_json TEXT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_catalog_resources_store_type (tenant_id, store_id, resource_type, status, sort_order),
  CONSTRAINT fk_catalog_resources_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS product_resource_bindings (
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  product_id BIGINT UNSIGNED NOT NULL,
  resource_id BIGINT UNSIGNED NOT NULL,
  binding_type VARCHAR(40) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (product_id, resource_id, binding_type),
  KEY idx_product_resource_tenant (tenant_id, store_id, binding_type),
  CONSTRAINT fk_product_resource_product FOREIGN KEY (product_id) REFERENCES products(id),
  CONSTRAINT fk_product_resource_resource FOREIGN KEY (resource_id) REFERENCES catalog_resources(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS product_option_groups (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  product_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(100) NOT NULL,
  kind VARCHAR(24) NOT NULL DEFAULT 'ATTRIBUTE',
  selection_mode VARCHAR(16) NOT NULL DEFAULT 'SINGLE',
  min_select INT NOT NULL DEFAULT 0,
  max_select INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_product_options_product (tenant_id, store_id, product_id, status, sort_order),
  CONSTRAINT fk_product_options_product FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS product_option_values (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  group_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(100) NOT NULL,
  price_delta_cents BIGINT NOT NULL DEFAULT 0,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_product_option_values_group (tenant_id, store_id, group_id, status, sort_order),
  CONSTRAINT fk_product_option_values_group FOREIGN KEY (group_id) REFERENCES product_option_groups(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS modifier_items (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(100) NOT NULL,
  price_cents BIGINT NOT NULL DEFAULT 0,
  image_url VARCHAR(512) NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_modifier_items_store (tenant_id, store_id, status, sort_order),
  CONSTRAINT fk_modifier_items_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS modifier_groups (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(100) NOT NULL,
  min_select INT NOT NULL DEFAULT 0,
  max_select INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_modifier_groups_store (tenant_id, store_id, status, sort_order),
  CONSTRAINT fk_modifier_groups_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS modifier_group_items (
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  group_id BIGINT UNSIGNED NOT NULL,
  modifier_item_id BIGINT UNSIGNED NOT NULL,
  price_override_cents BIGINT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  sort_order INT NOT NULL DEFAULT 0,
  PRIMARY KEY (group_id, modifier_item_id),
  KEY idx_modifier_group_items_tenant (tenant_id, store_id),
  CONSTRAINT fk_modifier_group_items_group FOREIGN KEY (group_id) REFERENCES modifier_groups(id),
  CONSTRAINT fk_modifier_group_items_item FOREIGN KEY (modifier_item_id) REFERENCES modifier_items(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS product_modifier_groups (
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  product_id BIGINT UNSIGNED NOT NULL,
  modifier_group_id BIGINT UNSIGNED NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  PRIMARY KEY (product_id, modifier_group_id),
  KEY idx_product_modifier_tenant (tenant_id, store_id),
  CONSTRAINT fk_product_modifier_product FOREIGN KEY (product_id) REFERENCES products(id),
  CONSTRAINT fk_product_modifier_group FOREIGN KEY (modifier_group_id) REFERENCES modifier_groups(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
