ALTER TABLE agent ADD COLUMN default_model TEXT;

ALTER TABLE agent_task_queue ADD COLUMN requested_model TEXT;
ALTER TABLE agent_task_queue ADD COLUMN observed_model TEXT;
