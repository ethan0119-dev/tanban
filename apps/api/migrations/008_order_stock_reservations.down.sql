ALTER TABLE orders
  DROP KEY idx_orders_stock_reservation,
  DROP COLUMN stock_reserved_at,
  DROP COLUMN inventory_reserved;
