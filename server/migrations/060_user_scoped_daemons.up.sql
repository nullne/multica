-- Make local daemons user-scoped with workspace assignment.
--
-- Previously a daemon row was keyed by (workspace_id, daemon_id), forcing a
-- physical machine to register once per watched workspace. Now a daemon is
-- owned by a user and assigned to workspaces separately via daemon_workspace.

-- The daemon_token path was never wired into the request lifecycle (daemons
-- authenticate with the user's PAT/JWT). Drop it now so the new model is the
-- only auth story.
DROP TABLE IF EXISTS daemon_token;

-- Add owner column on daemon.
ALTER TABLE daemon ADD COLUMN user_id UUID REFERENCES "user"(id) ON DELETE CASCADE;

-- Backfill user_id from existing workspace membership. Pick the workspace's
-- owner; fall back to admin, then the earliest member.
UPDATE daemon d
SET user_id = (
    SELECT m.user_id
    FROM member m
    WHERE m.workspace_id = d.workspace_id
    ORDER BY CASE m.role
        WHEN 'owner' THEN 0
        WHEN 'admin' THEN 1
        ELSE 2
    END, m.created_at ASC
    LIMIT 1
);

-- If a daemon's workspace has no members left, drop it (and its runtimes)
-- rather than leaving orphan rows.
DELETE FROM agent_runtime
WHERE daemon_ref IN (SELECT id FROM daemon WHERE user_id IS NULL);
DELETE FROM daemon WHERE user_id IS NULL;

ALTER TABLE daemon ALTER COLUMN user_id SET NOT NULL;

-- Replace the workspace-scoped uniqueness with a user-scoped one.
ALTER TABLE daemon DROP CONSTRAINT daemon_workspace_id_daemon_id_key;
ALTER TABLE daemon DROP COLUMN workspace_id;
ALTER TABLE daemon ADD CONSTRAINT daemon_user_id_daemon_id_key UNIQUE (user_id, daemon_id);

CREATE INDEX idx_daemon_user ON daemon(user_id);

-- daemon_workspace tracks which workspaces a daemon is allowed to serve.
-- enabled=FALSE leaves the row in place so it survives toggles without
-- losing history. The composite primary key prevents duplicates.
CREATE TABLE daemon_workspace (
    daemon_id    UUID NOT NULL REFERENCES daemon(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (daemon_id, workspace_id)
);

CREATE INDEX idx_daemon_workspace_workspace ON daemon_workspace(workspace_id);
CREATE INDEX idx_daemon_workspace_enabled ON daemon_workspace(workspace_id, enabled);

-- Backfill assignments from the existing runtime rows: every workspace where
-- this daemon already had runtime entries is treated as enabled.
INSERT INTO daemon_workspace (daemon_id, workspace_id, enabled)
SELECT DISTINCT ar.daemon_ref, ar.workspace_id, TRUE
FROM agent_runtime ar
WHERE ar.daemon_ref IS NOT NULL
ON CONFLICT DO NOTHING;
