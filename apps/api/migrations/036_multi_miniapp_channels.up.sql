CREATE TABLE IF NOT EXISTS tenant_miniapp_channels (
  tenant_id BIGINT UNSIGNED NOT NULL,
  primary_mode VARCHAR(24) NOT NULL DEFAULT 'PUBLIC',
  public_enabled TINYINT(1) NOT NULL DEFAULT 1,
  dedicated_enabled TINYINT(1) NOT NULL DEFAULT 0,
  dedicated_display_name VARCHAR(120) NOT NULL DEFAULT '',
  dedicated_channel_key VARCHAR(64) COLLATE utf8mb4_bin NULL,
  dedicated_appid VARCHAR(64) COLLATE utf8mb4_bin NULL,
  dedicated_app_secret_cipher TEXT NOT NULL,
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (tenant_id),
  UNIQUE KEY uk_tenant_miniapp_channel_key (dedicated_channel_key),
  UNIQUE KEY uk_tenant_miniapp_appid (dedicated_appid),
  CONSTRAINT fk_tenant_miniapp_channel_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO tenant_miniapp_channels(
  tenant_id,primary_mode,public_enabled,dedicated_enabled,dedicated_app_secret_cipher
)
SELECT id,'PUBLIC',1,0,'' FROM tenants WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS customer_wechat_identities (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  channel_key VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  appid VARCHAR(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  openid VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
  unionid VARCHAR(128) COLLATE utf8mb4_bin NULL,
  last_login_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_customer_wechat_channel_openid (tenant_id,channel_key,openid),
  UNIQUE KEY uk_customer_wechat_customer_channel (tenant_id,customer_id,channel_key),
  KEY idx_customer_wechat_unionid (tenant_id,unionid),
  CONSTRAINT fk_customer_wechat_identity_customer FOREIGN KEY (tenant_id,customer_id) REFERENCES customers(tenant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO customer_wechat_identities(tenant_id,customer_id,channel_key,appid,openid,unionid,last_login_at)
SELECT tenant_id,id,'tanban-public','',wechat_openid,unionid,COALESCE(last_seen_at,created_at)
FROM customers
WHERE wechat_openid IS NOT NULL AND wechat_openid<>'';

ALTER TABLE orders
  ADD COLUMN source_miniapp_channel_key VARCHAR(64) COLLATE utf8mb4_bin NOT NULL DEFAULT 'tanban-public' AFTER source,
  ADD COLUMN source_miniapp_appid VARCHAR(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER source_miniapp_channel_key,
  ADD KEY idx_orders_miniapp_channel (tenant_id,source_miniapp_channel_key,created_at);
