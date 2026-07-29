ALTER TABLE payment_transactions
  DROP INDEX uk_payment_order,
  ADD COLUMN provider_request_no VARCHAR(64) NOT NULL DEFAULT '' AFTER customer_openid;

UPDATE payment_transactions p
JOIN orders o ON o.id=p.order_id AND o.tenant_id=p.tenant_id
SET p.provider_request_no=o.order_no
WHERE p.provider_request_no='';

ALTER TABLE payment_transactions
  ADD UNIQUE KEY uk_payment_provider_request (provider, provider_request_no);
