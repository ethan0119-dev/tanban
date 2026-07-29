ALTER TABLE orders
  ADD KEY idx_orders_tenant_store_paid (tenant_id, store_id, paid_at);

