-- name: ListAgentRuntimes :many
SELECT * FROM agent_runtime
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: GetAgentRuntime :one
SELECT * FROM agent_runtime
WHERE id = $1;

-- name: GetAgentRuntimeForWorkspace :one
SELECT * FROM agent_runtime
WHERE id = $1 AND workspace_id = $2;

-- name: UpsertAgentRuntime :one
INSERT INTO agent_runtime (
    workspace_id,
    daemon_id,
    daemon_ref,
    name,
    runtime_mode,
    provider,
    status,
    auth_status,
    device_info,
    metadata,
    last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
ON CONFLICT (workspace_id, daemon_id, provider)
DO UPDATE SET
    daemon_ref = EXCLUDED.daemon_ref,
    name = EXCLUDED.name,
    runtime_mode = EXCLUDED.runtime_mode,
    status = EXCLUDED.status,
    auth_status = EXCLUDED.auth_status,
    device_info = EXCLUDED.device_info,
    metadata = EXCLUDED.metadata,
    last_seen_at = now(),
    updated_at = now()
RETURNING *;

-- name: UpdateAgentRuntimeHeartbeat :one
UPDATE agent_runtime
SET status = 'online', last_seen_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateAgentRuntimeAuthStatus :exec
UPDATE agent_runtime
SET auth_status = $2, updated_at = now()
WHERE id = $1;

-- name: UpdateRuntimesAuthStatusByDaemon :exec
UPDATE agent_runtime
SET auth_status = $2, updated_at = now()
WHERE daemon_ref = $1 AND provider = $3;

-- name: SetAgentRuntimeOffline :exec
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE id = $1;

-- name: UpdateRuntimesHeartbeatByDaemon :exec
UPDATE agent_runtime
SET status = 'online', last_seen_at = now(), updated_at = now()
WHERE daemon_ref = $1;

-- name: SetRuntimesOfflineByDaemon :exec
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE daemon_ref = $1;

-- name: ListRuntimesByDaemon :many
SELECT * FROM agent_runtime
WHERE daemon_ref = $1
ORDER BY provider ASC;

-- name: FindAvailableRuntimeForProvider :one
-- Finds the best online runtime for a given workspace + provider,
-- preferring runtimes with the fewest active tasks (simple load balancing).
SELECT ar.* FROM agent_runtime ar
WHERE ar.workspace_id = $1
  AND ar.provider = $2
  AND ar.status = 'online'
ORDER BY (
  SELECT COUNT(*) FROM agent_task_queue atq
  WHERE atq.runtime_id = ar.id AND atq.status IN ('queued', 'dispatched', 'running')
) ASC
LIMIT 1;

-- name: FindAvailableRuntimeForProviders :one
-- Finds the best online runtime matching any of the given providers.
SELECT ar.* FROM agent_runtime ar
WHERE ar.workspace_id = $1
  AND ar.provider = ANY(@providers::text[])
  AND ar.status = 'online'
ORDER BY (
  SELECT COUNT(*) FROM agent_task_queue atq
  WHERE atq.runtime_id = ar.id AND atq.status IN ('queued', 'dispatched', 'running')
) ASC
LIMIT 1;

-- name: FindAvailableRuntimeConstrained :one
-- Finds the best online runtime matching optional constraints (provider, daemon, label).
-- All constraints are optional — pass NULL to skip.
SELECT ar.* FROM agent_runtime ar
LEFT JOIN daemon d ON ar.daemon_ref = d.id
WHERE ar.workspace_id = $1
  AND ar.status = 'online'
  AND (sqlc.narg('provider')::text IS NULL OR ar.provider = sqlc.narg('provider'))
  AND (sqlc.narg('providers')::text[] IS NULL OR ar.provider = ANY(sqlc.narg('providers')::text[]))
  AND (sqlc.narg('daemon_id')::uuid IS NULL OR ar.daemon_ref = sqlc.narg('daemon_id'))
  AND (sqlc.narg('daemon_label')::text IS NULL OR sqlc.narg('daemon_label') = ANY(d.labels))
ORDER BY (
  SELECT COUNT(*) FROM agent_task_queue atq
  WHERE atq.runtime_id = ar.id AND atq.status IN ('queued', 'dispatched', 'running')
) ASC
LIMIT 1;

-- name: MarkStaleRuntimesOffline :many
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE status = 'online'
  AND last_seen_at < now() - make_interval(secs => @stale_seconds::double precision)
RETURNING id, workspace_id;

-- name: FailTasksForOfflineRuntimes :many
-- Marks dispatched/running tasks as failed when their runtime is offline.
-- This cleans up orphaned tasks after a daemon crash or network partition.
UPDATE agent_task_queue
SET status = 'failed', completed_at = now(), error = 'runtime went offline'
WHERE status IN ('dispatched', 'running')
  AND runtime_id IN (
    SELECT id FROM agent_runtime WHERE status = 'offline'
  )
RETURNING id, agent_id, issue_id;
