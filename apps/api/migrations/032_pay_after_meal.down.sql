DROP TABLE IF EXISTS order_addition_requests;

ALTER TABLE orders
  DROP COLUMN addition_count,
  DROP COLUMN settlement_mode_snapshot;
