-- Stores created after the original store-operations migration did not yet
-- receive normalized weekly periods, which made them appear permanently closed.
INSERT INTO store_business_periods(tenant_id,store_id,weekday,start_minute,end_minute,sort_order,status)
SELECT s.tenant_id,s.id,days.weekday,
  CASE
    WHEN s.business_hours LIKE '%-%' AND STR_TO_DATE(TRIM(SUBSTRING_INDEX(s.business_hours,'-',1)),'%H:%i') IS NOT NULL
      THEN HOUR(STR_TO_DATE(TRIM(SUBSTRING_INDEX(s.business_hours,'-',1)),'%H:%i'))*60+MINUTE(STR_TO_DATE(TRIM(SUBSTRING_INDEX(s.business_hours,'-',1)),'%H:%i'))
    ELSE 0
  END,
  CASE
    WHEN TRIM(SUBSTRING_INDEX(s.business_hours,'-',-1))='24:00' THEN 1440
    WHEN s.business_hours LIKE '%-%' AND STR_TO_DATE(TRIM(SUBSTRING_INDEX(s.business_hours,'-',-1)),'%H:%i') IS NOT NULL
      THEN HOUR(STR_TO_DATE(TRIM(SUBSTRING_INDEX(s.business_hours,'-',-1)),'%H:%i'))*60+MINUTE(STR_TO_DATE(TRIM(SUBSTRING_INDEX(s.business_hours,'-',-1)),'%H:%i'))
    ELSE 0
  END,
  0,'ACTIVE'
FROM stores s
JOIN (
  SELECT 1 weekday UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4
  UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7
) days
WHERE s.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM store_business_periods existing
    WHERE existing.tenant_id=s.tenant_id AND existing.store_id=s.id AND existing.deleted_at IS NULL
  );
