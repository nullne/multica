DROP INDEX IF EXISTS idx_webhook_event_log_action;
ALTER TABLE webhook_event_log DROP COLUMN IF EXISTS action_id;
