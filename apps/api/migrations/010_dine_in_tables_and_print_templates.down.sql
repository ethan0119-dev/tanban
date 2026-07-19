DROP TABLE IF EXISTS print_templates;

ALTER TABLE printer_devices
  DROP COLUMN output_type;

ALTER TABLE orders
  DROP KEY idx_orders_table;

ALTER TABLE orders
  DROP KEY idx_orders_type_created;

ALTER TABLE orders
  DROP COLUMN table_code_snapshot;

ALTER TABLE orders
  DROP COLUMN table_public_id_snapshot;

ALTER TABLE orders
  DROP COLUMN table_name_snapshot;

ALTER TABLE orders
  DROP COLUMN table_area_name_snapshot;

ALTER TABLE orders
  DROP COLUMN table_id;

ALTER TABLE orders
  DROP COLUMN order_type;

DROP TABLE IF EXISTS table_codes;
DROP TABLE IF EXISTS table_areas;
