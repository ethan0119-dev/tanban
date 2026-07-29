ALTER TABLE stores
  ADD COLUMN timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai' AFTER business_hours;

CREATE TABLE IF NOT EXISTS store_business_periods (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  weekday TINYINT UNSIGNED NOT NULL COMMENT 'ISO weekday: Monday=1, Sunday=7',
  start_minute SMALLINT UNSIGNED NOT NULL,
  end_minute SMALLINT UNSIGNED NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_store_business_period (tenant_id, store_id, weekday, start_minute, end_minute),
  KEY idx_store_business_period_lookup (tenant_id, store_id, status, weekday, sort_order),
  CONSTRAINT fk_store_business_period_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS store_business_overrides (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  override_type VARCHAR(24) NOT NULL COMMENT 'OPEN or CLOSED',
  starts_at DATETIME(3) NOT NULL,
  ends_at DATETIME(3) NOT NULL,
  reason VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_store_business_override_lookup (tenant_id, store_id, status, starts_at, ends_at),
  CONSTRAINT fk_store_business_override_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS fast_food_plates (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(80) NOT NULL,
  plate_code VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
  public_scene VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_fast_food_plates_scene (public_scene),
  UNIQUE KEY uk_fast_food_plates_store_code (tenant_id, store_id, plate_code),
  KEY idx_fast_food_plates_store (tenant_id, store_id, status, sort_order),
  CONSTRAINT fk_fast_food_plates_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_pickup_sequences (
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  business_date DATE NOT NULL,
  last_value INT UNSIGNED NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (tenant_id, store_id, business_date),
  CONSTRAINT fk_order_pickup_sequence_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE orders
  ADD COLUMN business_date DATE NULL AFTER order_type,
  ADD COLUMN pickup_sequence INT UNSIGNED NULL AFTER business_date,
  ADD COLUMN pickup_code VARCHAR(16) NOT NULL DEFAULT '' AFTER pickup_sequence,
  ADD COLUMN fast_food_plate_id BIGINT UNSIGNED NULL AFTER pickup_code,
  ADD COLUMN fast_food_plate_public_id_snapshot VARCHAR(32) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER fast_food_plate_id,
  ADD COLUMN fast_food_plate_name_snapshot VARCHAR(80) NOT NULL DEFAULT '' AFTER fast_food_plate_public_id_snapshot,
  ADD COLUMN fast_food_plate_code_snapshot VARCHAR(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER fast_food_plate_name_snapshot,
  ADD UNIQUE KEY uk_orders_pickup_sequence (tenant_id, store_id, business_date, pickup_sequence),
  ADD KEY idx_orders_fast_food_plate (tenant_id, store_id, fast_food_plate_id, created_at);

INSERT IGNORE INTO store_business_periods(tenant_id,store_id,weekday,start_minute,end_minute,sort_order,status)
SELECT s.tenant_id,s.id,d.weekday,0,0,0,'ACTIVE'
FROM stores s
JOIN (
  SELECT 1 weekday UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4
  UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7
) d
WHERE s.deleted_at IS NULL;

UPDATE store_business_periods p
JOIN stores s ON s.id=p.store_id AND s.tenant_id=p.tenant_id
SET p.start_minute=HOUR(STR_TO_DATE(SUBSTRING_INDEX(s.business_hours,'-',1),'%H:%i'))*60+MINUTE(STR_TO_DATE(SUBSTRING_INDEX(s.business_hours,'-',1),'%H:%i')),
    p.end_minute=CASE
      WHEN SUBSTRING_INDEX(s.business_hours,'-',-1)='24:00' THEN 1440
      ELSE HOUR(STR_TO_DATE(SUBSTRING_INDEX(s.business_hours,'-',-1),'%H:%i'))*60+MINUTE(STR_TO_DATE(SUBSTRING_INDEX(s.business_hours,'-',-1),'%H:%i'))
    END
WHERE s.business_hours LIKE '%-%'
  AND STR_TO_DATE(SUBSTRING_INDEX(s.business_hours,'-',1),'%H:%i') IS NOT NULL
  AND (SUBSTRING_INDEX(s.business_hours,'-',-1)='24:00' OR STR_TO_DATE(SUBSTRING_INDEX(s.business_hours,'-',-1),'%H:%i') IS NOT NULL);
