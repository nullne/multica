-- Add provider column to agent (declarative — agent says what provider it needs,
-- not which specific runtime it runs on).
ALTER TABLE agent ADD COLUMN provider TEXT NOT NULL DEFAULT '';

-- Backfill provider from the currently linked runtime.
UPDATE agent a SET provider = rt.provider
FROM agent_runtime rt WHERE a.runtime_id = rt.id;

-- Drop the FK constraint — agent no longer requires a specific runtime.
ALTER TABLE agent DROP CONSTRAINT IF EXISTS agent_runtime_id_fkey;

-- Make runtime_id nullable (kept for backward compat, ignored in dispatch).
ALTER TABLE agent ALTER COLUMN runtime_id DROP NOT NULL;
