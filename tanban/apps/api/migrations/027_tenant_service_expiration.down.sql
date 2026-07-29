ALTER TABLE tenants
  DROP KEY idx_tenants_service_expires_at,
  DROP COLUMN service_expires_at;
