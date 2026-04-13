-- Change agent.provider (single TEXT) to agent.providers (TEXT array).
ALTER TABLE agent ADD COLUMN providers TEXT[] NOT NULL DEFAULT '{}';

-- Backfill: convert single provider to array.
UPDATE agent SET providers = ARRAY[provider] WHERE provider != '';

-- Remove deprecated columns.
ALTER TABLE agent DROP COLUMN provider;
ALTER TABLE agent DROP COLUMN max_concurrent_tasks;
ALTER TABLE agent DROP COLUMN runtime_id;
ALTER TABLE agent DROP COLUMN runtime_mode;
ALTER TABLE agent DROP COLUMN runtime_config;

-- Default github_code_access to 'read' for new agents.
ALTER TABLE agent ALTER COLUMN github_code_access SET DEFAULT 'read';
