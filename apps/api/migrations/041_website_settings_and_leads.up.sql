CREATE TABLE IF NOT EXISTS website_settings (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  setting_key VARCHAR(64) NOT NULL,
  setting_value TEXT NOT NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_website_settings_key (setting_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS customer_leads (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  phone VARCHAR(20) NOT NULL,
  source VARCHAR(32) NOT NULL DEFAULT 'website',
  status VARCHAR(24) NOT NULL DEFAULT 'NEW',
  note VARCHAR(500) NOT NULL DEFAULT '',
  ip_address VARCHAR(45) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_leads_status (status),
  KEY idx_leads_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO website_settings (setting_key, setting_value) VALUES
  ('contact_phone', '400-888-1234'),
  ('contact_wechat', '扫码添加客服微信'),
  ('contact_email', 'hello@tanban.cn'),
  ('wechat_qr_url', ''),
  ('hero_image_url', ''),
  ('seo_title', '摊伴 TANBAN - 小店，也值得拥有一套好用的经营系统'),
  ('seo_description', '扫码点单、平板收银、自动打印、会员储值，一套摊伴就够了。')
ON DUPLICATE KEY UPDATE setting_key=setting_key;
