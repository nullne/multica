DROP INDEX IF EXISTS idx_agent_task_queue_result_comment_id;

ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS result_comment_id;
