ALTER TABLE categories
  ADD COLUMN in_store_enabled TINYINT(1) NOT NULL DEFAULT 1 AFTER sort_order,
  ADD COLUMN delivery_enabled TINYINT(1) NOT NULL DEFAULT 0 AFTER in_store_enabled;

ALTER TABLE products
  ADD COLUMN in_store_enabled TINYINT(1) NOT NULL DEFAULT 1 AFTER recommended,
  ADD COLUMN delivery_enabled TINYINT(1) NOT NULL DEFAULT 0 AFTER in_store_enabled;
