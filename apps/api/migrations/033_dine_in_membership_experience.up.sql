ALTER TABLE orders
  ADD COLUMN diner_count INT UNSIGNED NOT NULL DEFAULT 1 AFTER addition_count,
  ADD COLUMN member_id_snapshot BIGINT UNSIGNED NULL AFTER customer_id,
  ADD COLUMN member_level_id_snapshot BIGINT UNSIGNED NULL AFTER member_id_snapshot,
  ADD COLUMN member_level_name_snapshot VARCHAR(80) NOT NULL DEFAULT '' AFTER member_level_id_snapshot,
  ADD COLUMN member_discount_cents BIGINT NOT NULL DEFAULT 0 AFTER merchandise_subtotal_cents;

ALTER TABLE products
  ADD COLUMN member_discount_enabled TINYINT(1) NOT NULL DEFAULT 0 AFTER recommended;

ALTER TABLE order_items
  ADD COLUMN addition_sequence INT UNSIGNED NOT NULL DEFAULT 1 AFTER order_id,
  ADD COLUMN original_unit_price_cents BIGINT NOT NULL DEFAULT 0 AFTER modifier_price_cents,
  ADD COLUMN member_discount_cents BIGINT NOT NULL DEFAULT 0 AFTER original_unit_price_cents,
  ADD COLUMN member_level_id_snapshot BIGINT UNSIGNED NULL AFTER member_discount_cents,
  ADD COLUMN member_level_name_snapshot VARCHAR(80) NOT NULL DEFAULT '' AFTER member_level_id_snapshot;

UPDATE order_items
SET original_unit_price_cents=unit_price_cents
WHERE original_unit_price_cents=0;
