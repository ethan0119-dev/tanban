ALTER TABLE payment_transactions
  ADD COLUMN payment_method VARCHAR(32) NOT NULL DEFAULT '' AFTER provider,
  ADD COLUMN provider_transaction_no VARCHAR(128) NOT NULL DEFAULT '' AFTER provider_order_no,
  ADD COLUMN device_info VARCHAR(64) NOT NULL DEFAULT '' AFTER provider_transaction_no,
  ADD KEY idx_payment_provider_transaction (provider, provider_transaction_no);
