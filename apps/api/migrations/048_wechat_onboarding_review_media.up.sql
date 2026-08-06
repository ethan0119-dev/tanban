CREATE TABLE wechat_pay_onboarding_review_media (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id BIGINT NOT NULL,
  field_name VARCHAR(64) NOT NULL,
  ordinal_no INT NOT NULL DEFAULT 0,
  content_type VARCHAR(64) NOT NULL,
  original_filename VARCHAR(255) NOT NULL DEFAULT '',
  ciphertext LONGTEXT NOT NULL,
  key_version VARCHAR(16) NOT NULL DEFAULT 'v1',
  wechat_media_id VARCHAR(1024) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_wechat_onboarding_review_media (tenant_id, field_name, ordinal_no),
  CONSTRAINT fk_wechat_onboarding_review_media_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);
