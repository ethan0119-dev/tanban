DROP TABLE IF EXISTS product_images;

ALTER TABLE order_items
  DROP KEY idx_order_items_product;

UPDATE products SET image_url=LEFT(image_url,512) WHERE CHAR_LENGTH(image_url)>512;
ALTER TABLE products
  MODIFY COLUMN image_url VARCHAR(512) NOT NULL DEFAULT '';

ALTER TABLE media_assets
  DROP FOREIGN KEY fk_media_assets_group,
  DROP KEY idx_media_assets_group,
  DROP COLUMN group_id;

DROP TABLE IF EXISTS media_asset_groups;
