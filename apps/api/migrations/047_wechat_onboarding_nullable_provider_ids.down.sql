ALTER TABLE wechat_pay_onboarding_applications
  DROP INDEX uk_wechat_onboarding_business_code,
  DROP INDEX uk_wechat_onboarding_applyment_id;

UPDATE wechat_pay_onboarding_applications
SET business_code = CONCAT('ROLLBACK-DRAFT-', tenant_id)
WHERE business_code IS NULL;

UPDATE wechat_pay_onboarding_applications
SET wechat_applyment_id = CONCAT('ROLLBACK-DRAFT-', tenant_id)
WHERE wechat_applyment_id IS NULL;

ALTER TABLE wechat_pay_onboarding_applications
  MODIFY COLUMN business_code VARCHAR(64) NOT NULL DEFAULT '',
  MODIFY COLUMN wechat_applyment_id VARCHAR(64) NOT NULL DEFAULT '',
  ADD UNIQUE KEY uk_wechat_onboarding_business_code (business_code),
  ADD UNIQUE KEY uk_wechat_onboarding_applyment_id (wechat_applyment_id);
