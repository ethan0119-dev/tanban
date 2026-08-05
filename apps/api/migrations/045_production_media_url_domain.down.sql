-- This one-time production-domain rewrite is intentionally irreversible.
-- Reverting every new-domain URL would also corrupt media created after rollout.
SELECT 1;
