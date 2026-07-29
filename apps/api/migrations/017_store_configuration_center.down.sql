ALTER TABLE tenants
  DROP COLUMN payment_settlement_cycle,
  DROP COLUMN payment_fee_bps;

DROP TABLE IF EXISTS store_operation_settings;
