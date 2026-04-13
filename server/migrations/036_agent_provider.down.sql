-- Re-add NOT NULL and FK constraint on runtime_id.
UPDATE agent SET runtime_id = (
    SELECT id FROM agent_runtime WHERE provider = agent.provider AND workspace_id = agent.workspace_id LIMIT 1
) WHERE runtime_id IS NULL;

ALTER TABLE agent ALTER COLUMN runtime_id SET NOT NULL;
ALTER TABLE agent ADD CONSTRAINT agent_runtime_id_fkey
    FOREIGN KEY (runtime_id) REFERENCES agent_runtime(id) ON DELETE RESTRICT;

ALTER TABLE agent DROP COLUMN IF EXISTS provider;
