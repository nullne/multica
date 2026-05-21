ALTER TABLE agent_task_queue
  ADD COLUMN result_comment_id UUID REFERENCES comment(id) ON DELETE SET NULL;

CREATE INDEX idx_agent_task_queue_result_comment_id
  ON agent_task_queue (result_comment_id)
  WHERE result_comment_id IS NOT NULL;
