ALTER TABLE customer_coupons
  ADD COLUMN order_id BIGINT UNSIGNED NULL AFTER campaign_id,
  ADD KEY idx_customer_coupons_order (tenant_id, order_id, status);
