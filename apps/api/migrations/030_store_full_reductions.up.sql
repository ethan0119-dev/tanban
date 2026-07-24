CREATE TABLE IF NOT EXISTS store_full_reduction_campaigns (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(100) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  threshold_cents BIGINT NOT NULL,
  discount_cents BIGINT NOT NULL,
  order_types_json JSON NOT NULL,
  active_from DATETIME(3) NULL,
  active_to DATETIME(3) NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'DRAFT',
  version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_by BIGINT UNSIGNED NOT NULL,
  updated_by BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_store_full_reductions_active (tenant_id, store_id, status, active_from, active_to)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE orders
  ADD COLUMN merchandise_subtotal_cents BIGINT NOT NULL DEFAULT 0 AFTER total_cents,
  ADD COLUMN store_promotion_id BIGINT UNSIGNED NULL AFTER merchandise_subtotal_cents,
  ADD COLUMN store_promotion_name VARCHAR(100) NOT NULL DEFAULT '' AFTER store_promotion_id,
  ADD COLUMN store_promotion_discount_cents BIGINT NOT NULL DEFAULT 0 AFTER store_promotion_name,
  ADD COLUMN coupon_campaign_id BIGINT UNSIGNED NULL AFTER store_promotion_discount_cents,
  ADD COLUMN coupon_name VARCHAR(100) NOT NULL DEFAULT '' AFTER coupon_campaign_id,
  ADD COLUMN coupon_discount_cents BIGINT NOT NULL DEFAULT 0 AFTER coupon_name;
