CREATE TABLE IF NOT EXISTS miniapp_notification_templates (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  channel_key VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  appid VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  scene VARCHAR(40) NOT NULL,
  template_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
  title VARCHAR(120) NOT NULL,
  page_path VARCHAR(255) NOT NULL DEFAULT '',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_miniapp_notification_template_scene (appid,scene),
  UNIQUE KEY uk_miniapp_notification_template_id (appid,template_id),
  KEY idx_miniapp_notification_template_channel (channel_key,enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO miniapp_notification_templates(channel_key,appid,scene,template_id,title,page_path,enabled)
VALUES
  ('tanban-public','wx087d633542ae8d0b','PICKUP_READY','sPKz9ZotFXeTAQz08giDX9dcarm1uBGp9BqdtE-uQH8','取餐提醒','pages/order-detail/index',1),
  ('tanban-public','wx087d633542ae8d0b','RECHARGE_SUCCESS','4Ft2cM2A8zyFFzn04v4TbLGDaggJxRVz_fQHuKtBCS4','会员充值成功通知','pages/recharge/index',1),
  ('tanban-public','wx087d633542ae8d0b','BALANCE_CONSUMED','gMUJbiXDqPKC0LHG3yGpSrALVOw9VFDNh0YUU_4tMOU','储值余额使用提醒','pages/order-detail/index',1)
ON DUPLICATE KEY UPDATE
  channel_key=VALUES(channel_key),
  template_id=VALUES(template_id),
  title=VALUES(title),
  page_path=VALUES(page_path),
  enabled=VALUES(enabled);

CREATE TABLE IF NOT EXISTS customer_subscription_results (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  channel_key VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  appid VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  scene VARCHAR(40) NOT NULL,
  template_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
  result VARCHAR(16) NOT NULL,
  request_id VARCHAR(80) NOT NULL,
  request_context VARCHAR(40) NOT NULL DEFAULT '',
  business_no VARCHAR(128) NOT NULL DEFAULT '',
  claimed_at DATETIME(3) NULL,
  requested_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_customer_subscription_request (tenant_id,customer_id,channel_key,request_id,scene),
  KEY idx_customer_subscription_available (tenant_id,customer_id,channel_key,scene,result,claimed_at,id),
  KEY idx_customer_subscription_onboarding (tenant_id,customer_id,channel_key,requested_at),
  CONSTRAINT fk_customer_subscription_customer FOREIGN KEY (tenant_id,customer_id) REFERENCES customers(tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS miniapp_notification_outbox (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  subscription_result_id BIGINT UNSIGNED NOT NULL,
  channel_key VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  appid VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  openid VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
  scene VARCHAR(40) NOT NULL,
  template_id VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
  business_type VARCHAR(40) NOT NULL,
  business_no VARCHAR(128) NOT NULL,
  page_path VARCHAR(255) NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  available_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_error VARCHAR(500) NOT NULL DEFAULT '',
  provider_response TEXT NULL,
  processed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_miniapp_notification_business (tenant_id,channel_key,scene,business_type,business_no),
  UNIQUE KEY uk_miniapp_notification_subscription (subscription_result_id),
  KEY idx_miniapp_notification_pending (status,available_at,id),
  KEY idx_miniapp_notification_customer (tenant_id,customer_id,created_at),
  CONSTRAINT fk_miniapp_notification_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_miniapp_notification_customer FOREIGN KEY (tenant_id,customer_id) REFERENCES customers(tenant_id,id),
  CONSTRAINT fk_miniapp_notification_subscription FOREIGN KEY (subscription_result_id) REFERENCES customer_subscription_results(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE customer_account_payment_intents
  ADD COLUMN source_miniapp_channel_key VARCHAR(64) COLLATE utf8mb4_bin NOT NULL DEFAULT 'tanban-public' AFTER customer_openid,
  ADD COLUMN source_miniapp_appid VARCHAR(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER source_miniapp_channel_key,
  ADD KEY idx_customer_account_payment_miniapp (tenant_id,source_miniapp_channel_key,created_at);
