-- name: UpsertDaemon :one
INSERT INTO daemon (
    user_id, daemon_id, status, cli_version, device_name, device_info, metadata, last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (user_id, daemon_id)
DO UPDATE SET
    status = EXCLUDED.status,
    cli_version = EXCLUDED.cli_version,
    device_name = CASE WHEN daemon.device_name = '' OR daemon.device_name = daemon.daemon_id
                       THEN EXCLUDED.device_name
                       ELSE daemon.device_name END,
    device_info = EXCLUDED.device_info,
    metadata = EXCLUDED.metadata,
    last_seen_at = now(),
    updated_at = now()
RETURNING *;

-- name: GetDaemon :one
SELECT * FROM daemon WHERE id = $1;

-- name: GetDaemonForUser :one
SELECT * FROM daemon WHERE id = $1 AND user_id = $2;

-- name: GetDaemonByUserAndDaemonID :one
SELECT * FROM daemon WHERE user_id = $1 AND daemon_id = $2;

-- name: ListDaemonsForUser :many
SELECT * FROM daemon
WHERE user_id = $1
ORDER BY archived_at NULLS FIRST, created_at ASC;

-- name: ListDaemonsForWorkspace :many
-- All daemons that are currently enabled for a workspace.
SELECT d.* FROM daemon d
JOIN daemon_workspace dw ON dw.daemon_id = d.id
JOIN member m ON m.user_id = d.user_id AND m.workspace_id = dw.workspace_id
WHERE dw.workspace_id = $1 AND dw.enabled = TRUE
ORDER BY d.archived_at NULLS FIRST, d.created_at ASC;

-- name: ListOnlineDaemonsForWorkspace :many
SELECT d.* FROM daemon d
JOIN daemon_workspace dw ON dw.daemon_id = d.id
JOIN member m ON m.user_id = d.user_id AND m.workspace_id = dw.workspace_id
WHERE dw.workspace_id = $1 AND dw.enabled = TRUE AND d.status = 'online'
ORDER BY d.created_at ASC;

-- name: UpdateDaemonHeartbeat :one
UPDATE daemon
SET status = 'online', last_seen_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetDaemonOffline :exec
UPDATE daemon
SET status = 'offline', updated_at = now()
WHERE id = $1;

-- name: SetDaemonAndRuntimesOffline :exec
WITH updated_daemon AS (
    UPDATE daemon SET status = 'offline', updated_at = now() WHERE id = $1
)
UPDATE agent_runtime SET status = 'offline', updated_at = now()
WHERE daemon_ref = $1;

-- name: UpdateDaemonLabels :one
UPDATE daemon SET labels = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateDaemonFields :one
UPDATE daemon SET
    device_name = COALESCE(sqlc.narg('device_name'), device_name),
    labels = COALESCE(sqlc.narg('labels'), labels),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetDaemonStatus :exec
UPDATE daemon SET status = $2, updated_at = now()
WHERE id = $1;

-- name: SetRuntimeStatus :exec
UPDATE agent_runtime SET status = $2, updated_at = now()
WHERE id = $1;

-- name: SetDaemonAndRuntimesUpdating :exec
WITH updated_daemon AS (
    UPDATE daemon SET status = 'updating', updated_at = now() WHERE id = $1
)
UPDATE agent_runtime SET status = 'updating', updated_at = now()
WHERE daemon_ref = $1;

-- name: MarkStaleDaemonsOffline :many
UPDATE daemon
SET status = 'offline', updated_at = now()
WHERE status IN ('online', 'updating')
  AND last_seen_at < now() - make_interval(secs => @stale_seconds::double precision)
RETURNING id, user_id;

-- name: ArchiveDaemon :one
UPDATE daemon SET archived_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RestoreDaemon :one
UPDATE daemon SET archived_at = NULL, updated_at = now()
WHERE id = $1
RETURNING *;
