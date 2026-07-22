CREATE TABLE IF NOT EXISTS accounts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username VARCHAR(64) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  display_name VARCHAR(80) NOT NULL DEFAULT '',
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  platform_role VARCHAR(40) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_accounts_username (username),
  KEY idx_accounts_platform_role (platform_role, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tenant_memberships (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  account_id BIGINT UNSIGNED NOT NULL,
  role VARCHAR(40) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_memberships_account_tenant (account_id, tenant_id),
  KEY idx_tenant_memberships_tenant_role (tenant_id, role, status),
  CONSTRAINT fk_tenant_memberships_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_tenant_memberships_account FOREIGN KEY (account_id) REFERENCES accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO accounts(id,username,password_hash,display_name,status,platform_role,created_at,updated_at,deleted_at)
SELECT id,username,password_hash,display_name,status,
  CASE WHEN tenant_id=0 THEN role ELSE NULL END,
  created_at,updated_at,deleted_at
FROM users;

INSERT IGNORE INTO tenant_memberships(tenant_id,account_id,role,status,created_at,updated_at,deleted_at)
SELECT tenant_id,id,role,status,created_at,updated_at,deleted_at
FROM users
WHERE tenant_id>0 AND role LIKE 'MERCHANT\_%';
