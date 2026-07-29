CREATE TABLE IF NOT EXISTS store_operation_settings (
  store_id BIGINT UNSIGNED NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  settlement_mode VARCHAR(24) NOT NULL DEFAULT 'PAY_BEFORE',
  ordering_mode VARCHAR(24) NOT NULL DEFAULT 'MULTI_PERSON',
  distance_check_enabled TINYINT(1) NOT NULL DEFAULT 0,
  distance_limit_m INT UNSIGNED NOT NULL DEFAULT 5000,
  store_latitude DECIMAL(10,7) NULL,
  store_longitude DECIMAL(10,7) NULL,
  require_customer_phone TINYINT(1) NOT NULL DEFAULT 0,
  allow_order_remark TINYINT(1) NOT NULL DEFAULT 1,
  allow_item_remark TINYINT(1) NOT NULL DEFAULT 1,
  order_reminder_enabled TINYINT(1) NOT NULL DEFAULT 1,
  order_reminder_interval_minutes INT UNSIGNED NOT NULL DEFAULT 5,
  takeaway_verification_enabled TINYINT(1) NOT NULL DEFAULT 0,
  reviews_enabled TINYINT(1) NOT NULL DEFAULT 0,
  customer_service_phone VARCHAR(32) NOT NULL DEFAULT '',
  customer_service_wechat VARCHAR(80) NOT NULL DEFAULT '',
  customer_service_qr_url VARCHAR(1024) NOT NULL DEFAULT '',
  privacy_policy_text MEDIUMTEXT NOT NULL,
  user_agreement_text MEDIUMTEXT NOT NULL,
  official_account_notify_enabled TINYINT(1) NOT NULL DEFAULT 0,
  official_account_events_json TEXT NOT NULL,
  notification_recipient_label VARCHAR(120) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (store_id),
  UNIQUE KEY uk_store_operation_settings_scope (tenant_id, store_id),
  CONSTRAINT fk_store_operation_settings_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO store_operation_settings(
  store_id,tenant_id,settlement_mode,ordering_mode,privacy_policy_text,user_agreement_text,official_account_events_json
)
SELECT id,tenant_id,'PAY_BEFORE','MULTI_PERSON','','',
  '["ORDER_PAID","REFUND_CREATED","PRINT_FAILED"]'
FROM stores WHERE deleted_at IS NULL;

ALTER TABLE tenants
  ADD COLUMN payment_fee_bps SMALLINT UNSIGNED NOT NULL DEFAULT 38 AFTER payment_sub_appid,
  ADD COLUMN payment_settlement_cycle VARCHAR(24) NOT NULL DEFAULT 'T1' AFTER payment_fee_bps;
