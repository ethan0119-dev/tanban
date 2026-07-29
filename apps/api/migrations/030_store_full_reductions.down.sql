ALTER TABLE orders
  DROP COLUMN coupon_discount_cents,
  DROP COLUMN coupon_name,
  DROP COLUMN coupon_campaign_id,
  DROP COLUMN store_promotion_discount_cents,
  DROP COLUMN store_promotion_name,
  DROP COLUMN store_promotion_id,
  DROP COLUMN merchandise_subtotal_cents;

DROP TABLE IF EXISTS store_full_reduction_campaigns;
