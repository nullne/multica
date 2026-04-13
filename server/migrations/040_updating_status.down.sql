UPDATE daemon SET status = 'online' WHERE status = 'updating';
UPDATE agent_runtime SET status = 'online' WHERE status = 'updating';

ALTER TABLE daemon DROP CONSTRAINT IF EXISTS daemon_status_check;
ALTER TABLE daemon ADD CONSTRAINT daemon_status_check CHECK (status IN ('online', 'offline'));

ALTER TABLE agent_runtime DROP CONSTRAINT IF EXISTS agent_runtime_status_check;
ALTER TABLE agent_runtime ADD CONSTRAINT agent_runtime_status_check CHECK (status IN ('online', 'offline'));
