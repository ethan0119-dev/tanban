CREATE TABLE IF NOT EXISTS store_profiles (
  store_id BIGINT UNSIGNED NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  visible_in_miniapp TINYINT(1) NOT NULL DEFAULT 1,
  contact_name VARCHAR(80) NOT NULL DEFAULT '',
  region VARCHAR(120) NOT NULL DEFAULT '',
  main_products VARCHAR(255) NOT NULL DEFAULT '',
  average_spend_cents BIGINT UNSIGNED NOT NULL DEFAULT 0,
  service_channels_json TEXT NOT NULL,
  environment_image_urls_json TEXT NOT NULL,
  food_safety_image_urls_json TEXT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (store_id),
  UNIQUE KEY uk_store_profiles_scope (tenant_id, store_id),
  KEY idx_store_profiles_visibility (tenant_id, visible_in_miniapp),
  CONSTRAINT fk_store_profiles_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO store_profiles(
  store_id,tenant_id,visible_in_miniapp,contact_name,region,main_products,average_spend_cents,
  service_channels_json,environment_image_urls_json,food_safety_image_urls_json
)
SELECT s.id,s.tenant_id,1,t.contact_name,'','',0,'["DINE_IN","TAKEOUT"]','[]','[]'
FROM stores s
JOIN tenants t ON t.id=s.tenant_id
WHERE s.deleted_at IS NULL;
