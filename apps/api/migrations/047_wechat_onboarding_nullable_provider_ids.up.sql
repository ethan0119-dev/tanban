ALTER TABLE wechat_pay_onboarding_applications
  DROP INDEX uk_wechat_onboarding_business_code,
  DROP INDEX uk_wechat_onboarding_applyment_id;

ALTER TABLE wechat_pay_onboarding_applications
  MODIFY COLUMN business_code VARCHAR(64) NULL DEFAULT NULL,
  MODIFY COLUMN wechat_applyment_id VARCHAR(64) NULL DEFAULT NULL;

UPDATE wechat_pay_onboarding_applications
SET business_code = NULL
WHERE business_code = '';

UPDATE wechat_pay_onboarding_applications
SET wechat_applyment_id = NULL
WHERE wechat_applyment_id = '';

ALTER TABLE wechat_pay_onboarding_applications
  ADD UNIQUE KEY uk_wechat_onboarding_business_code (business_code),
  ADD UNIQUE KEY uk_wechat_onboarding_applyment_id (wechat_applyment_id);
