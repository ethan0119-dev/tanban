ALTER TABLE tenants
  ADD COLUMN payment_onboarding_status VARCHAR(24) NOT NULL DEFAULT 'NOT_APPLIED' AFTER payment_sub_appid,
  ADD COLUMN payment_product_authorization_status VARCHAR(24) NOT NULL DEFAULT 'NOT_AUTHORIZED' AFTER payment_onboarding_status,
  ADD COLUMN payment_refund_authorized TINYINT(1) NOT NULL DEFAULT 0 AFTER payment_product_authorization_status;
