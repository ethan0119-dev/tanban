ALTER TABLE tenants
  ADD COLUMN business_license_media_id BIGINT UNSIGNED NULL AFTER payment_sub_appid,
  ADD COLUMN food_business_license_media_id BIGINT UNSIGNED NULL AFTER business_license_media_id,
  ADD KEY idx_tenants_business_license_media (business_license_media_id),
  ADD KEY idx_tenants_food_business_license_media (food_business_license_media_id);
