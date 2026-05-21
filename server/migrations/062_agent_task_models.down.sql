ALTER TABLE agent_task_queue DROP COLUMN observed_model;
ALTER TABLE agent_task_queue DROP COLUMN requested_model;

ALTER TABLE agent DROP COLUMN default_model;
