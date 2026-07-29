ALTER TABLE orders
  ADD COLUMN settlement_mode_snapshot VARCHAR(24) NOT NULL DEFAULT 'PAY_BEFORE' AFTER order_type,
  ADD COLUMN addition_count INT UNSIGNED NOT NULL DEFAULT 1 AFTER settlement_mode_snapshot;

CREATE TABLE IF NOT EXISTS order_addition_requests (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  order_id BIGINT UNSIGNED NOT NULL,
  idempotency_key VARCHAR(128) NOT NULL,
  request_fingerprint VARCHAR(64) NOT NULL,
  addition_sequence INT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_order_addition_idempotency (tenant_id,store_id,idempotency_key),
  UNIQUE KEY uk_order_addition_sequence (tenant_id,order_id,addition_sequence),
  KEY idx_order_addition_order (tenant_id,order_id,created_at),
  CONSTRAINT fk_order_addition_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
