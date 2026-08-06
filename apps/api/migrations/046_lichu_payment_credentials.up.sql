ALTER TABLE tenants
  ADD COLUMN payment_terminal_id VARCHAR(32) NOT NULL DEFAULT '' AFTER payment_sub_appid,
  ADD COLUMN payment_access_token_cipher TEXT NOT NULL AFTER payment_terminal_id;
