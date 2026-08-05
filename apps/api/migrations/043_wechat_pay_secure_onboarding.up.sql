ALTER TABLE wechat_pay_onboarding_applications
  ADD COLUMN sensitive_ciphertext LONGTEXT NULL AFTER business_material_ready,
  ADD COLUMN sensitive_key_version VARCHAR(16) NOT NULL DEFAULT '' AFTER sensitive_ciphertext,
  ADD COLUMN business_code VARCHAR(64) NOT NULL DEFAULT '' AFTER sensitive_key_version,
  ADD COLUMN wechat_applyment_id VARCHAR(64) NOT NULL DEFAULT '' AFTER business_code,
  ADD COLUMN wechat_applyment_state VARCHAR(64) NOT NULL DEFAULT '' AFTER wechat_applyment_id,
  ADD COLUMN wechat_state_message VARCHAR(1000) NOT NULL DEFAULT '' AFTER wechat_applyment_state,
  ADD COLUMN sign_url VARCHAR(1000) NOT NULL DEFAULT '' AFTER wechat_state_message,
  ADD COLUMN provider_submitted_at DATETIME(3) NULL AFTER sign_url,
  ADD UNIQUE KEY uk_wechat_onboarding_business_code (business_code),
  ADD UNIQUE KEY uk_wechat_onboarding_applyment_id (wechat_applyment_id);
