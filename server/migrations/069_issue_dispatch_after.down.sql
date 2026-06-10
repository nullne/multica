DROP INDEX IF EXISTS idx_issue_dispatch_pending;

ALTER TABLE issue
    DROP COLUMN IF EXISTS dispatch_after,
    DROP COLUMN IF EXISTS dispatch_after_fired_at;
