ALTER TABLE tenant_miniapp_channels
  ADD COLUMN public_default_entry TINYINT(1) NOT NULL DEFAULT 0 AFTER public_enabled,
  ADD KEY idx_tenant_miniapp_public_default (public_default_entry, public_enabled, tenant_id);

UPDATE tenant_miniapp_channels c
JOIN stores s ON s.tenant_id=c.tenant_id AND s.code='manong-coffee-gulou' AND s.deleted_at IS NULL
SET c.public_default_entry=1
WHERE c.public_enabled=1;
