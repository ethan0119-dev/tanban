ALTER TABLE orders
  DROP FOREIGN KEY fk_orders_customer,
  DROP KEY idx_orders_tenant_customer,
  DROP COLUMN customer_id;

DROP TABLE IF EXISTS stored_value_records;
DROP TABLE IF EXISTS stored_value_settings;
DROP TABLE IF EXISTS stored_value_rules;
DROP TABLE IF EXISTS balance_ledger;
DROP TABLE IF EXISTS balance_accounts;
DROP TABLE IF EXISTS member_level_orders;
DROP TABLE IF EXISTS member_card_issuances;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS membership_settings;
DROP TABLE IF EXISTS member_levels;
DROP TABLE IF EXISTS customer_tag_assignments;
DROP TABLE IF EXISTS customer_tags;
DROP TABLE IF EXISTS customers;

ALTER TABLE orders
  MODIFY customer_openid VARCHAR(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '';
