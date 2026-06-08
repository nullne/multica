-- Managed routines (system-owned, e.g. GitHub auto-fix) and removal of the
-- legacy webhook + recurring template tables. Their data was already
-- backfilled into routines by migration 062; execution has been disabled
-- since. Routines are now the single automation path.
ALTER TABLE routine ADD COLUMN managed BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX idx_routine_managed_workspace ON routine(workspace_id) WHERE managed = TRUE;

DROP TABLE IF EXISTS webhook_event_log;
DROP TABLE IF EXISTS webhook_action;
DROP TABLE IF EXISTS webhook;
DROP TABLE IF EXISTS recurring_template_subscriber;
DROP TABLE IF EXISTS recurring_issue_template;
