CREATE TABLE IF NOT EXISTS platform_announcements (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  title VARCHAR(160) NOT NULL,
  summary VARCHAR(300) NOT NULL DEFAULT '',
  content TEXT NOT NULL,
  category VARCHAR(32) NOT NULL DEFAULT 'SYSTEM_UPDATE',
  severity VARCHAR(24) NOT NULL DEFAULT 'INFO',
  audience_type VARCHAR(24) NOT NULL DEFAULT 'ALL',
  status VARCHAR(24) NOT NULL DEFAULT 'DRAFT',
  published_at DATETIME(3) NULL,
  withdrawn_at DATETIME(3) NULL,
  created_by BIGINT UNSIGNED NOT NULL,
  updated_by BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_platform_announcements_status_published (status, published_at),
  KEY idx_platform_announcements_category (category, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS platform_announcement_targets (
  announcement_id BIGINT UNSIGNED NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (announcement_id, tenant_id),
  KEY idx_platform_announcement_targets_tenant (tenant_id, announcement_id),
  CONSTRAINT fk_platform_announcement_targets_announcement FOREIGN KEY (announcement_id) REFERENCES platform_announcements(id) ON DELETE CASCADE,
  CONSTRAINT fk_platform_announcement_targets_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS merchant_notification_recipients (
  announcement_id BIGINT UNSIGNED NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (announcement_id, tenant_id),
  KEY idx_merchant_notification_recipients_tenant (tenant_id, announcement_id),
  CONSTRAINT fk_merchant_notification_recipients_announcement FOREIGN KEY (announcement_id) REFERENCES platform_announcements(id) ON DELETE CASCADE,
  CONSTRAINT fk_merchant_notification_recipients_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS merchant_notification_reads (
  announcement_id BIGINT UNSIGNED NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  read_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (announcement_id, user_id),
  KEY idx_merchant_notification_reads_tenant_user (tenant_id, user_id, read_at),
  CONSTRAINT fk_merchant_notification_reads_announcement FOREIGN KEY (announcement_id) REFERENCES platform_announcements(id) ON DELETE CASCADE,
  CONSTRAINT fk_merchant_notification_reads_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
