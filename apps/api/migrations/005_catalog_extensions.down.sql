ALTER TABLE order_items
  DROP COLUMN item_remark,
  DROP COLUMN configuration_json,
  DROP COLUMN modifier_price_cents,
  DROP COLUMN base_price_cents,
  DROP COLUMN product_type;

DROP TABLE IF EXISTS product_modifier_groups;
DROP TABLE IF EXISTS modifier_group_items;
DROP TABLE IF EXISTS modifier_groups;
DROP TABLE IF EXISTS modifier_items;
DROP TABLE IF EXISTS product_option_values;
DROP TABLE IF EXISTS product_option_groups;
DROP TABLE IF EXISTS product_resource_bindings;
DROP TABLE IF EXISTS catalog_resources;

ALTER TABLE products
  DROP COLUMN print_label_resource_id,
  DROP COLUMN sale_periods_json,
  DROP COLUMN channels_json,
  DROP COLUMN cashier_only,
  DROP COLUMN max_per_order,
  DROP COLUMN recommended,
  DROP COLUMN unit_resource_id,
  DROP COLUMN product_type;

ALTER TABLE categories
  DROP COLUMN sale_periods_json,
  DROP COLUMN channels_json,
  DROP COLUMN cashier_only,
  DROP COLUMN is_default,
  DROP COLUMN icon_url,
  DROP COLUMN category_type,
  DROP COLUMN parent_id;
