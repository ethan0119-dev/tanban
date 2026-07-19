ALTER TABLE orders
  ADD COLUMN request_fingerprint CHAR(64) NOT NULL DEFAULT '' AFTER idempotency_key;

ALTER TABLE refunds
  ADD COLUMN request_fingerprint CHAR(64) NOT NULL DEFAULT '' AFTER idempotency_key;

-- Snapshot provider routing data with the durable payment intent. Reconciliation
-- must not pick up a different merchant binding after platform configuration
-- changes while a provider request is in flight.
ALTER TABLE payment_transactions
  ADD COLUMN merchant_no VARCHAR(64) NOT NULL DEFAULT '' AFTER provider,
  ADD COLUMN sub_appid VARCHAR(64) NOT NULL DEFAULT '' AFTER merchant_no,
  ADD COLUMN customer_openid VARCHAR(128) NOT NULL DEFAULT '' AFTER sub_appid;
