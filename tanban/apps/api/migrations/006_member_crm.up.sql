CREATE TABLE IF NOT EXISTS customers (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  source_store_id BIGINT UNSIGNED NULL,
  public_id VARCHAR(40) COLLATE utf8mb4_bin NOT NULL,
  wechat_openid VARCHAR(128) COLLATE utf8mb4_bin NULL,
  guest_key VARCHAR(128) COLLATE utf8mb4_bin NULL,
  unionid VARCHAR(128) COLLATE utf8mb4_bin NULL,
  name VARCHAR(80) NOT NULL DEFAULT '',
  avatar_url VARCHAR(512) NOT NULL DEFAULT '',
  phone VARCHAR(32) NOT NULL DEFAULT '',
  source VARCHAR(32) NOT NULL DEFAULT 'MANUAL',
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  remark VARCHAR(500) NOT NULL DEFAULT '',
  registered_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_seen_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_customers_tenant_public (tenant_id, public_id),
  UNIQUE KEY uk_customers_tenant_openid (tenant_id, wechat_openid),
  UNIQUE KEY uk_customers_tenant_guest (tenant_id, guest_key),
  UNIQUE KEY uk_customers_tenant_id (tenant_id, id),
  KEY idx_customers_tenant_status (tenant_id, status, registered_at),
  KEY idx_customers_tenant_phone (tenant_id, phone),
  CONSTRAINT fk_customers_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_customers_source_store FOREIGN KEY (source_store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS customer_tags (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(64) NOT NULL,
  color VARCHAR(24) NOT NULL DEFAULT 'blue',
  description VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_customer_tags_tenant_name (tenant_id, name),
  UNIQUE KEY uk_customer_tags_tenant_id (tenant_id, id),
  KEY idx_customer_tags_tenant_status (tenant_id, status),
  CONSTRAINT fk_customer_tags_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS customer_tag_assignments (
  tenant_id BIGINT UNSIGNED NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  tag_id BIGINT UNSIGNED NOT NULL,
  assigned_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  source VARCHAR(24) NOT NULL DEFAULT 'MANUAL',
  assigned_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (tenant_id, customer_id, tag_id),
  KEY idx_customer_tag_assignments_tag (tenant_id, tag_id, customer_id),
  CONSTRAINT fk_customer_tag_assignments_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id),
  CONSTRAINT fk_customer_tag_assignments_tag FOREIGN KEY (tenant_id, tag_id) REFERENCES customer_tags(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS member_levels (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(80) NOT NULL,
  rank_no INT NOT NULL DEFAULT 0,
  acquire_type VARCHAR(24) NOT NULL DEFAULT 'GROWTH',
  growth_threshold BIGINT NOT NULL DEFAULT 0,
  price_cents BIGINT NOT NULL DEFAULT 0,
  valid_days INT NOT NULL DEFAULT 0,
  benefits_json TEXT NOT NULL,
  upgrade_gift_json TEXT NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_member_levels_tenant_name (tenant_id, name),
  UNIQUE KEY uk_member_levels_tenant_id (tenant_id, id),
  KEY idx_member_levels_tenant_rank (tenant_id, status, rank_no),
  CONSTRAINT fk_member_levels_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS membership_settings (
  tenant_id BIGINT UNSIGNED NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  card_name VARCHAR(80) NOT NULL DEFAULT '会员卡',
  card_color VARCHAR(24) NOT NULL DEFAULT '#8b5635',
  card_image_url VARCHAR(512) NOT NULL DEFAULT '',
  auto_enroll TINYINT(1) NOT NULL DEFAULT 1,
  default_level_id BIGINT UNSIGNED NULL,
  growth_per_yuan INT NOT NULL DEFAULT 1,
  agreement_url VARCHAR(512) NOT NULL DEFAULT '',
  show_balance TINYINT(1) NOT NULL DEFAULT 1,
  updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (tenant_id),
  CONSTRAINT fk_membership_settings_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_membership_settings_level FOREIGN KEY (tenant_id, default_level_id) REFERENCES member_levels(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS members (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  member_no VARCHAR(48) NOT NULL,
  current_level_id BIGINT UNSIGNED NULL,
  growth_value BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  joined_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  expires_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_members_tenant_customer (tenant_id, customer_id),
  UNIQUE KEY uk_members_tenant_no (tenant_id, member_no),
  UNIQUE KEY uk_members_tenant_id (tenant_id, id),
  KEY idx_members_tenant_level (tenant_id, current_level_id, status),
  CONSTRAINT fk_members_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id),
  CONSTRAINT fk_members_level FOREIGN KEY (tenant_id, current_level_id) REFERENCES member_levels(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS member_card_issuances (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  issue_no VARCHAR(48) NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  member_id BIGINT UNSIGNED NOT NULL,
  level_id BIGINT UNSIGNED NULL,
  issue_source VARCHAR(24) NOT NULL DEFAULT 'MANUAL',
  idempotency_key VARCHAR(128) NOT NULL,
  request_fingerprint CHAR(64) NOT NULL,
  level_snapshot_json TEXT NOT NULL,
  valid_from DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  valid_to DATETIME(3) NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_member_card_issuances_no (tenant_id, issue_no),
  UNIQUE KEY uk_member_card_issuances_idempotency (tenant_id, idempotency_key),
  KEY idx_member_card_issuances_customer (tenant_id, customer_id, created_at),
  CONSTRAINT fk_member_card_issuances_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id),
  CONSTRAINT fk_member_card_issuances_member FOREIGN KEY (tenant_id, member_id) REFERENCES members(tenant_id, id),
  CONSTRAINT fk_member_card_issuances_level FOREIGN KEY (tenant_id, level_id) REFERENCES member_levels(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS member_level_orders (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  order_no VARCHAR(48) NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  member_id BIGINT UNSIGNED NULL,
  level_id BIGINT UNSIGNED NOT NULL,
  level_snapshot_json TEXT NOT NULL,
  amount_cents BIGINT NOT NULL DEFAULT 0,
  payment_method VARCHAR(32) NOT NULL DEFAULT 'MANUAL',
  payment_status VARCHAR(24) NOT NULL DEFAULT 'RECORDED',
  status VARCHAR(24) NOT NULL DEFAULT 'COMPLETED',
  remark VARCHAR(255) NOT NULL DEFAULT '',
  idempotency_key VARCHAR(128) NOT NULL,
  request_fingerprint CHAR(64) NOT NULL,
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_member_level_orders_no (tenant_id, order_no),
  UNIQUE KEY uk_member_level_orders_idempotency (tenant_id, idempotency_key),
  KEY idx_member_level_orders_customer (tenant_id, customer_id, created_at),
  CONSTRAINT fk_member_level_orders_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id),
  CONSTRAINT fk_member_level_orders_member FOREIGN KEY (tenant_id, member_id) REFERENCES members(tenant_id, id),
  CONSTRAINT fk_member_level_orders_level FOREIGN KEY (tenant_id, level_id) REFERENCES member_levels(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS balance_accounts (
  tenant_id BIGINT UNSIGNED NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  principal_cents BIGINT NOT NULL DEFAULT 0,
  bonus_cents BIGINT NOT NULL DEFAULT 0,
  version BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (tenant_id, customer_id),
  CONSTRAINT fk_balance_accounts_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS balance_ledger (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  account_bucket VARCHAR(24) NOT NULL,
  delta_cents BIGINT NOT NULL,
  balance_before_cents BIGINT NOT NULL,
  balance_after_cents BIGINT NOT NULL,
  entry_type VARCHAR(32) NOT NULL,
  business_type VARCHAR(32) NOT NULL,
  business_no VARCHAR(64) NOT NULL DEFAULT '',
  idempotency_key VARCHAR(128) NOT NULL,
  reversal_of BIGINT UNSIGNED NULL,
  operator_user_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_balance_ledger_idempotency (tenant_id, idempotency_key),
  KEY idx_balance_ledger_customer (tenant_id, customer_id, created_at),
  KEY idx_balance_ledger_business (tenant_id, business_type, business_no),
  CONSTRAINT fk_balance_ledger_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id),
  CONSTRAINT fk_balance_ledger_reversal FOREIGN KEY (reversal_of) REFERENCES balance_ledger(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS stored_value_rules (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(100) NOT NULL,
  recharge_cents BIGINT NOT NULL,
  gift_cents BIGINT NOT NULL DEFAULT 0,
  gift_growth BIGINT NOT NULL DEFAULT 0,
  benefits_json TEXT NOT NULL,
  per_customer_limit INT NOT NULL DEFAULT 0,
  starts_at DATETIME(3) NULL,
  ends_at DATETIME(3) NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_stored_value_rules_tenant_name (tenant_id, name),
  UNIQUE KEY uk_stored_value_rules_tenant_id (tenant_id, id),
  KEY idx_stored_value_rules_status (tenant_id, status, starts_at, ends_at),
  CONSTRAINT fk_stored_value_rules_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS stored_value_settings (
  tenant_id BIGINT UNSIGNED NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  min_recharge_cents BIGINT NOT NULL DEFAULT 100,
  max_recharge_cents BIGINT NOT NULL DEFAULT 1000000,
  max_balance_cents BIGINT NOT NULL DEFAULT 1000000,
  deduction_order VARCHAR(32) NOT NULL DEFAULT 'BONUS_FIRST',
  refund_policy VARCHAR(32) NOT NULL DEFAULT 'MANUAL_REVIEW',
  agreement_url VARCHAR(512) NOT NULL DEFAULT '',
  show_in_miniapp TINYINT(1) NOT NULL DEFAULT 0,
  updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (tenant_id),
  CONSTRAINT fk_stored_value_settings_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS stored_value_records (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  record_no VARCHAR(48) NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  rule_id BIGINT UNSIGNED NULL,
  rule_snapshot_json TEXT NOT NULL,
  principal_cents BIGINT NOT NULL,
  gift_cents BIGINT NOT NULL DEFAULT 0,
  payment_method VARCHAR(32) NOT NULL DEFAULT 'MANUAL',
  status VARCHAR(24) NOT NULL DEFAULT 'CONFIRMED',
  idempotency_key VARCHAR(128) NOT NULL,
  request_fingerprint CHAR(64) NOT NULL,
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_stored_value_records_no (tenant_id, record_no),
  UNIQUE KEY uk_stored_value_records_idempotency (tenant_id, idempotency_key),
  KEY idx_stored_value_records_customer (tenant_id, customer_id, created_at),
  CONSTRAINT fk_stored_value_records_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id),
  CONSTRAINT fk_stored_value_records_rule FOREIGN KEY (tenant_id, rule_id) REFERENCES stored_value_rules(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- OpenID is an opaque, case-sensitive identifier. Change the source column
-- before grouping and joining historical orders so backfill cannot merge two
-- distinct WeChat identities under the database's default general_ci collation.
ALTER TABLE orders
  MODIFY customer_openid VARCHAR(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '';

ALTER TABLE orders
  ADD COLUMN customer_id BIGINT UNSIGNED NULL AFTER customer_openid,
  ADD KEY idx_orders_tenant_customer (tenant_id, customer_id, created_at),
  ADD CONSTRAINT fk_orders_customer FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id);

INSERT IGNORE INTO customers(tenant_id,source_store_id,public_id,wechat_openid,name,phone,source,status,registered_at)
SELECT tenant_id,MIN(store_id),CONCAT('CUH',LEFT(SHA2(CONCAT(tenant_id,':',customer_openid),256),32)),customer_openid,MAX(customer_name),MAX(customer_phone),'ORDER_BACKFILL','ACTIVE',MIN(created_at)
FROM orders
WHERE customer_openid<>''
GROUP BY tenant_id,customer_openid;

UPDATE orders o
JOIN customers c ON c.tenant_id=o.tenant_id AND c.wechat_openid=o.customer_openid
SET o.customer_id=c.id
WHERE o.customer_id IS NULL AND o.customer_openid<>'';
