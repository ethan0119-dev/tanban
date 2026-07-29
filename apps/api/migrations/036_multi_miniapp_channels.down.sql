ALTER TABLE orders
  DROP KEY idx_orders_miniapp_channel,
  DROP COLUMN source_miniapp_appid,
  DROP COLUMN source_miniapp_channel_key;

DROP TABLE IF EXISTS customer_wechat_identities;
DROP TABLE IF EXISTS tenant_miniapp_channels;
