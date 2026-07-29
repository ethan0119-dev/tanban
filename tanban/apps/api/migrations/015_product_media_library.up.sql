CREATE TABLE IF NOT EXISTS media_asset_groups (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(80) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_by BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_media_asset_groups_store (tenant_id, store_id, sort_order, id),
  CONSTRAINT fk_media_asset_groups_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE media_assets
  ADD COLUMN group_id BIGINT UNSIGNED NULL AFTER store_id,
  ADD KEY idx_media_assets_group (tenant_id, store_id, group_id, status, created_at),
  ADD CONSTRAINT fk_media_assets_group FOREIGN KEY (group_id) REFERENCES media_asset_groups(id);

ALTER TABLE products
  MODIFY COLUMN image_url VARCHAR(1024) NOT NULL DEFAULT '';

ALTER TABLE order_items
  ADD KEY idx_order_items_product (tenant_id, product_id, order_id);

CREATE TABLE IF NOT EXISTS product_images (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  product_id BIGINT UNSIGNED NOT NULL,
  media_asset_id BIGINT UNSIGNED NULL,
  url VARCHAR(1024) NOT NULL,
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_product_images_product (tenant_id, store_id, product_id, is_primary, sort_order, id),
  KEY idx_product_images_asset (media_asset_id),
  CONSTRAINT fk_product_images_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_product_images_product FOREIGN KEY (product_id) REFERENCES products(id),
  CONSTRAINT fk_product_images_asset FOREIGN KEY (media_asset_id) REFERENCES media_assets(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO product_images(tenant_id,store_id,product_id,url,is_primary,sort_order)
SELECT tenant_id,store_id,id,image_url,1,0
FROM products
WHERE image_url<>'' AND deleted_at IS NULL;
