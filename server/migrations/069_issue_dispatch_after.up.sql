-- Add dispatch_after to gate assign-triggered agent dispatch until a given
-- time. dispatch_after_fired_at marks the one-shot dispatch as consumed so the
-- scheduler never enqueues twice for the same scheduled time.
ALTER TABLE issue
    ADD COLUMN dispatch_after TIMESTAMPTZ,
    ADD COLUMN dispatch_after_fired_at TIMESTAMPTZ;

CREATE INDEX idx_issue_dispatch_pending ON issue (dispatch_after)
    WHERE dispatch_after IS NOT NULL AND dispatch_after_fired_at IS NULL;
