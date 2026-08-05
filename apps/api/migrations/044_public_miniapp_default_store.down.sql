ALTER TABLE tenant_miniapp_channels
  DROP KEY idx_tenant_miniapp_public_default,
  DROP COLUMN public_default_entry;
