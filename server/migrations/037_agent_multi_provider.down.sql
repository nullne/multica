ALTER TABLE agent ADD COLUMN runtime_config JSONB NOT NULL DEFAULT '{}';
ALTER TABLE agent ADD COLUMN runtime_mode TEXT NOT NULL DEFAULT 'local';
ALTER TABLE agent ADD COLUMN runtime_id UUID;
ALTER TABLE agent ADD COLUMN max_concurrent_tasks INTEGER NOT NULL DEFAULT 6;
ALTER TABLE agent ADD COLUMN provider TEXT NOT NULL DEFAULT '';
UPDATE agent SET provider = providers[1] WHERE array_length(providers, 1) > 0;
ALTER TABLE agent DROP COLUMN providers;
ALTER TABLE agent ALTER COLUMN github_code_access SET DEFAULT 'write';
