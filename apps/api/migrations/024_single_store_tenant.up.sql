ALTER TABLE stores
  ADD UNIQUE KEY uk_stores_tenant (tenant_id);
