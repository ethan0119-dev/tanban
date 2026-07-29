ALTER TABLE products
  DROP COLUMN delivery_enabled,
  DROP COLUMN in_store_enabled;

ALTER TABLE categories
  DROP COLUMN delivery_enabled,
  DROP COLUMN in_store_enabled;
