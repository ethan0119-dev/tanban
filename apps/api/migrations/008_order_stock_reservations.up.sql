ALTER TABLE orders
  ADD COLUMN inventory_reserved TINYINT(1) NOT NULL DEFAULT 0 AFTER payment_status,
  ADD COLUMN stock_reserved_at DATETIME(3) NULL AFTER inventory_reserved,
  ADD KEY idx_orders_stock_reservation (status, payment_status, inventory_reserved, stock_reserved_at);

UPDATE orders
SET inventory_reserved=1,
    stock_reserved_at=created_at
WHERE status='PENDING_PAYMENT' AND payment_status='UNPAID';
