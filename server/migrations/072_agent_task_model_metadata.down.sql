ALTER TABLE agent_task_queue
    DROP COLUMN IF EXISTS thinking_level,
    DROP COLUMN IF EXISTS model,
    DROP COLUMN IF EXISTS provider;
