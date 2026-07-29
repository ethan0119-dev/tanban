ALTER TABLE payment_transactions
  ADD UNIQUE KEY uk_payment_order (tenant_id, order_id);

ALTER TABLE orders
  ADD COLUMN fulfillment_type VARCHAR(24) NOT NULL DEFAULT 'PICKUP' AFTER source;

ALTER TABLE refunds
  ADD COLUMN idempotency_key VARCHAR(128) NULL AFTER refund_no;

ALTER TABLE refunds
  ADD COLUMN last_error VARCHAR(500) NOT NULL DEFAULT '' AFTER status;

ALTER TABLE refunds
  ADD UNIQUE KEY uk_refunds_idempotency (tenant_id, idempotency_key);
