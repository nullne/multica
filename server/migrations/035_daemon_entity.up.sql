-- Create the daemon table as a first-class entity.
-- A daemon represents a local process running on a machine, managing one or more runtimes.
CREATE TABLE daemon (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    daemon_id       TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'offline' CHECK (status IN ('online', 'offline')),
    cli_version     TEXT NOT NULL DEFAULT '',
    device_name     TEXT NOT NULL DEFAULT '',
    device_info     TEXT NOT NULL DEFAULT '',
    metadata        JSONB NOT NULL DEFAULT '{}',
    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, daemon_id)
);

-- Add daemon_ref FK to agent_runtime, pointing to the new daemon table.
ALTER TABLE agent_runtime ADD COLUMN daemon_ref UUID REFERENCES daemon(id) ON DELETE SET NULL;

-- Migrate existing runtimes: create daemon rows from distinct (workspace_id, daemon_id) pairs,
-- then backfill daemon_ref.
INSERT INTO daemon (workspace_id, daemon_id, status, device_info, last_seen_at, created_at, updated_at)
SELECT DISTINCT ON (workspace_id, daemon_id)
    workspace_id,
    daemon_id,
    status,
    device_info,
    last_seen_at,
    created_at,
    updated_at
FROM agent_runtime
WHERE daemon_id IS NOT NULL AND daemon_id != ''
ORDER BY workspace_id, daemon_id, created_at ASC;

UPDATE agent_runtime ar
SET daemon_ref = d.id
FROM daemon d
WHERE ar.daemon_id = d.daemon_id
  AND ar.workspace_id = d.workspace_id
  AND ar.daemon_id IS NOT NULL
  AND ar.daemon_id != '';
