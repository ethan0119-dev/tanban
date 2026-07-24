ALTER TABLE tenants
  ADD COLUMN service_expires_at DATE NULL AFTER status,
  ADD KEY idx_tenants_service_expires_at (service_expires_at);
