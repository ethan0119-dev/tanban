ALTER TABLE tenants
  DROP KEY idx_tenants_food_business_license_media,
  DROP KEY idx_tenants_business_license_media,
  DROP COLUMN food_business_license_media_id,
  DROP COLUMN business_license_media_id;
