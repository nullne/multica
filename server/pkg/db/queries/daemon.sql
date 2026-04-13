-- name: UpsertDaemon :one
INSERT INTO daemon (
    workspace_id, daemon_id, status, cli_version, device_name, device_info, metadata, last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (workspace_id, daemon_id)
DO UPDATE SET
    status = EXCLUDED.status,
    cli_version = EXCLUDED.cli_version,
    device_name = EXCLUDED.device_name,
    device_info = EXCLUDED.device_info,
    metadata = EXCLUDED.metadata,
    last_seen_at = now(),
    updated_at = now()
RETURNING *;

-- name: GetDaemon :one
SELECT * FROM daemon WHERE id = $1;

-- name: ListDaemons :many
SELECT * FROM daemon
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: ListOnlineDaemons :many
SELECT * FROM daemon
WHERE workspace_id = $1 AND status = 'online'
ORDER BY created_at ASC;

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

-- name: MarkStaleDaemonsOffline :many
UPDATE daemon
SET status = 'offline', updated_at = now()
WHERE status = 'online'
  AND last_seen_at < now() - make_interval(secs => @stale_seconds::double precision)
RETURNING id, workspace_id;
