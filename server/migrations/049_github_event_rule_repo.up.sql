-- Add per-repository scoping to GitHub event rules.
-- A rule with repo_full_name = '' is a workspace-wide default for the event type.
-- A rule with a specific "owner/repo" overrides the default for that repo.
ALTER TABLE github_event_rule
    ADD COLUMN repo_full_name TEXT NOT NULL DEFAULT '';

ALTER TABLE github_event_rule
    DROP CONSTRAINT github_event_rule_workspace_id_event_type_key;

ALTER TABLE github_event_rule
    ADD CONSTRAINT github_event_rule_workspace_event_repo_key
    UNIQUE (workspace_id, event_type, repo_full_name);

CREATE INDEX idx_github_event_rule_lookup
    ON github_event_rule(workspace_id, event_type, repo_full_name);
