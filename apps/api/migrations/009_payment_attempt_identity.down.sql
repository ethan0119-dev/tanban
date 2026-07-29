ALTER TABLE payment_transactions
  DROP KEY uk_payment_provider_request,
  DROP COLUMN provider_request_no,
  ADD UNIQUE KEY uk_payment_order (tenant_id, order_id);
