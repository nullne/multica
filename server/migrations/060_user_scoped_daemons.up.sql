-- Make local daemons user-scoped with workspace assignment.
--
-- Previously a daemon row was keyed by (workspace_id, daemon_id), forcing a
-- physical machine to register once per watched workspace. Now a daemon is
-- owned by a user and assigned to workspaces separately via daemon_workspace.

-- The daemon_token path was never wired into the request lifecycle (daemons
-- authenticate with the user's PAT/JWT). Drop it now so the new model is the
-- only auth story.
DROP TABLE IF EXISTS daemon_token;

-- Existing daemon rows cannot be reliably mapped onto the new user-scoped
-- ownership model: the old schema never recorded which user registered the
-- daemon, so any backfill from workspace membership would be a guess and
-- could attribute someone else's machine to the workspace owner. Wipe the
-- old rows and let local daemons re-register under their owning user on
-- next start. Local runtime rows (daemon_ref IS NOT NULL) go with them;
-- cloud runtime rows are untouched.
DELETE FROM agent_runtime WHERE daemon_ref IS NOT NULL;
DELETE FROM daemon;

-- Replace the workspace-scoped key with a user-scoped one.
ALTER TABLE daemon DROP CONSTRAINT daemon_workspace_id_daemon_id_key;
ALTER TABLE daemon DROP COLUMN workspace_id;
ALTER TABLE daemon ADD COLUMN user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE;
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
