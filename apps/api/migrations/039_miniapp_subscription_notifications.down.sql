ALTER TABLE customer_account_payment_intents
  DROP KEY idx_customer_account_payment_miniapp,
  DROP COLUMN source_miniapp_appid,
  DROP COLUMN source_miniapp_channel_key;

DROP TABLE IF EXISTS miniapp_notification_outbox;
DROP TABLE IF EXISTS customer_subscription_results;
DROP TABLE IF EXISTS miniapp_notification_templates;
