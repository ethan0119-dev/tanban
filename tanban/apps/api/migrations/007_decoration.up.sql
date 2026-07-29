CREATE TABLE IF NOT EXISTS store_decoration_versions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  version_no INT UNSIGNED NOT NULL,
  schema_version INT UNSIGNED NOT NULL DEFAULT 1,
  config_json MEDIUMTEXT NOT NULL,
  publish_note VARCHAR(255) NOT NULL DEFAULT '',
  source_version_id BIGINT UNSIGNED NULL,
  published_by BIGINT UNSIGNED NOT NULL,
  published_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_decoration_version (tenant_id, store_id, version_no),
  KEY idx_decoration_versions_store (tenant_id, store_id, published_at),
  KEY idx_decoration_versions_source (source_version_id),
  CONSTRAINT fk_decoration_versions_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_decoration_versions_source FOREIGN KEY (source_version_id) REFERENCES store_decoration_versions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS store_decorations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  schema_version INT UNSIGNED NOT NULL DEFAULT 1,
  draft_json MEDIUMTEXT NOT NULL,
  draft_revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  published_version_id BIGINT UNSIGNED NULL,
  created_by BIGINT UNSIGNED NOT NULL,
  updated_by BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_store_decorations_store (tenant_id, store_id),
  KEY idx_store_decorations_published (published_version_id),
  CONSTRAINT fk_store_decorations_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_store_decorations_published FOREIGN KEY (published_version_id) REFERENCES store_decoration_versions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS media_assets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(120) NOT NULL,
  kind VARCHAR(24) NOT NULL DEFAULT 'IMAGE',
  url VARCHAR(1024) NOT NULL,
  storage_key VARCHAR(512) NOT NULL DEFAULT '',
  mime_type VARCHAR(100) NOT NULL DEFAULT '',
  width INT UNSIGNED NOT NULL DEFAULT 0,
  height INT UNSIGNED NOT NULL DEFAULT 0,
  size_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_by BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_media_assets_store (tenant_id, store_id, status, created_at),
  CONSTRAINT fk_media_assets_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
