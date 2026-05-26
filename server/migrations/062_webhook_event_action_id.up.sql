ALTER TABLE webhook_event_log ADD COLUMN action_id UUID REFERENCES webhook_action(id) ON DELETE SET NULL;
CREATE INDEX idx_webhook_event_log_action ON webhook_event_log(action_id, created_at DESC);
