UPDATE media_assets
SET url=REPLACE(url,'https://tbapi.666qwe.cn','https://api.tanban.com.cn')
WHERE url LIKE 'https://tbapi.666qwe.cn/%';

UPDATE marketing_placements
SET image_url=REPLACE(image_url,'https://tbapi.666qwe.cn','https://api.tanban.com.cn')
WHERE image_url LIKE 'https://tbapi.666qwe.cn/%';

UPDATE product_images
SET url=REPLACE(url,'https://tbapi.666qwe.cn','https://api.tanban.com.cn')
WHERE url LIKE 'https://tbapi.666qwe.cn/%';

UPDATE products
SET image_url=REPLACE(image_url,'https://tbapi.666qwe.cn','https://api.tanban.com.cn')
WHERE image_url LIKE 'https://tbapi.666qwe.cn/%';

UPDATE store_decoration_versions
SET config_json=REPLACE(config_json,'https://tbapi.666qwe.cn','https://api.tanban.com.cn')
WHERE config_json LIKE '%https://tbapi.666qwe.cn/%';

UPDATE store_decorations
SET draft_json=REPLACE(draft_json,'https://tbapi.666qwe.cn','https://api.tanban.com.cn')
WHERE draft_json LIKE '%https://tbapi.666qwe.cn/%';

UPDATE store_operation_settings
SET customer_service_qr_url=REPLACE(customer_service_qr_url,'https://tbapi.666qwe.cn','https://api.tanban.com.cn')
WHERE customer_service_qr_url LIKE 'https://tbapi.666qwe.cn/%';

UPDATE stores
SET logo_url=REPLACE(logo_url,'https://tbapi.666qwe.cn','https://api.tanban.com.cn'),
    banner_url=REPLACE(banner_url,'https://tbapi.666qwe.cn','https://api.tanban.com.cn')
WHERE logo_url LIKE 'https://tbapi.666qwe.cn/%'
   OR banner_url LIKE 'https://tbapi.666qwe.cn/%';
