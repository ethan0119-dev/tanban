ALTER TABLE refunds
  DROP KEY uk_refunds_idempotency,
  DROP COLUMN last_error,
  DROP COLUMN idempotency_key;

ALTER TABLE payment_transactions
  DROP KEY uk_payment_order;

ALTER TABLE orders
  DROP COLUMN fulfillment_type;
