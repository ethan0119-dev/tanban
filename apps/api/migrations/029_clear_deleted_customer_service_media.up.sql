UPDATE store_operation_settings os
JOIN media_assets a
  ON a.tenant_id=os.tenant_id
 AND a.store_id=os.store_id
 AND a.url=os.customer_service_qr_url
SET os.customer_service_qr_url=''
WHERE a.kind='IMAGE'
  AND (a.status<>'ACTIVE' OR a.deleted_at IS NOT NULL);
