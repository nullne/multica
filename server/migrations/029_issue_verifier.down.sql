DROP INDEX IF EXISTS idx_issue_verifier_agent;

ALTER TABLE issue
    DROP COLUMN IF EXISTS verifier_agent_id;
