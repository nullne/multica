DROP TABLE IF EXISTS daemon_workspace;

DROP INDEX IF EXISTS idx_daemon_user;

ALTER TABLE daemon DROP CONSTRAINT IF EXISTS daemon_user_id_daemon_id_key;
ALTER TABLE daemon ADD COLUMN workspace_id UUID REFERENCES workspace(id) ON DELETE CASCADE;

-- Best-effort backfill: link each daemon to one workspace its owner belongs to.
UPDATE daemon d
SET workspace_id = (
    SELECT m.workspace_id
    FROM member m
    WHERE m.user_id = d.user_id
    ORDER BY m.created_at ASC
    LIMIT 1
);

DELETE FROM daemon WHERE workspace_id IS NULL;

ALTER TABLE daemon ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE daemon ADD CONSTRAINT daemon_workspace_id_daemon_id_key UNIQUE (workspace_id, daemon_id);

ALTER TABLE daemon DROP COLUMN user_id;

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
