-- Reverse the unification of GitHub webhook into the generic webhook system.

-- 1. Recreate github_event_rule table (matches schema after migration 049).
CREATE TABLE IF NOT EXISTS github_event_rule (
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
    repo_full_name        TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT github_event_rule_workspace_event_repo_key
        UNIQUE (workspace_id, event_type, repo_full_name)
);
CREATE INDEX IF NOT EXISTS idx_github_event_rule_workspace ON github_event_rule(workspace_id);
CREATE INDEX IF NOT EXISTS idx_github_event_rule_lookup
    ON github_event_rule(workspace_id, event_type, repo_full_name);

-- 2. Restore issue.linked_pr_url + linked_branch columns (data is reconstructed from issue_link).
ALTER TABLE issue
    ADD COLUMN linked_pr_url TEXT,
    ADD COLUMN linked_branch TEXT;

UPDATE issue i
SET linked_pr_url = l.url
FROM issue_link l
WHERE l.issue_id = i.id
  AND l.direction = 'output'
  AND l.kind = 'pr';

UPDATE issue i
SET linked_branch = l.url
FROM issue_link l
WHERE l.issue_id = i.id
  AND l.direction = 'output'
  AND l.kind = 'branch';

-- 3. Drop issue_link.
DROP TABLE IF EXISTS issue_link;

-- 4. Revert action_type and source_type CHECKs.
ALTER TABLE webhook_action DROP CONSTRAINT IF EXISTS webhook_action_action_type_check;
ALTER TABLE webhook_action ADD CONSTRAINT webhook_action_action_type_check
    CHECK (action_type IN ('create_issue'));

ALTER TABLE webhook DROP CONSTRAINT IF EXISTS webhook_source_type_check;
ALTER TABLE webhook ADD CONSTRAINT webhook_source_type_check
    CHECK (source_type IN ('standard', 'oss-alert'));

-- 5. Drop webhook bot/installation columns.
DROP INDEX IF EXISTS idx_webhook_installation_id;
ALTER TABLE webhook
    DROP COLUMN IF EXISTS installation_id,
    DROP COLUMN IF EXISTS bot_user_id;

-- 6. Drop user.kind.
DROP INDEX IF EXISTS idx_user_kind;
ALTER TABLE "user" DROP COLUMN IF EXISTS kind;
