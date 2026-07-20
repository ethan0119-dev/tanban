ALTER TABLE orders
  DROP INDEX idx_orders_fast_food_plate,
  DROP INDEX uk_orders_pickup_sequence,
  DROP COLUMN fast_food_plate_code_snapshot,
  DROP COLUMN fast_food_plate_name_snapshot,
  DROP COLUMN fast_food_plate_public_id_snapshot,
  DROP COLUMN fast_food_plate_id,
  DROP COLUMN pickup_code,
  DROP COLUMN pickup_sequence,
  DROP COLUMN business_date;

DROP TABLE IF EXISTS order_pickup_sequences;
DROP TABLE IF EXISTS fast_food_plates;
DROP TABLE IF EXISTS store_business_overrides;
DROP TABLE IF EXISTS store_business_periods;

ALTER TABLE stores DROP COLUMN timezone;
