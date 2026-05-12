DROP TABLE IF EXISTS daemon_workspace;

DROP INDEX IF EXISTS idx_daemon_user;

ALTER TABLE daemon DROP CONSTRAINT IF EXISTS daemon_user_id_daemon_id_key;

-- Same shape as the up migration: the daemon rows in the new model carry a
-- user_id with no recorded workspace, so rolling back can't reconstruct the
-- workspace_id without guessing. Drop them and let the old code re-register
-- daemons under their workspace context.
DELETE FROM agent_runtime WHERE daemon_ref IS NOT NULL;
DELETE FROM daemon;

ALTER TABLE daemon DROP COLUMN user_id;
ALTER TABLE daemon ADD COLUMN workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE;
ALTER TABLE daemon ADD CONSTRAINT daemon_workspace_id_daemon_id_key UNIQUE (workspace_id, daemon_id);

CREATE TABLE daemon_token (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    daemon_id TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_daemon_token_hash ON daemon_token(token_hash);
CREATE INDEX idx_daemon_token_workspace_daemon ON daemon_token(workspace_id, daemon_id);
