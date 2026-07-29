ALTER TABLE order_items
  DROP COLUMN member_level_name_snapshot,
  DROP COLUMN member_level_id_snapshot,
  DROP COLUMN member_discount_cents,
  DROP COLUMN original_unit_price_cents,
  DROP COLUMN addition_sequence;

ALTER TABLE products
  DROP COLUMN member_discount_enabled;

ALTER TABLE orders
  DROP COLUMN member_discount_cents,
  DROP COLUMN member_level_name_snapshot,
  DROP COLUMN member_level_id_snapshot,
  DROP COLUMN member_id_snapshot,
  DROP COLUMN diner_count;
