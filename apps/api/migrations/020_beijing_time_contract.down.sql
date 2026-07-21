UPDATE store_business_overrides
SET starts_at=DATE_SUB(starts_at, INTERVAL 8 HOUR),
    ends_at=DATE_SUB(ends_at, INTERVAL 8 HOUR);
