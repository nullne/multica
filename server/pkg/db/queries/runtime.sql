-- name: ListAgentRuntimes :many
-- Cloud runtimes (daemon_ref IS NULL) always show. Local runtimes only show
-- while their daemon is enabled for this workspace AND the owner is still a
-- workspace member — the same constraint dispatch and the daemon list use.
SELECT ar.* FROM agent_runtime ar
LEFT JOIN daemon d ON d.id = ar.daemon_ref
WHERE ar.workspace_id = $1
  AND (
    ar.daemon_ref IS NULL
    OR EXISTS (
      SELECT 1 FROM daemon_workspace dw
      JOIN member m ON m.user_id = d.user_id AND m.workspace_id = dw.workspace_id
      WHERE dw.daemon_id = ar.daemon_ref
        AND dw.workspace_id = ar.workspace_id
        AND dw.enabled = TRUE
    )
  )
ORDER BY ar.created_at ASC;

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
-- Keep auth state fresh for every workspace the daemon owner still belongs
-- to, even when the daemon is not workspace-visible there. Explicit issue
-- dispatches can target those hidden runtimes by daemon ID. Runtimes in
-- workspaces where the owner is no longer a member are left frozen.
UPDATE agent_runtime ar
SET auth_status = $2, updated_at = now()
FROM daemon d
JOIN member m ON m.user_id = d.user_id
WHERE ar.daemon_ref = $1
  AND ar.provider = $3
  AND d.id = ar.daemon_ref
  AND m.workspace_id = ar.workspace_id;

-- name: SetAgentRuntimeOffline :exec
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE id = $1;

-- name: UpdateRuntimesHeartbeatByDaemon :exec
-- A heartbeat reconciles every runtime owned by this daemon against the
-- owner's current workspace memberships:
--   - online for workspaces where the owner is still a member (hidden
--     assignments included, so explicit daemon-targeted dispatch keeps
--     working).
--   - offline for workspaces the owner has left, so general workspace
--     dispatch and visibility stop returning them.
UPDATE agent_runtime ar
SET status = CASE WHEN owner_in_workspace THEN 'online' ELSE 'offline' END,
    last_seen_at = CASE WHEN owner_in_workspace THEN now() ELSE ar.last_seen_at END,
    updated_at = now()
FROM (
  SELECT EXISTS (
    SELECT 1 FROM member m
    JOIN daemon d ON d.user_id = m.user_id
    WHERE d.id = ar2.daemon_ref AND m.workspace_id = ar2.workspace_id
  ) AS owner_in_workspace,
  ar2.id AS runtime_id
  FROM agent_runtime ar2
  WHERE ar2.daemon_ref = $1
) sub
WHERE ar.id = sub.runtime_id;

-- name: SetRuntimesOfflineByDaemon :exec
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE daemon_ref = $1;

-- name: SetRuntimesOfflineByDaemonAndWorkspace :exec
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE daemon_ref = $1 AND workspace_id = $2;

-- name: DeleteRuntimesByDaemonAndWorkspace :exec
DELETE FROM agent_runtime
WHERE daemon_ref = $1 AND workspace_id = $2;

-- name: ListRuntimesByDaemon :many
SELECT * FROM agent_runtime
WHERE daemon_ref = $1
ORDER BY provider ASC;

-- name: FindAvailableRuntimeForProvider :one
-- Finds the best online runtime for a given workspace + provider,
-- preferring runtimes with the fewest active tasks (simple load balancing).
-- For local runtimes (daemon_ref IS NOT NULL), the daemon must be enabled
-- for the workspace AND the daemon's owner must still be a member.
-- Cloud runtimes (no daemon_ref) bypass both checks.
SELECT ar.* FROM agent_runtime ar
LEFT JOIN daemon d ON d.id = ar.daemon_ref
WHERE ar.workspace_id = $1
  AND ar.provider = $2
  AND ar.status = 'online'
  AND (
    ar.daemon_ref IS NULL
    OR EXISTS (
      SELECT 1 FROM daemon_workspace dw
      JOIN member m ON m.user_id = d.user_id AND m.workspace_id = dw.workspace_id
      WHERE dw.daemon_id = ar.daemon_ref
        AND dw.workspace_id = ar.workspace_id
        AND dw.enabled = TRUE
    )
  )
ORDER BY (
  SELECT COUNT(*) FROM agent_task_queue atq
  WHERE atq.runtime_id = ar.id AND atq.status IN ('queued', 'dispatched', 'running')
) ASC
LIMIT 1;

-- name: FindAvailableRuntimeForProviders :one
-- Finds the best online runtime matching any of the given providers.
SELECT ar.* FROM agent_runtime ar
LEFT JOIN daemon d ON d.id = ar.daemon_ref
WHERE ar.workspace_id = $1
  AND ar.provider = ANY(@providers::text[])
  AND ar.status = 'online'
  AND (
    ar.daemon_ref IS NULL
    OR EXISTS (
      SELECT 1 FROM daemon_workspace dw
      JOIN member m ON m.user_id = d.user_id AND m.workspace_id = dw.workspace_id
      WHERE dw.daemon_id = ar.daemon_ref
        AND dw.workspace_id = ar.workspace_id
        AND dw.enabled = TRUE
    )
  )
ORDER BY (
  SELECT COUNT(*) FROM agent_task_queue atq
  WHERE atq.runtime_id = ar.id AND atq.status IN ('queued', 'dispatched', 'running')
) ASC
LIMIT 1;

-- name: FindAvailableRuntimeConstrained :one
-- Finds the best online runtime matching optional constraints (provider, daemon, label).
-- All constraints are optional — pass NULL to skip.
-- Local runtimes (daemon_ref IS NOT NULL) normally require an enabled
-- daemon_workspace assignment and workspace membership. When daemon_id is
-- explicitly constrained, the enabled-row requirement is waived as long as
-- the daemon owner is still a workspace member.
SELECT ar.* FROM agent_runtime ar
LEFT JOIN daemon d ON d.id = ar.daemon_ref
WHERE ar.workspace_id = $1
  AND ar.status = 'online'
  AND (sqlc.narg('provider')::text IS NULL OR ar.provider = sqlc.narg('provider'))
  AND (sqlc.narg('providers')::text[] IS NULL OR ar.provider = ANY(sqlc.narg('providers')::text[]))
  AND (sqlc.narg('daemon_id')::uuid IS NULL OR ar.daemon_ref = sqlc.narg('daemon_id'))
  AND (sqlc.narg('daemon_label')::text IS NULL OR sqlc.narg('daemon_label') = ANY(d.labels))
  AND (
    ar.daemon_ref IS NULL
    OR (
      sqlc.narg('daemon_id')::uuid IS NOT NULL
      AND ar.daemon_ref = sqlc.narg('daemon_id')
      AND EXISTS (
        SELECT 1 FROM member m
        WHERE m.user_id = d.user_id
          AND m.workspace_id = ar.workspace_id
      )
    )
    OR EXISTS (
      SELECT 1 FROM daemon_workspace dw
      JOIN member m ON m.user_id = d.user_id AND m.workspace_id = dw.workspace_id
      WHERE dw.daemon_id = ar.daemon_ref
        AND dw.workspace_id = ar.workspace_id
        AND dw.enabled = TRUE
    )
  )
ORDER BY (
  SELECT COUNT(*) FROM agent_task_queue atq
  WHERE atq.runtime_id = ar.id AND atq.status IN ('queued', 'dispatched', 'running')
) ASC
LIMIT 1;

-- name: MarkStaleRuntimesOffline :many
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE status IN ('online', 'updating')
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
