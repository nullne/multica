-- Allow "github" as a valid creator_type for issues created by GitHub App events.
ALTER TABLE issue DROP CONSTRAINT issue_creator_type_check;
ALTER TABLE issue ADD CONSTRAINT issue_creator_type_check CHECK (creator_type IN ('member', 'agent', 'webhook', 'github'));

-- GitHub event rules: per workspace + event type mapping to an agent.
CREATE TABLE github_event_rule (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    event_type            TEXT NOT NULL CHECK (event_type IN ('push', 'pull_request', 'issues', 'issue_comment')),
    agent_id              UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    enabled               BOOLEAN NOT NULL DEFAULT true,
    title_template        TEXT NOT NULL DEFAULT '',
    description_template  TEXT NOT NULL DEFAULT '',
    dispatch_provider     TEXT,
    dispatch_daemon_id    UUID REFERENCES daemon(id) ON DELETE SET NULL,
    dispatch_daemon_label TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, event_type)
);

CREATE INDEX idx_github_event_rule_workspace ON github_event_rule(workspace_id);
