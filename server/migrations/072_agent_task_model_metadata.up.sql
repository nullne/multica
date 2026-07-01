ALTER TABLE agent_task_queue
    ADD COLUMN provider TEXT,
    ADD COLUMN model TEXT,
    ADD COLUMN thinking_level TEXT;
