ALTER TABLE customer_coupons
  DROP KEY idx_customer_coupons_order,
  DROP COLUMN order_id;
