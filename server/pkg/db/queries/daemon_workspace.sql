-- name: UpsertDaemonWorkspace :one
-- Enable (or re-enable) a daemon for a workspace. Idempotent.
INSERT INTO daemon_workspace (daemon_id, workspace_id, enabled)
VALUES ($1, $2, $3)
ON CONFLICT (daemon_id, workspace_id)
DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = now()
RETURNING *;

-- name: GetDaemonWorkspace :one
SELECT * FROM daemon_workspace
WHERE daemon_id = $1 AND workspace_id = $2;

-- name: ListWorkspacesForDaemon :many
SELECT * FROM daemon_workspace
WHERE daemon_id = $1
ORDER BY created_at ASC;

-- name: ListEnabledWorkspacesForDaemon :many
-- Workspaces a daemon is currently allowed to serve, scoped to workspaces
-- where the daemon's owner is still a member.
SELECT dw.workspace_id
FROM daemon_workspace dw
JOIN daemon d ON d.id = dw.daemon_id
JOIN member m ON m.workspace_id = dw.workspace_id AND m.user_id = d.user_id
WHERE dw.daemon_id = $1 AND dw.enabled = TRUE
ORDER BY dw.created_at ASC;

-- name: DeleteDaemonWorkspace :exec
DELETE FROM daemon_workspace
WHERE daemon_id = $1 AND workspace_id = $2;
