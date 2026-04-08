ALTER TABLE issue
    ADD COLUMN verifier_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL;

CREATE INDEX idx_issue_verifier_agent ON issue(verifier_agent_id);
