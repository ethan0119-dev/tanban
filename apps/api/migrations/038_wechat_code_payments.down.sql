ALTER TABLE payment_transactions
  DROP INDEX idx_payment_provider_transaction,
  DROP COLUMN device_info,
  DROP COLUMN provider_transaction_no,
  DROP COLUMN payment_method;
