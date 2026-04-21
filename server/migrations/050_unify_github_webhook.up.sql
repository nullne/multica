-- Unify GitHub webhook into the generic webhook system.
-- This migration:
--   1. Adds bot user kind so webhooks can post comments under a dedicated identity.
--   2. Adds bot_user_id and installation_id to the webhook table.
--   3. Extends source_type CHECK to include 'github' and action_type CHECK to include 'comment_issue'.
--   4. Creates the issue_link table for reverse lookup from external URLs to Multica issues.
--   5. Migrates issue.linked_pr_url + linked_branch into issue_link rows (direction='output'),
--      then drops those columns.
--   6. Drops the now-superseded github_event_rule table.

-- 1. Bot user kind
ALTER TABLE "user"
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'human'
        CHECK (kind IN ('human', 'bot'));

CREATE INDEX idx_user_kind ON "user"(kind);

-- 2. Webhook bot binding + GitHub installation binding
ALTER TABLE webhook
    ADD COLUMN bot_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    ADD COLUMN installation_id BIGINT;

CREATE UNIQUE INDEX idx_webhook_installation_id
    ON webhook(installation_id)
    WHERE installation_id IS NOT NULL;

-- 3. Extend source_type and action_type CHECKs
ALTER TABLE webhook DROP CONSTRAINT IF EXISTS webhook_source_type_check;
ALTER TABLE webhook ADD CONSTRAINT webhook_source_type_check
    CHECK (source_type IN ('standard', 'oss-alert', 'github'));

ALTER TABLE webhook_action DROP CONSTRAINT IF EXISTS webhook_action_action_type_check;
ALTER TABLE webhook_action ADD CONSTRAINT webhook_action_action_type_check
    CHECK (action_type IN ('create_issue', 'comment_issue'));

-- 4. issue_link table
CREATE TABLE issue_link (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    kind TEXT NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('source', 'output')),
    url TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, url)
);

CREATE INDEX idx_issue_link_issue ON issue_link(issue_id);
CREATE INDEX idx_issue_link_workspace_url ON issue_link(workspace_id, url);

-- 5. Migrate existing issue.linked_pr_url + linked_branch into issue_link, then drop columns.
INSERT INTO issue_link (issue_id, workspace_id, source_type, kind, direction, url, external_id)
SELECT id, workspace_id, 'github', 'pr', 'output', linked_pr_url, ''
FROM issue
WHERE linked_pr_url IS NOT NULL AND linked_pr_url <> ''
ON CONFLICT (workspace_id, url) DO NOTHING;

INSERT INTO issue_link (issue_id, workspace_id, source_type, kind, direction, url, external_id)
SELECT id, workspace_id, 'github', 'branch', 'output', linked_branch, ''
FROM issue
WHERE linked_branch IS NOT NULL AND linked_branch <> ''
ON CONFLICT (workspace_id, url) DO NOTHING;

ALTER TABLE issue
    DROP COLUMN IF EXISTS linked_pr_url,
    DROP COLUMN IF EXISTS linked_branch;

-- 6. Drop the per-event-rule table; routing is now handled by webhook_action filters.
DROP TABLE IF EXISTS github_event_rule;
