CREATE TABLE IF NOT EXISTS attribute_groups (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(100) NOT NULL,
  selection_mode VARCHAR(16) NOT NULL DEFAULT 'SINGLE',
  min_select INT NOT NULL DEFAULT 0,
  max_select INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_attribute_groups_store (tenant_id, store_id, status, sort_order, id),
  CONSTRAINT fk_attribute_groups_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS attribute_values (
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
  KEY idx_attribute_values_group (tenant_id, store_id, group_id, status, sort_order, id),
  CONSTRAINT fk_attribute_values_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_attribute_values_group FOREIGN KEY (group_id) REFERENCES attribute_groups(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE product_option_groups
  ADD COLUMN attribute_group_id BIGINT UNSIGNED NULL AFTER product_id,
  ADD KEY idx_product_option_attribute_group (tenant_id, store_id, attribute_group_id),
  ADD CONSTRAINT fk_product_option_attribute_group FOREIGN KEY (attribute_group_id) REFERENCES attribute_groups(id);

ALTER TABLE product_option_values
  ADD COLUMN attribute_value_id BIGINT UNSIGNED NULL AFTER group_id,
  ADD KEY idx_product_option_attribute_value (tenant_id, store_id, attribute_value_id),
  ADD CONSTRAINT fk_product_option_attribute_value FOREIGN KEY (attribute_value_id) REFERENCES attribute_values(id);
