ALTER TABLE payment_transactions
  DROP COLUMN customer_openid,
  DROP COLUMN sub_appid,
  DROP COLUMN merchant_no;

ALTER TABLE refunds DROP COLUMN request_fingerprint;
ALTER TABLE orders DROP COLUMN request_fingerprint;
