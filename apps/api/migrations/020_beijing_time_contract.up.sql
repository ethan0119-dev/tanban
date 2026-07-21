UPDATE stores
SET timezone='Asia/Shanghai'
WHERE timezone<>'Asia/Shanghai';

-- Overrides created before this migration were explicitly persisted as UTC
-- wall-clock DATETIME values. Convert them once to Beijing wall-clock values.
UPDATE store_business_overrides
SET starts_at=DATE_ADD(starts_at, INTERVAL 8 HOUR),
    ends_at=DATE_ADD(ends_at, INTERVAL 8 HOUR);
